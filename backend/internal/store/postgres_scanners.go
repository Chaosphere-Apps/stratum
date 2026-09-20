package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
	"time"
)

func scanWorkspace(row rowScanner) (domain.Workspace, error) {
	var workspace domain.Workspace
	err := row.Scan(&workspace.ID, &workspace.Name, &workspace.OwnerID, &workspace.CreatedAt, &workspace.UpdatedAt)
	return workspace, err
}

func scanDesign(row rowScanner) (domain.Design, error) {
	var design domain.Design
	var document string
	var canvasSnapshot *string
	err := row.Scan(
		&design.ID,
		&design.WorkspaceID,
		&design.Name,
		&design.Access,
		&design.Title,
		&document,
		&canvasSnapshot,
		&design.VersionNumber,
		&design.CreatedBy,
		&design.CreatedAt,
		&design.UpdatedAt,
	)
	if err != nil {
		return domain.Design{}, err
	}
	design.Document = json.RawMessage(document)
	design.DocumentRevision = domain.DesignRevision(design.Document)
	if canvasSnapshot != nil {
		design.CanvasSnapshot = json.RawMessage(*canvasSnapshot)
	}
	return design, nil
}

func scanDesignVersion(row rowScanner) (domain.DesignVersion, error) {
	var version domain.DesignVersion
	var document string
	var canvasSnapshot *string
	err := row.Scan(
		&version.ID,
		&version.DesignID,
		&version.WorkspaceID,
		&version.VersionNumber,
		&version.Status,
		&version.Remarks,
		&document,
		&canvasSnapshot,
		&version.CreatedBy,
		&version.CreatedAt,
		&version.UpdatedAt,
	)
	if err != nil {
		return domain.DesignVersion{}, err
	}
	version.Document = json.RawMessage(document)
	if canvasSnapshot != nil {
		version.CanvasSnapshot = json.RawMessage(*canvasSnapshot)
	}
	return version, nil
}

