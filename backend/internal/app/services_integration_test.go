package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

func TestServicesCoreWorkflow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := store.NewMemoryRepository()
	services := NewServices(StaticRepositoryProvider{Repo: repo}, nil)

	required, err := services.Identity.PasswordSetupRequired(ctx)
	requireNoError(t, err)
	if required {
		t.Fatal("an empty repository has no existing account requiring a password")
	}

	admin, err := services.Identity.CreateFirstAdmin(ctx, "Admin", "admin@example.com", "password123")
	requireNoError(t, err)
	if admin.Role != "admin" || !admin.PasswordSet {
		t.Fatalf("first admin = %#v", admin)
	}
	required, err = services.Identity.PasswordSetupRequired(ctx)
	requireNoError(t, err)
	if required {
		t.Fatal("first-admin setup must be complete")
	}

	authenticated, err := services.Identity.Authenticate(ctx, "ADMIN@example.com", "password123")
	requireNoError(t, err)
	if authenticated.ID != admin.ID {
		t.Fatalf("authenticated user = %q, want %q", authenticated.ID, admin.ID)
	}
	token, err := services.Identity.CreateSession(ctx, admin.ID, time.Now().Add(time.Hour))
	requireNoError(t, err)
	if token == "" {
		t.Fatal("session token must not be empty")
	}
	byToken, err := services.Identity.UserBySessionToken(ctx, token)
	requireNoError(t, err)
	if byToken.ID != admin.ID {
		t.Fatalf("session user = %q, want %q", byToken.ID, admin.ID)
	}
	if _, err := services.Identity.UserBySessionToken(ctx, ""); err == nil {
		t.Fatal("blank session token must be rejected")
	}
	requireNoError(t, services.Identity.DeleteSession(ctx, token))
	if _, err := services.Identity.UserBySessionToken(ctx, token); err == nil {
		t.Fatal("deleted session must not authenticate")
	}

	reviewer, err := services.Identity.CreateUser(ctx, "Reviewer", "reviewer@example.com", "reviewer", "temporary-password")
	requireNoError(t, err)
	member, err := services.Identity.CreateUser(ctx, "Member", "member@example.com", "member", "")
	requireNoError(t, err)
	member, err = services.Identity.UpdateUser(ctx, member.ID, "Updated Member", "member@example.com", "architect", "active", "")
	requireNoError(t, err)
	if member.DisplayName != "Updated Member" || member.Role != "architect" {
		t.Fatalf("updated member = %#v", member)
	}
	users, err := services.Identity.ListUsers(ctx)
	requireNoError(t, err)
	if len(users) != 3 {
		t.Fatalf("user count = %d, want 3", len(users))
	}
	userPage, pageInfo, err := services.Identity.ListUsersPage(ctx, store.PageOptions{Limit: 2})
	requireNoError(t, err)
	if len(userPage) != 2 || !pageInfo.HasMore || pageInfo.NextCursor == "" {
		t.Fatalf("user page = %d, info = %#v", len(userPage), pageInfo)
	}

	resetToken, reset, err := services.Identity.CreatePasswordResetToken(ctx, member.ID, time.Now().Add(time.Hour))
	requireNoError(t, err)
	if resetToken == "" || reset.UserID != member.ID {
		t.Fatalf("password reset = %q %#v", resetToken, reset)
	}
	resetUser, err := services.Identity.ResetPasswordWithToken(ctx, resetToken, "replacement-password")
	requireNoError(t, err)
	if !resetUser.PasswordSet {
		t.Fatal("password reset must mark the password as set")
	}
	_, err = services.Identity.Authenticate(ctx, member.Email, "replacement-password")
	requireNoError(t, err)

	t.Run("configuration", func(t *testing.T) {
		signIn, err := services.Identity.GetSignInConfig(ctx)
		requireNoError(t, err)
		invalid := signIn
		invalid.LocalPasswordEnabled = false
		invalid.SSOEnabled = false
		if _, err := services.Identity.UpdateSignInConfig(ctx, invalid, ""); err == nil {
			t.Fatal("disabling every sign-in method should fail")
		}
		signIn.LocalPasswordEnabled = false
		signIn.SSOEnabled = true
		signIn.Issuer = "https://id.example.com/oauth2/default"
		signIn.ClientID = "stratum-client"
		signIn.RedirectURI = "https://stratum.example.com/api/auth/oidc/callback"
		signIn, err = services.Identity.UpdateSignInConfig(ctx, signIn, "oidc-secret")
		requireNoError(t, err)
		if signIn.LocalPasswordEnabled || !signIn.ClientSecretSet {
			t.Fatalf("sign-in config = %#v", signIn)
		}
		if _, err := services.Identity.Authenticate(ctx, admin.Email, "password123"); err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatalf("disabled local authentication error = %v", err)
		}
		if _, _, err := services.Identity.CreatePasswordResetToken(ctx, member.ID, time.Now().Add(time.Hour)); err == nil {
			t.Fatal("password reset must be disabled with local sign-in")
		}
		if _, err := services.Identity.ResetPasswordWithToken(ctx, "unused", "password"); err == nil {
			t.Fatal("password reset redemption must be disabled with local sign-in")
		}
		signIn.LocalPasswordEnabled = true
		_, err = services.Identity.UpdateSignInConfig(ctx, signIn, "")
		requireNoError(t, err)

		ai, err := services.Identity.UpdateAIProviderConfig(ctx, domain.AIProviderConfig{
			Enabled: true, Provider: "openai", Model: "gpt-test", BaseURL: "https://ai.example.test",
		}, "api-secret")
		requireNoError(t, err)
		if !ai.APIKeySet || ai.APIKey != "" {
			t.Fatalf("sanitized AI config = %#v", ai)
		}
		aiSecret, err := services.Identity.GetAIProviderConfigWithSecret(ctx)
		requireNoError(t, err)
		if aiSecret.APIKey != "api-secret" {
			t.Fatalf("AI secret = %q", aiSecret.APIKey)
		}
		_, err = services.Identity.GetAIProviderConfig(ctx)
		requireNoError(t, err)

		mcp, err := services.Identity.UpdateMCPConfig(ctx, domain.MCPConfig{Enabled: true, EndpointPath: "enterprise-mcp", ReadDesigns: true})
		requireNoError(t, err)
		if !mcp.Enabled || mcp.EndpointPath != "/enterprise-mcp" {
			t.Fatalf("MCP config = %#v", mcp)
		}
		_, err = services.Identity.GetMCPConfig(ctx)
		requireNoError(t, err)

		telemetry, err := services.Identity.UpdateTelemetryIntegrationConfig(ctx, domain.TelemetryIntegrationConfig{
			Enabled: true, Provider: "prometheus", DisplayName: "Production", BaseURL: "https://metrics.example.test",
			AuthMode: "bearer", QueryWindow: "10m",
		}, "metrics-secret")
		requireNoError(t, err)
		if !telemetry.SecretSet || telemetry.Secret != "" {
			t.Fatalf("sanitized telemetry config = %#v", telemetry)
		}
		telemetrySecret, err := services.Identity.GetTelemetryIntegrationConfigWithSecret(ctx)
		requireNoError(t, err)
		if telemetrySecret.Secret != "metrics-secret" {
			t.Fatalf("telemetry secret = %q", telemetrySecret.Secret)
		}
		_, err = services.Identity.GetTelemetryIntegrationConfig(ctx)
		requireNoError(t, err)
	})

	guest, err := services.Workspaces.GetOrCreateGuest(ctx)
	requireNoError(t, err)
	workspace, err := services.Workspaces.CreateWithAccess(ctx, "Payments", admin.ID, nil)
	requireNoError(t, err)
	if workspace.Name != "Payments" {
		t.Fatalf("workspace = %#v", workspace)
	}
	_, err = services.Workspaces.Get(ctx, workspace.ID)
	requireNoError(t, err)
	workspaces, err := services.Workspaces.List(ctx)
	requireNoError(t, err)
	if len(workspaces) != 2 {
		t.Fatalf("workspace count = %d, want guest plus payments", len(workspaces))
	}
	workspacePage, workspacePageInfo, err := services.Workspaces.ListPage(ctx, store.PageOptions{Limit: 1})
	requireNoError(t, err)
	if len(workspacePage) != 1 || !workspacePageInfo.HasMore {
		t.Fatalf("workspace page = %d, info = %#v", len(workspacePage), workspacePageInfo)
	}

	group, err := services.ACL.CreateGroup(ctx, domain.AccessGroup{Name: "Architecture", Description: "Architecture reviewers", OktaGroupName: "stratum-architects"})
	requireNoError(t, err)
	group, err = services.ACL.UpdateGroup(ctx, group.ID, domain.AccessGroup{Name: "Architecture Guild", Description: group.Description, OktaGroupName: group.OktaGroupName})
	requireNoError(t, err)
	if group.Name != "Architecture Guild" {
		t.Fatalf("updated group = %#v", group)
	}
	members, err := services.ACL.ReplaceGroupMembers(ctx, group.ID, []string{reviewer.ID, member.ID})
	requireNoError(t, err)
	if len(members) != 2 {
		t.Fatalf("group member count = %d, want 2", len(members))
	}
	listedMembers, err := services.ACL.ListGroupMembers(ctx, group.ID)
	requireNoError(t, err)
	if len(listedMembers) != 2 {
		t.Fatalf("listed group member count = %d", len(listedMembers))
	}
	groupIDs, err := services.ACL.ListUserGroupIDs(ctx, reviewer.ID)
	requireNoError(t, err)
	if len(groupIDs) != 1 || groupIDs[0] != group.ID {
		t.Fatalf("reviewer group IDs = %#v", groupIDs)
	}
	groups, err := services.ACL.ListGroups(ctx)
	requireNoError(t, err)
	if len(groups) != 1 || groups[0].MemberCount != 2 {
		t.Fatalf("groups = %#v", groups)
	}

	workspaceUserGrant, err := services.ACL.GrantWorkspaceAccess(ctx, domain.WorkspaceAccess{
		WorkspaceID: workspace.ID, UserID: reviewer.ID, CanRead: true,
	})
	requireNoError(t, err)
	if !workspaceUserGrant.CanRead {
		t.Fatalf("workspace user grant = %#v", workspaceUserGrant)
	}
	workspaceGroupGrant, err := services.ACL.GrantWorkspaceGroupAccess(ctx, domain.WorkspaceGroupAccess{
		WorkspaceID: workspace.ID, GroupID: group.ID, CanRead: true, CanCreateDesign: true,
	})
	requireNoError(t, err)
	if !workspaceGroupGrant.CanCreateDesign {
		t.Fatalf("workspace group grant = %#v", workspaceGroupGrant)
	}
	workspaceAccess, err := services.ACL.ListWorkspaceAccess(ctx, workspace.ID)
	requireNoError(t, err)
	if len(workspaceAccess) != 2 {
		t.Fatalf("workspace user grant count = %d, want owner and reviewer", len(workspaceAccess))
	}
	workspaceGroupAccess, err := services.ACL.ListWorkspaceGroupAccess(ctx, workspace.ID)
	requireNoError(t, err)
	if len(workspaceGroupAccess) != 1 {
		t.Fatalf("workspace group grant count = %d", len(workspaceGroupAccess))
	}

	document := json.RawMessage(`{"schemaVersion":"stratum/v1","components":[{"id":"api","type":"compute.service"}],"connectors":[]}`)
	design, err := services.Designs.Create(ctx, workspace.ID, "Payments API", document, admin.ID)
	requireNoError(t, err)
	if string(design.Document) != string(document) {
		t.Fatalf("design document changed: got %q want %q", design.Document, document)
	}
	design, err = services.Designs.UpdateMetadata(ctx, workspace.ID, design.ID, "Payments Platform", "workspace")
	requireNoError(t, err)
	if design.Name != "Payments Platform" || design.Access != "workspace" {
		t.Fatalf("updated design = %#v", design)
	}
	design.Document = json.RawMessage(`{"schemaVersion":"stratum/v1","components":[{"id":"api","type":"compute.service"},{"id":"db","type":"data.sql_database"}],"connectors":[]}`)
	design, err = services.Designs.Upsert(ctx, design)
	requireNoError(t, err)
	_, err = services.Designs.Get(ctx, workspace.ID, design.ID)
	requireNoError(t, err)
	designs, err := services.Designs.List(ctx, workspace.ID)
	requireNoError(t, err)
	if len(designs) != 1 {
		t.Fatalf("design count = %d, want 1", len(designs))
	}
	designPage, designPageInfo, err := services.Designs.ListPage(ctx, workspace.ID, store.PageOptions{Limit: 1})
	requireNoError(t, err)
	if len(designPage) != 1 || designPageInfo.HasMore {
		t.Fatalf("design page = %d, info = %#v", len(designPage), designPageInfo)
	}

	designUserGrant, err := services.ACL.GrantDesignAccess(ctx, domain.DesignAccess{
		WorkspaceID: workspace.ID, DesignID: design.ID, UserID: reviewer.ID, CanRead: true, CanComment: true, CanReview: true,
	})
	requireNoError(t, err)
	if !designUserGrant.CanReview {
		t.Fatalf("design user grant = %#v", designUserGrant)
	}
	designGroupGrant, err := services.ACL.GrantDesignGroupAccess(ctx, domain.DesignGroupAccess{
		WorkspaceID: workspace.ID, DesignID: design.ID, GroupID: group.ID, CanRead: true, CanEdit: true,
	})
	requireNoError(t, err)
	if !designGroupGrant.CanEdit {
		t.Fatalf("design group grant = %#v", designGroupGrant)
	}
	designAccess, err := services.ACL.ListDesignAccess(ctx, workspace.ID, design.ID)
	requireNoError(t, err)
	if len(designAccess) != 1 {
		t.Fatalf("design user grant count = %d", len(designAccess))
	}
	designGroupAccess, err := services.ACL.ListDesignGroupAccess(ctx, workspace.ID, design.ID)
	requireNoError(t, err)
	if len(designGroupAccess) != 1 {
		t.Fatalf("design group grant count = %d", len(designGroupAccess))
	}

	doc, err := services.Docs.Create(ctx, workspace.ID, design.ID, "Runbook", "# Recovery\nDo the safe thing.", "markdown")
	requireNoError(t, err)
	doc, err = services.Docs.Update(ctx, workspace.ID, design.ID, doc.ID, "Recovery runbook", doc.Body+"\n", "markdown")
	requireNoError(t, err)
	if doc.Title != "Recovery runbook" {
		t.Fatalf("updated doc = %#v", doc)
	}
	_, err = services.Docs.Get(ctx, workspace.ID, design.ID, doc.ID)
	requireNoError(t, err)
	docs, err := services.Docs.List(ctx, workspace.ID, design.ID)
	requireNoError(t, err)
	if len(docs) != 1 {
		t.Fatalf("doc count = %d, want 1", len(docs))
	}

	comment, err := services.Reviews.CreateComment(ctx, workspace.ID, design.ID, reviewer.ID, "Clarify failover", "api", "")
	requireNoError(t, err)
	if comment.ComponentID != "api" {
		t.Fatalf("comment = %#v", comment)
	}
	comments, err := services.Reviews.ListComments(ctx, workspace.ID, design.ID)
	requireNoError(t, err)
	if len(comments) != 1 {
		t.Fatalf("comment count = %d", len(comments))
	}
	commentPage, commentInfo, err := services.Reviews.ListCommentsPage(ctx, workspace.ID, design.ID, store.PageOptions{Limit: 1})
	requireNoError(t, err)
	if len(commentPage) != 1 || commentInfo.HasMore {
		t.Fatalf("comment page = %d, info = %#v", len(commentPage), commentInfo)
	}

	version, err := services.Versions.Create(ctx, workspace.ID, design.ID, admin.ID, "Initial review")
	requireNoError(t, err)
	reviews, err := services.Reviews.CreateReviews(ctx, workspace.ID, design.ID, version.ID, admin.ID, []string{reviewer.ID}, "Please review")
	requireNoError(t, err)
	if len(reviews) != 1 || reviews[0].VersionID != version.ID {
		t.Fatalf("reviews = %#v", reviews)
	}
	listedReviews, err := services.Reviews.ListReviews(ctx, workspace.ID, design.ID)
	requireNoError(t, err)
	if len(listedReviews) != 1 {
		t.Fatalf("review count = %d", len(listedReviews))
	}
	reviewPage, reviewInfo, err := services.Reviews.ListReviewsPage(ctx, workspace.ID, design.ID, store.PageOptions{Limit: 1})
	requireNoError(t, err)
	if len(reviewPage) != 1 || reviewInfo.HasMore {
		t.Fatalf("review page = %d, info = %#v", len(reviewPage), reviewInfo)
	}
	approved, err := services.Reviews.UpdateReview(ctx, workspace.ID, design.ID, reviews[0].ID, reviewer, "approved", "Looks good")
	requireNoError(t, err)
	if approved.Status != "approved" || approved.CompletedAt == nil {
		t.Fatalf("approved review = %#v", approved)
	}
	versions, err := services.Versions.List(ctx, workspace.ID, design.ID)
	requireNoError(t, err)
	if len(versions) != 1 || versions[0].Status != "reviewed" {
		t.Fatalf("versions after approval = %#v", versions)
	}
	versionPage, versionInfo, err := services.Versions.ListPage(ctx, workspace.ID, design.ID, store.PageOptions{Limit: 1})
	requireNoError(t, err)
	if len(versionPage) != 1 || versionInfo.HasMore {
		t.Fatalf("version page = %d, info = %#v", len(versionPage), versionInfo)
	}
	version, err = services.Versions.UpdateStatus(ctx, workspace.ID, design.ID, version.ID, "live")
	requireNoError(t, err)
	if version.Status != "live" {
		t.Fatalf("live version = %#v", version)
	}

	notifications, err := services.Identity.ListNotifications(ctx, reviewer.ID)
	requireNoError(t, err)
	if len(notifications) == 0 {
		t.Fatal("review request must notify the reviewer")
	}
	requireNoError(t, services.Identity.MarkNotificationRead(ctx, reviewer.ID, notifications[0].ID))
	notifications, err = services.Identity.ListNotifications(ctx, reviewer.ID)
	requireNoError(t, err)
	if !notifications[0].Read {
		t.Fatal("notification must be marked read")
	}

	asset, err := services.Catalog.CreateAsset(ctx, domain.CatalogAsset{
		Name: "Payments API", Kind: "component", Type: "compute.service", Owner: "Payments", Criticality: "high", CreatedBy: admin.ID,
	})
	requireNoError(t, err)
	asset, err = services.Catalog.UpdateAsset(ctx, asset.ID, domain.CatalogAsset{
		Name: asset.Name, Kind: asset.Kind, Type: asset.Type, Owner: asset.Owner, Criticality: "critical", Status: "active",
	})
	requireNoError(t, err)
	if asset.Criticality != "critical" {
		t.Fatalf("updated asset = %#v", asset)
	}
	assets, err := services.Catalog.ListAssets(ctx, "payments")
	requireNoError(t, err)
	if len(assets) != 1 {
		t.Fatalf("asset count = %d", len(assets))
	}
	assetPage, assetInfo, err := services.Catalog.ListAssetsPage(ctx, store.PageOptions{Limit: 1, Query: "payments"})
	requireNoError(t, err)
	if len(assetPage) != 1 || assetInfo.HasMore {
		t.Fatalf("asset page = %d, info = %#v", len(assetPage), assetInfo)
	}

	report, err := services.Analysis.AnalyzeDocument(design.Document)
	requireNoError(t, err)
	if len(report.Suites) == 0 {
		t.Fatal("analysis must return deterministic suites")
	}

	requireNoError(t, services.Docs.Delete(ctx, workspace.ID, design.ID, doc.ID))
	requireNoError(t, services.ACL.RevokeDesignAccess(ctx, workspace.ID, design.ID, reviewer.ID))
	requireNoError(t, services.ACL.RevokeDesignGroupAccess(ctx, workspace.ID, design.ID, group.ID))
	requireNoError(t, services.ACL.RevokeWorkspaceAccess(ctx, workspace.ID, reviewer.ID))
	requireNoError(t, services.ACL.RevokeWorkspaceGroupAccess(ctx, workspace.ID, group.ID))
	requireNoError(t, services.ACL.DeleteGroup(ctx, group.ID))
	requireNoError(t, services.Catalog.DeleteAsset(ctx, asset.ID))
	requireNoError(t, services.Versions.Delete(ctx, workspace.ID, design.ID, version.ID))
	requireNoError(t, services.Designs.Delete(ctx, workspace.ID, design.ID))
	requireNoError(t, services.Workspaces.Delete(ctx, workspace.ID))
	requireNoError(t, services.Identity.DeleteUser(ctx, member.ID))

	// The guest workspace is intentionally retained and serves as a stable fallback.
	if guest.ID != domain.GuestWorkspaceID {
		t.Fatalf("guest workspace = %#v", guest)
	}
}

func TestWorkspaceAccessLevelValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		level     string
		canCreate bool
		canManage bool
		wantError bool
	}{
		{name: "default viewer", level: "", canCreate: false, canManage: false},
		{name: "viewer", level: " VIEWER ", canCreate: false, canManage: false},
		{name: "contributor", level: "Contributor", canCreate: true, canManage: false},
		{name: "manager", level: "manager", canCreate: true, canManage: true},
		{name: "invalid", level: "owner", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canCreate, canManage, err := workspaceAccessLevel(test.level)
			if (err != nil) != test.wantError {
				t.Fatalf("workspaceAccessLevel(%q) error = %v", test.level, err)
			}
			if canCreate != test.canCreate || canManage != test.canManage {
				t.Fatalf("workspaceAccessLevel(%q) = (%t, %t), want (%t, %t)", test.level, canCreate, canManage, test.canCreate, test.canManage)
			}
		})
	}
}

func TestStorageAdminService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	unconfigured := StorageAdminService{}
	if unconfigured.Configurable() {
		t.Fatal("nil storage backend must not be configurable")
	}
	if status := unconfigured.Status(ctx); status.Mode != store.StorageModeDatabase || !status.DatabaseConnected {
		t.Fatalf("static storage status = %#v", status)
	}
	if err := unconfigured.TestDatabase(ctx, "postgres://example"); err == nil {
		t.Fatal("testing a static storage backend must fail")
	}
	if _, err := unconfigured.ConfigureDatabase(ctx, "postgres://example"); err == nil {
		t.Fatal("configuring a static storage backend must fail")
	}
	if _, _, err := unconfigured.MigrateUsersToDatabase(ctx); err == nil {
		t.Fatal("migrating a static storage backend must fail")
	}

	backend := &fakeStorageBackend{repo: store.NewMemoryRepository(), status: store.StorageStatus{Mode: store.StorageModeStateless}}
	service := StorageAdminService{storage: backend}
	if !service.Configurable() || service.Status(ctx).Mode != store.StorageModeStateless {
		t.Fatal("configurable storage status was not delegated")
	}
	requireNoError(t, service.TestDatabase(ctx, "postgres://valid"))
	configured, err := service.ConfigureDatabase(ctx, "postgres://valid")
	requireNoError(t, err)
	if configured.Mode != store.StorageModeDatabase {
		t.Fatalf("configured status = %#v", configured)
	}
	migrated, count, err := service.MigrateUsersToDatabase(ctx)
	requireNoError(t, err)
	if migrated.Mode != store.StorageModeDatabase || count != 2 {
		t.Fatalf("migration result = %#v, %d", migrated, count)
	}
	if backend.testURL != "postgres://valid" || backend.configureURL != "postgres://valid" {
		t.Fatalf("storage URLs = test %q configure %q", backend.testURL, backend.configureURL)
	}

	backend.err = errors.New("database unavailable")
	if err := service.TestDatabase(ctx, "postgres://bad"); !errors.Is(err, backend.err) {
		t.Fatalf("test database error = %v", err)
	}
	if _, err := service.ConfigureDatabase(ctx, "postgres://bad"); !errors.Is(err, backend.err) {
		t.Fatalf("configure database error = %v", err)
	}
	if _, _, err := service.MigrateUsersToDatabase(ctx); !errors.Is(err, backend.err) {
		t.Fatalf("migrate users error = %v", err)
	}
}

type fakeStorageBackend struct {
	repo         store.Repository
	status       store.StorageStatus
	testURL      string
	configureURL string
	err          error
}

func (f *fakeStorageBackend) Repository() store.Repository { return f.repo }

func (f *fakeStorageBackend) Status(context.Context) store.StorageStatus { return f.status }

func (f *fakeStorageBackend) TestDatabase(_ context.Context, databaseURL string) error {
	f.testURL = databaseURL
	return f.err
}

func (f *fakeStorageBackend) ConfigureDatabase(_ context.Context, databaseURL string) (store.StorageStatus, error) {
	f.configureURL = databaseURL
	if f.err != nil {
		return store.StorageStatus{}, f.err
	}
	f.status = store.StorageStatus{Mode: store.StorageModeDatabase, DatabaseConnected: true}
	return f.status, nil
}

func (f *fakeStorageBackend) MigrateUsersToDatabase(context.Context) (store.StorageStatus, int, error) {
	if f.err != nil {
		return store.StorageStatus{}, 0, f.err
	}
	return store.StorageStatus{Mode: store.StorageModeDatabase, DatabaseConnected: true}, 2, nil
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