func scanDesignDoc(row rowScanner) (domain.DesignDoc, error) {
	var doc domain.DesignDoc
	err := row.Scan(
		&doc.ID,
		&doc.WorkspaceID,
		&doc.DesignID,
		&doc.Title,
		&doc.Body,
		&doc.Format,
		&doc.CreatedBy,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	return doc, err
}

func scanUser(row rowScanner) (domain.User, error) {
	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.DisplayName,
		&user.Email,
		&user.PasswordSet,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.LastSeenAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func scanSignInConfig(row rowScanner) (domain.SignInConfig, error) {
	var config domain.SignInConfig
	err := row.Scan(
		&config.LocalPasswordEnabled,
		&config.SSOEnabled,
		&config.Provider,
		&config.OktaDomain,
		&config.Issuer,
		&config.ClientID,
		&config.ClientSecretSet,
		&config.RedirectURI,
		&config.PostLogoutRedirectURI,
		&config.Scopes,
		&config.GroupsClaim,
		&config.AdminGroup,
		&config.ReviewerGroup,
		&config.JITProvisioning,
		&config.UpdatedAt,
	)
	return config, err
}

func scanSignInConfigWithSecret(row rowScanner) (domain.SignInConfig, error) {
	var config domain.SignInConfig
	err := row.Scan(
		&config.LocalPasswordEnabled,
		&config.SSOEnabled,
		&config.Provider,
		&config.OktaDomain,
		&config.Issuer,
		&config.ClientID,
		&config.ClientSecret,
		&config.RedirectURI,
		&config.PostLogoutRedirectURI,
		&config.Scopes,
		&config.GroupsClaim,
		&config.AdminGroup,
		&config.ReviewerGroup,
		&config.JITProvisioning,
		&config.UpdatedAt,
	)
	config.ClientSecretSet = strings.TrimSpace(config.ClientSecret) != ""
	return config, err
}

func scanAIProviderConfig(row rowScanner) (domain.AIProviderConfig, error) {
	var config domain.AIProviderConfig
	var verifiedAt sql.NullTime
	err := row.Scan(
		&config.Enabled,
		&config.Provider,
		&config.Model,
		&config.BaseURL,
		&config.APIKey,
		&config.APIKeySet,
		&verifiedAt,
		&config.UpdatedAt,
	)
	if verifiedAt.Valid {
		config.VerifiedAt = verifiedAt.Time
	}
	return config, err
}

func scanWorkspaceAccess(row rowScanner) (domain.WorkspaceAccess, error) {
	var access domain.WorkspaceAccess
	err := row.Scan(
		&access.WorkspaceID,
		&access.UserID,
		&access.CanRead,
		&access.CanCreateDesign,
		&access.CanManage,
		&access.CreatedAt,
		&access.UpdatedAt,
	)
	return access, err
}

func scanAccessGroup(row rowScanner) (domain.AccessGroup, error) {
	var group domain.AccessGroup
	err := row.Scan(
		&group.ID,
		&group.Name,
		&group.Description,
		&group.OktaGroupName,
		&group.MemberCount,
		&group.CreatedAt,
		&group.UpdatedAt,
	)
	return group, err
}

func scanAccessGroupMember(row rowScanner) (domain.AccessGroupMember, error) {
	var member domain.AccessGroupMember
	err := row.Scan(&member.GroupID, &member.UserID, &member.AddedAt)
	return member, err
}

func scanWorkspaceGroupAccess(row rowScanner) (domain.WorkspaceGroupAccess, error) {
	var access domain.WorkspaceGroupAccess
	err := row.Scan(
		&access.WorkspaceID,
		&access.GroupID,
		&access.CanRead,
		&access.CanCreateDesign,
		&access.CanManage,
		&access.CreatedAt,
		&access.UpdatedAt,
	)
	return access, err
}

func scanDesignAccess(row rowScanner) (domain.DesignAccess, error) {
	var access domain.DesignAccess
	err := row.Scan(
		&access.WorkspaceID,
		&access.DesignID,
		&access.UserID,
		&access.CanRead,
		&access.CanEdit,
		&access.CanComment,
		&access.CanReview,
		&access.CanManage,
		&access.CreatedAt,
		&access.UpdatedAt,
	)
	return access, err
}

func scanDesignGroupAccess(row rowScanner) (domain.DesignGroupAccess, error) {
	var access domain.DesignGroupAccess
	err := row.Scan(
		&access.WorkspaceID,
		&access.DesignID,
		&access.GroupID,
		&access.CanRead,
		&access.CanEdit,
		&access.CanComment,
		&access.CanReview,
		&access.CanManage,
		&access.CreatedAt,
		&access.UpdatedAt,
	)
	return access, err
}

func scanMCPConfig(row rowScanner) (domain.MCPConfig, error) {
	var config domain.MCPConfig
	err := row.Scan(
		&config.Enabled,
		&config.EndpointPath,
		&config.ReadCatalog,
		&config.ReadDesigns,
		&config.CreateDraftDesign,
		&config.RunAnalysis,
		&config.FetchImpactReport,
		&config.RequireAdminConsent,
		&config.UpdatedAt,
	)
	return config, err
}

func scanTelemetryIntegrationConfig(row rowScanner) (domain.TelemetryIntegrationConfig, error) {
	var config domain.TelemetryIntegrationConfig
	var filters string
	err := row.Scan(
		&config.Enabled,
		&config.Provider,
		&config.DisplayName,
		&config.BaseURL,
		&config.AuthMode,
		&config.Secret,
		&config.SecretSet,
		&config.CustomHeaderName,
		&config.QueryWindow,
		&config.RequestTotalMetric,
		&config.RequestFailedMetric,
		&config.ServerLatencyMetric,
		&config.ClientLatencyMetric,
		&filters,
		&config.UpdatedAt,
	)
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	if strings.TrimSpace(filters) != "" {
		if err := json.Unmarshal([]byte(filters), &config.Filters); err != nil {
			return domain.TelemetryIntegrationConfig{}, err
		}
	}
	return config, nil
}

func scanDesignComment(row rowScanner) (domain.DesignComment, error) {
	var comment domain.DesignComment
	err := row.Scan(
		&comment.ID,
		&comment.WorkspaceID,
		&comment.DesignID,
		&comment.AuthorID,
		&comment.Body,
		&comment.ComponentID,
		&comment.ConnectorID,
		&comment.CreatedAt,
	)
	return comment, err
}

func scanDesignReviewRequest(row rowScanner) (domain.DesignReviewRequest, error) {
	var review domain.DesignReviewRequest
	err := row.Scan(
		&review.ID,
		&review.WorkspaceID,
		&review.DesignID,
		&review.VersionID,
		&review.VersionNumber,
		&review.RequestedBy,
		&review.ReviewerID,
		&review.Status,
		&review.Message,
		&review.Summary,
		&review.CreatedAt,
		&review.UpdatedAt,
		&review.CompletedAt,
	)
	return review, err
}

func scanNotification(row rowScanner) (domain.Notification, error) {
	var notification domain.Notification
	err := row.Scan(
		&notification.ID,
		&notification.UserID,
		&notification.WorkspaceID,
		&notification.DesignID,
		&notification.Type,
		&notification.Title,
		&notification.Body,
		&notification.Read,
		&notification.CreatedAt,
	)
	return notification, err
}

func scanCatalogAsset(row rowScanner) (domain.CatalogAsset, error) {
	var asset domain.CatalogAsset
	var aliases string
	var tags string
	var metadata string
	err := row.Scan(
		&asset.ID,
		&asset.Name,
		&asset.NormalizedName,
		&asset.Kind,
		&asset.Type,
		&asset.Owner,
		&asset.Description,
		&asset.Criticality,
		&asset.Status,
		&aliases,
		&asset.ReplacementAssetID,
		&asset.UpdateMessage,
		&tags,
		&metadata,
		&asset.CreatedBy,
		&asset.CreatedAt,
		&asset.UpdatedAt,
		&asset.UsedInDesignCount,
	)
	if err != nil {
		return domain.CatalogAsset{}, err
	}
	asset.Kind = normalizedCatalogAssetKind(asset.Kind)
	asset.Status = normalizedCatalogAssetStatus(asset.Status)
	asset.Aliases = splitStoredTags(aliases)
	asset.Tags = splitStoredTags(tags)
	asset.Metadata = json.RawMessage(metadata)
	return asset, nil
}

func splitStoredTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tags = append(tags, part)
		}
	}
	return tags
}

func ensureWorkspaceExists(ctx context.Context, tx pgx.Tx, workspaceID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workspaces WHERE id = $1)`, workspaceID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("workspace not found")
	}
	return nil
}

func selectDesignForUpdate(ctx context.Context, tx pgx.Tx, workspaceID string, designID string) (domain.Design, error) {
	row := tx.QueryRow(ctx, `
SELECT id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at
FROM designs
WHERE workspace_id = $1 AND id = $2
FOR UPDATE
`, workspaceID, designID)
	return scanDesign(row)
}

func ensureDesignExists(ctx context.Context, tx pgx.Tx, workspaceID string, designID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM designs WHERE workspace_id = $1 AND id = $2)`, workspaceID, designID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("design not found")
	}
	return nil
}

func createUserWithTx(ctx context.Context, tx pgx.Tx, now time.Time, displayName string, email string, role string, password string) (domain.User, error) {
	displayName = strings.TrimSpace(displayName)
	email = strings.ToLower(strings.TrimSpace(email))
	if displayName == "" {
		return domain.User{}, errors.New("display name is required")
	}
	if email == "" {
		return domain.User{}, errors.New("email is required")
	}
	passwordHash := ""
	if strings.TrimSpace(password) != "" {
		var err error
		passwordHash, err = hashPassword(password)
		if err != nil {
			return domain.User{}, err
		}
	}
	user := domain.User{
		ID:           fmt.Sprintf("user_%d", now.UnixNano()),
		DisplayName:  displayName,
		Email:        email,
		PasswordSet:  passwordHash != "",
		PasswordHash: passwordHash,
		Role:         normalizedUserRole(role),
		Status:       "active",
		LastSeenAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO users (id, display_name, email, password_hash, role, status, last_seen_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $7)
`, user.ID, user.DisplayName, user.Email, user.PasswordHash, user.Role, user.Status, now); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func claimLegacyGuestData(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, `UPDATE workspaces SET owner_id = $1, updated_at = NOW() WHERE id = $2 AND owner_id IS NULL`, userID, domain.GuestWorkspaceID); err != nil {
		return err
	}
	legacyIDs := []string{"", "guest-user"}
	for _, legacyID := range legacyIDs {
		if _, err := tx.Exec(ctx, `UPDATE designs SET created_by = $1 WHERE created_by = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE design_versions SET created_by = $1 WHERE created_by = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE design_docs SET created_by = $1 WHERE created_by = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE design_comments SET author_id = $1 WHERE author_id = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE design_review_requests SET requested_by = $1 WHERE requested_by = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE notifications SET user_id = $1 WHERE user_id = $2`, userID, legacyID); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultSignInConfig(ctx context.Context, tx pgx.Tx, now time.Time) error {
	config := defaultSignInConfig(now)
	_, err := tx.Exec(ctx, `
INSERT INTO sign_in_settings (
	id, local_password_enabled, sso_enabled, provider, okta_domain, issuer, client_id, client_secret,
	redirect_uri, post_logout_redirect_uri, scopes, groups_claim, admin_group, reviewer_group, jit_provisioning, updated_at
)
VALUES ('default', $1, $2, $3, '', '', '', '', '', '', $4, $5, '', '', $6, $7)
ON CONFLICT (id) DO NOTHING
`, config.LocalPasswordEnabled, config.SSOEnabled, config.Provider, config.Scopes, config.GroupsClaim, config.JITProvisioning, config.UpdatedAt)
	return err
}

func ensureDefaultAIProviderConfig(ctx context.Context, tx pgx.Tx, now time.Time) error {
	config := defaultAIProviderConfig(now)
	_, err := tx.Exec(ctx, `
INSERT INTO ai_provider_settings (id, enabled, provider, model, base_url, api_key, verified_at, updated_at)
VALUES ('default', $1, $2, $3, $4, '', NULL, $5)
ON CONFLICT (id) DO NOTHING
`, config.Enabled, config.Provider, config.Model, config.BaseURL, config.UpdatedAt)
	return err
}

func ensureDefaultMCPConfig(ctx context.Context, tx pgx.Tx, now time.Time) error {
	config := defaultMCPConfig(now)
	_, err := tx.Exec(ctx, `
INSERT INTO mcp_settings (
	id, enabled, endpoint_path, read_catalog, read_designs, create_draft_design,
	run_analysis, fetch_impact_report, require_admin_consent, updated_at
)
VALUES ('default', $1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO NOTHING
`, config.Enabled, config.EndpointPath, config.ReadCatalog, config.ReadDesigns, config.CreateDraftDesign, config.RunAnalysis, config.FetchImpactReport, config.RequireAdminConsent, config.UpdatedAt)
	return err
}

func ensureDefaultTelemetryIntegrationConfig(ctx context.Context, tx pgx.Tx, now time.Time) error {
	config := defaultTelemetryIntegrationConfig(now)
	filterBytes, err := json.Marshal(config.Filters)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO telemetry_integration_settings (
	id, enabled, provider, display_name, base_url, auth_mode, secret, custom_header_name, query_window,
	request_total_metric, request_failed_metric, server_latency_metric, client_latency_metric, filters, updated_at
)
VALUES ('default', $1, $2, $3, $4, $5, '', $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (id) DO NOTHING
`, config.Enabled, config.Provider, config.DisplayName, config.BaseURL, config.AuthMode, config.CustomHeaderName,
		config.QueryWindow, config.RequestTotalMetric, config.RequestFailedMetric, config.ServerLatencyMetric, config.ClientLatencyMetric, string(filterBytes), config.UpdatedAt)
	return err
}

func getTelemetryIntegrationConfigWithTx(ctx context.Context, tx pgx.Tx) (domain.TelemetryIntegrationConfig, error) {
	row := tx.QueryRow(ctx, `
SELECT enabled, provider, display_name, base_url, auth_mode, secret, secret <> '', custom_header_name,
	query_window, request_total_metric, request_failed_metric, server_latency_metric, client_latency_metric, filters, updated_at
FROM telemetry_integration_settings
WHERE id = 'default'
FOR UPDATE
`)
	return scanTelemetryIntegrationConfig(row)
}

func getSignInConfigWithTx(ctx context.Context, tx pgx.Tx) (domain.SignInConfig, error) {
	row := tx.QueryRow(ctx, `
SELECT local_password_enabled, sso_enabled, provider, okta_domain, issuer, client_id, client_secret <> '', redirect_uri,
	post_logout_redirect_uri, scopes, groups_claim, admin_group, reviewer_group, jit_provisioning, updated_at
FROM sign_in_settings
WHERE id = 'default'
`)
	return scanSignInConfig(row)
}

func (r *PostgresRepository) firstUserID(ctx context.Context) string {
	var userID string
	_ = r.pool.QueryRow(ctx, `
SELECT id
FROM users
WHERE status <> 'disabled'
ORDER BY created_at ASC
LIMIT 1
`).Scan(&userID)
	return userID
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func isUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && (constraintName == "" || pgErr.ConstraintName == constraintName)
}

func nullableJSONText(value json.RawMessage) *string {
	if len(value) == 0 {
		return nil
	}
	text := string(value)
	return &text
}

func nullableTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func notificationFor(userID string, workspaceID string, designID string, notificationType string, title string, body string, now time.Time) domain.Notification {
	return domain.Notification{
		ID:          fmt.Sprintf("notification_%d_%s", now.UnixNano(), userID),
		UserID:      userID,
		WorkspaceID: workspaceID,
		DesignID:    designID,
		Type:        notificationType,
		Title:       title,
		Body:        body,
		Read:        false,
		CreatedAt:   now,
	}
}

func insertNotification(ctx context.Context, tx pgx.Tx, notification domain.Notification) error {
	if strings.TrimSpace(notification.UserID) == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
INSERT INTO notifications (id, user_id, workspace_id, design_id, type, title, body, read, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, notification.ID, notification.UserID, notification.WorkspaceID, notification.DesignID, notification.Type, notification.Title, notification.Body, notification.Read, notification.CreatedAt)
	return err
}
