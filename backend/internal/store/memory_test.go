package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/domain"
)

func TestMemoryRepositoryGuestWorkspace(t *testing.T) {
	repo := NewMemoryRepository()

	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	if workspace.ID != domain.GuestWorkspaceID {
		t.Fatalf("workspace id = %q, want %q", workspace.ID, domain.GuestWorkspaceID)
	}

	second, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("second GetOrCreateGuestWorkspace returned error: %v", err)
	}
	if !workspace.CreatedAt.Equal(second.CreatedAt) {
		t.Fatalf("guest workspace should be stable across calls")
	}
}

func TestMemoryRepositoryRejectsDuplicateUserEmail(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	if _, err := repo.CreateUser(context.Background(), "Duplicate", "ADMIN@example.com", "member", ""); err == nil || !strings.Contains(err.Error(), "email already exists") {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

func TestMemoryRepositoryRejectsDuplicateEmailOnUpdate(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	user, err := repo.CreateUser(context.Background(), "Tarun", "tarun@example.com", "member", "")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if _, err := repo.UpdateUser(context.Background(), user.ID, "", "admin@example.com", "", "", ""); err == nil || !strings.Contains(err.Error(), "email already exists") {
		t.Fatalf("expected duplicate email update error, got %v", err)
	}
}

func TestMemoryRepositoryUpsertDesign(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}

	document := json.RawMessage(`{"id":"design-1","title":"Initial"}`)
	created, err := repo.UpsertDesign(context.Background(), domain.Design{
		ID:          "design-1",
		WorkspaceID: workspace.ID,
		Name:        "Initial",
		Access:      "private",
		Title:       "Initial",
		Document:    document,
		CreatedBy:   "user_test",
	})
	if err != nil {
		t.Fatalf("UpsertDesign returned error: %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("created design timestamps should be populated")
	}

	time.Sleep(time.Millisecond)

	updated, err := repo.UpsertDesign(context.Background(), domain.Design{
		ID:          "design-1",
		WorkspaceID: workspace.ID,
		Title:       "Updated",
		Document:    json.RawMessage(`{"id":"design-1","title":"Updated"}`),
		CreatedBy:   "user_test",
	})
	if err != nil {
		t.Fatalf("second UpsertDesign returned error: %v", err)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("created timestamp should be preserved")
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("updated timestamp should advance")
	}

	designs, err := repo.ListDesigns(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("ListDesigns returned error: %v", err)
	}
	if len(designs) != 1 {
		t.Fatalf("design count = %d, want 1", len(designs))
	}
	if designs[0].Name != "Initial" {
		t.Fatalf("stored name = %q, want Initial", designs[0].Name)
	}
}

func TestMemoryRepositoryUpdateDesignMetadataDoesNotCreateVersion(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}

	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Initial", nil, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	updated, err := repo.UpdateDesignMetadata(context.Background(), workspace.ID, created.ID, "Renamed", "workspace")
	if err != nil {
		t.Fatalf("UpdateDesignMetadata returned error: %v", err)
	}
	if updated.Name != "Renamed" {
		t.Fatalf("updated name = %q, want Renamed", updated.Name)
	}
	if updated.Access != "workspace" {
		t.Fatalf("updated access = %q, want workspace", updated.Access)
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, created.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("version count = %d, want 0", len(versions))
	}
}

func TestMemoryRepositoryManualDesignVersions(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Initial", nil, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}

	version, err := repo.CreateDesignVersion(context.Background(), workspace.ID, created.ID, "user_test", "")
	if err != nil {
		t.Fatalf("CreateDesignVersion returned error: %v", err)
	}
	if version.VersionNumber != 1 || version.Status != "draft" {
		t.Fatalf("version = (%d, %q), want (1, draft)", version.VersionNumber, version.Status)
	}
	if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, created.ID, version.ID, "live"); err == nil {
		t.Fatal("draft version should not transition directly to live")
	}
	pending, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, created.ID, version.ID, "pending_review")
	if err != nil {
		t.Fatalf("UpdateDesignVersionStatus pending returned error: %v", err)
	}
	if pending.Status != "pending_review" {
		t.Fatalf("version status = %q, want pending_review", pending.Status)
	}
	reviewed, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, created.ID, version.ID, "reviewed")
	if err != nil {
		t.Fatalf("UpdateDesignVersionStatus reviewed returned error: %v", err)
	}
	if reviewed.Status != "reviewed" {
		t.Fatalf("version status = %q, want reviewed", reviewed.Status)
	}
	live, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, created.ID, version.ID, "live")
	if err != nil {
		t.Fatalf("UpdateDesignVersionStatus live returned error: %v", err)
	}
	if live.Status != "live" {
		t.Fatalf("version status = %q, want live", live.Status)
	}
}

func TestMemoryRepositoryDesignChangePushesEditableVersionToDraft(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Initial", nil, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	version, err := repo.CreateDesignVersion(context.Background(), workspace.ID, created.ID, "user_test", "")
	if err != nil {
		t.Fatalf("CreateDesignVersion returned error: %v", err)
	}
	if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, created.ID, version.ID, "pending_review"); err != nil {
		t.Fatalf("UpdateDesignVersionStatus pending returned error: %v", err)
	}
	if _, err := repo.UpsertDesign(context.Background(), domain.Design{
		ID:          created.ID,
		WorkspaceID: created.WorkspaceID,
		Document:    json.RawMessage(`{"id":"changed","title":"Changed","components":[]}`),
	}); err != nil {
		t.Fatalf("UpsertDesign returned error: %v", err)
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, created.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	if len(versions) != 1 || versions[0].Status != "draft" {
		t.Fatalf("versions = %#v, want one draft version", versions)
	}
}

func TestMemoryRepositoryReviewRequestsAreVersionScopedAndBulkCreated(t *testing.T) {
	repo := NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	reviewerOne, err := repo.CreateUser(context.Background(), "Reviewer One", "reviewer1@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser reviewer one returned error: %v", err)
	}
	reviewerTwo, err := repo.CreateUser(context.Background(), "Reviewer Two", "reviewer2@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser reviewer two returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Reviewable", nil, admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	version, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion returned error: %v", err)
	}

	reviews, err := repo.CreateDesignReviewRequests(context.Background(), workspace.ID, design.ID, version.ID, admin.ID, []string{reviewerOne.ID, reviewerTwo.ID, reviewerOne.ID}, "please review")
	if err != nil {
		t.Fatalf("CreateDesignReviewRequests returned error: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("review count = %d, want 2", len(reviews))
	}
	for _, review := range reviews {
		if review.VersionID != version.ID || review.VersionNumber != version.VersionNumber || review.Status != "requested" {
			t.Fatalf("review version/status = %#v, want version %q requested", review, version.ID)
		}
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	if versions[0].Status != "pending_review" {
		t.Fatalf("version status = %q, want pending_review", versions[0].Status)
	}

	if _, err := repo.UpdateDesignReviewRequest(context.Background(), workspace.ID, design.ID, reviews[0].ID, "approved", "ok"); err != nil {
		t.Fatalf("UpdateDesignReviewRequest first approval returned error: %v", err)
	}
	versions, err = repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions after first approval returned error: %v", err)
	}
	if versions[0].Status != "pending_review" {
		t.Fatalf("version status after one approval = %q, want pending_review", versions[0].Status)
	}
	if _, err := repo.UpdateDesignReviewRequest(context.Background(), workspace.ID, design.ID, reviews[1].ID, "approved", "ok"); err != nil {
		t.Fatalf("UpdateDesignReviewRequest second approval returned error: %v", err)
	}
	versions, err = repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions after all approvals returned error: %v", err)
	}
	if versions[0].Status != "reviewed" {
		t.Fatalf("version status after all approvals = %q, want reviewed", versions[0].Status)
	}
}

func TestMemoryRepositoryReviewChangesRequestedDoesNotMarkVersionReviewed(t *testing.T) {
	repo := NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	reviewer, err := repo.CreateUser(context.Background(), "Reviewer", "reviewer@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser reviewer returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Reviewable", nil, admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	version, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion returned error: %v", err)
	}
	reviews, err := repo.CreateDesignReviewRequests(context.Background(), workspace.ID, design.ID, version.ID, admin.ID, []string{reviewer.ID}, "please review")
	if err != nil {
		t.Fatalf("CreateDesignReviewRequests returned error: %v", err)
	}
	if _, err := repo.UpdateDesignReviewRequest(context.Background(), workspace.ID, design.ID, reviews[0].ID, "changes_requested", "needs work"); err != nil {
		t.Fatalf("UpdateDesignReviewRequest changes requested returned error: %v", err)
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	if versions[0].Status != "pending_review" {
		t.Fatalf("version status = %q, want pending_review after changes requested", versions[0].Status)
	}
}

func TestMemoryRepositoryOnlyOneLiveVersion(t *testing.T) {
	repo := NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Versioned", nil, admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	versionOne, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion one returned error: %v", err)
	}
	versionTwo, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion two returned error: %v", err)
	}
	for _, version := range []domain.DesignVersion{versionOne, versionTwo} {
		if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, design.ID, version.ID, "pending_review"); err != nil {
			t.Fatalf("UpdateDesignVersionStatus pending for %s returned error: %v", version.ID, err)
		}
		if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, design.ID, version.ID, "reviewed"); err != nil {
			t.Fatalf("UpdateDesignVersionStatus reviewed for %s returned error: %v", version.ID, err)
		}
	}
	if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, design.ID, versionOne.ID, "live"); err != nil {
		t.Fatalf("UpdateDesignVersionStatus live one returned error: %v", err)
	}
	if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, design.ID, versionTwo.ID, "live"); err != nil {
		t.Fatalf("UpdateDesignVersionStatus live two returned error: %v", err)
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	liveCount := 0
	for _, version := range versions {
		if version.Status == "live" {
			liveCount++
			if version.ID != versionTwo.ID {
				t.Fatalf("live version id = %q, want %q", version.ID, versionTwo.ID)
			}
		}
		if version.ID == versionOne.ID && version.Status != "reviewed" {
			t.Fatalf("previous live version status = %q, want reviewed", version.Status)
		}
	}
	if liveCount != 1 {
		t.Fatalf("live version count = %d, want 1", liveCount)
	}
}

func TestMemoryRepositoryDeletesDesignVersionAndVersionReviews(t *testing.T) {
	repo := NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	reviewer, err := repo.CreateUser(context.Background(), "Reviewer", "reviewer@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser reviewer returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Reviewable", nil, admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	versionOne, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion one returned error: %v", err)
	}
	versionTwo, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion two returned error: %v", err)
	}
	if _, err := repo.CreateDesignReviewRequests(context.Background(), workspace.ID, design.ID, versionTwo.ID, admin.ID, []string{reviewer.ID}, "please review"); err != nil {
		t.Fatalf("CreateDesignReviewRequests returned error: %v", err)
	}

	if err := repo.DeleteDesignVersion(context.Background(), workspace.ID, design.ID, versionTwo.ID); err != nil {
		t.Fatalf("DeleteDesignVersion returned error: %v", err)
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	if len(versions) != 1 || versions[0].ID != versionOne.ID {
		t.Fatalf("versions = %#v, want only version one", versions)
	}
	reviews, err := repo.ListDesignReviewRequests(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignReviewRequests returned error: %v", err)
	}
	if len(reviews) != 0 {
		t.Fatalf("review count after version delete = %d, want 0", len(reviews))
	}
	updatedDesign, err := repo.GetDesign(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("GetDesign returned error: %v", err)
	}
	if updatedDesign.VersionNumber != versionOne.VersionNumber {
		t.Fatalf("design version number = %d, want %d", updatedDesign.VersionNumber, versionOne.VersionNumber)
	}
}

func TestMemoryRepositoryDeletesDesign(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}

	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Disposable", nil, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	if err := repo.DeleteDesign(context.Background(), workspace.ID, created.ID); err != nil {
		t.Fatalf("DeleteDesign returned error: %v", err)
	}
	if _, err := repo.GetDesign(context.Background(), workspace.ID, created.ID); err == nil {
		t.Fatal("deleted design should not be retrievable")
	}
	if _, err := repo.ListDesignVersions(context.Background(), workspace.ID, created.ID); err == nil {
		t.Fatal("deleted design versions should not be retrievable")
	}
}

func TestMemoryRepositoryManagesDesignDocsSeparately(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Documented Design", nil, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}

	doc, err := repo.CreateDesignDoc(context.Background(), workspace.ID, design.ID, "Runbook", "<p>hello</p>", "html")
	if err != nil {
		t.Fatalf("CreateDesignDoc returned error: %v", err)
	}
	if doc.WorkspaceID != workspace.ID || doc.DesignID != design.ID {
		t.Fatalf("doc belongs to workspace/design %q/%q, want %q/%q", doc.WorkspaceID, doc.DesignID, workspace.ID, design.ID)
	}
	if doc.Title != "Runbook" || doc.Body != "<p>hello</p>" || doc.Format != "html" {
		t.Fatalf("doc fields were not preserved: %#v", doc)
	}

	docs, err := repo.ListDesignDocs(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignDocs returned error: %v", err)
	}
	if len(docs) != 1 || docs[0].ID != doc.ID {
		t.Fatalf("docs = %#v, want one doc %q", docs, doc.ID)
	}

	updated, err := repo.UpdateDesignDoc(context.Background(), workspace.ID, design.ID, doc.ID, "Updated Runbook", "<p>bye</p>", "html")
	if err != nil {
		t.Fatalf("UpdateDesignDoc returned error: %v", err)
	}
	if updated.Title != "Updated Runbook" || updated.Body != "<p>bye</p>" {
		t.Fatalf("updated doc fields were not stored: %#v", updated)
	}

	storedDesign, err := repo.GetDesign(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("GetDesign returned error: %v", err)
	}
	if string(storedDesign.Document) != string(design.Document) {
		t.Fatal("updating docs should not rewrite the structured design document")
	}

	if err := repo.DeleteDesignDoc(context.Background(), workspace.ID, design.ID, doc.ID); err != nil {
		t.Fatalf("DeleteDesignDoc returned error: %v", err)
	}
	docs, err = repo.ListDesignDocs(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignDocs after delete returned error: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("doc count after delete = %d, want 0", len(docs))
	}
}

func TestMemoryRepositoryDeletesDesignDocsWithDesign(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Disposable With Docs", nil, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	doc, err := repo.CreateDesignDoc(context.Background(), workspace.ID, design.ID, "Notes", "<p>keep until design delete</p>", "html")
	if err != nil {
		t.Fatalf("CreateDesignDoc returned error: %v", err)
	}

	if err := repo.DeleteDesign(context.Background(), workspace.ID, design.ID); err != nil {
		t.Fatalf("DeleteDesign returned error: %v", err)
	}
	if _, err := repo.GetDesignDoc(context.Background(), workspace.ID, design.ID, doc.ID); err == nil {
		t.Fatal("design doc should be deleted with its design")
	}
}

func TestMemoryRepositoryRejectsWorkspaceDeleteWhenNotEmpty(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.CreateWorkspace(context.Background(), "Disposable Workspace")
	if err != nil {
		t.Fatalf("CreateWorkspace returned error: %v", err)
	}
	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Disposable", nil, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}

	if err := repo.DeleteWorkspace(context.Background(), workspace.ID); err == nil {
		t.Fatal("DeleteWorkspace should reject non-empty workspaces")
	}
	if _, err := repo.GetWorkspace(context.Background(), workspace.ID); err != nil {
		t.Fatal("workspace should remain after rejected delete")
	}
	if _, err := repo.GetDesign(context.Background(), workspace.ID, created.ID); err != nil {
		t.Fatal("design should remain after rejected workspace delete")
	}

	if err := repo.DeleteDesign(context.Background(), workspace.ID, created.ID); err != nil {
		t.Fatalf("DeleteDesign returned error: %v", err)
	}
	if err := repo.DeleteWorkspace(context.Background(), workspace.ID); err != nil {
		t.Fatalf("DeleteWorkspace returned error after emptying workspace: %v", err)
	}
	if _, err := repo.GetWorkspace(context.Background(), workspace.ID); err == nil {
		t.Fatal("deleted empty workspace should not be retrievable")
	}
}

func TestMemoryRepositoryDoesNotDeleteGuestWorkspace(t *testing.T) {
	repo := NewMemoryRepository()
	if err := repo.DeleteWorkspace(context.Background(), domain.GuestWorkspaceID); err == nil {
		t.Fatal("guest workspace delete should be rejected")
	}
}

func TestMemoryRepositoryCreateDesignPreservesProvidedDocumentBytes(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}

	document := json.RawMessage("{\n  \"schemaVersion\": \"sde-ui/v0.1\",\n  \"components\": []\n}")
	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Exact JSON", document, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	if string(created.Document) != string(document) {
		t.Fatalf("document was not preserved exactly:\n got: %q\nwant: %q", string(created.Document), string(document))
	}
}

func TestMemoryRepositoryCreateDesignGeneratesDocumentForNullInput(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}

	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Generated", json.RawMessage("null"), "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	if string(created.Document) == "null" {
		t.Fatal("null document should generate a default design document")
	}
	var document struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Document, &document); err != nil {
		t.Fatalf("created document is not JSON: %v", err)
	}
	if document.ID != created.ID {
		t.Fatalf("document id = %q, want %q", document.ID, created.ID)
	}
}

func TestMemoryRepositoryCatalogAssetsRejectDuplicateNormalizedNames(t *testing.T) {
	repo := NewMemoryRepository()

	created, err := repo.CreateCatalogAsset(context.Background(), domain.CatalogAsset{
		Name:      " Purchase   Service ",
		Type:      "compute.service",
		CreatedBy: "user_test",
	})
	if err != nil {
		t.Fatalf("CreateCatalogAsset returned error: %v", err)
	}
	if created.NormalizedName != "purchase service" {
		t.Fatalf("normalized name = %q, want purchase service", created.NormalizedName)
	}

	if _, err := repo.CreateCatalogAsset(context.Background(), domain.CatalogAsset{Name: "purchase service", Type: "compute.service"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate catalog asset error, got %v", err)
	}
}

func TestMemoryRepositoryCatalogAssetsTrackUsageAndBlockDelete(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	asset, err := repo.CreateCatalogAsset(context.Background(), domain.CatalogAsset{
		Name:      "Notification Service",
		Type:      "compute.service",
		CreatedBy: "user_test",
	})
	if err != nil {
		t.Fatalf("CreateCatalogAsset returned error: %v", err)
	}
	document := json.RawMessage(`{"components":[{"metadata":{"enterpriseAsset":{"assetId":"` + asset.ID + `"}}}]}`)
	if _, err := repo.CreateDesign(context.Background(), workspace.ID, "Uses Catalog", document, "user_test"); err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}

	assets, err := repo.ListCatalogAssets(context.Background(), "notification")
	if err != nil {
		t.Fatalf("ListCatalogAssets returned error: %v", err)
	}
	if len(assets) != 1 || assets[0].UsedInDesignCount != 1 {
		t.Fatalf("catalog assets = %#v, want one asset with one linked design", assets)
	}
	if err := repo.DeleteCatalogAsset(context.Background(), asset.ID); err == nil || !strings.Contains(err.Error(), "linked") {
		t.Fatalf("expected linked asset delete rejection, got %v", err)
	}
}

func TestMemoryRepositoryMCPConfigPersistsAdminSettings(t *testing.T) {
	repo := NewMemoryRepository()

	initial, err := repo.GetMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("GetMCPConfig returned error: %v", err)
	}
	if initial.Enabled || initial.EndpointPath != "/mcp" || !initial.RequireAdminConsent {
		t.Fatalf("initial mcp config = %#v", initial)
	}

	updated, err := repo.UpdateMCPConfig(context.Background(), domain.MCPConfig{
		Enabled:             true,
		EndpointPath:        "stratum-mcp",
		ReadCatalog:         true,
		ReadDesigns:         true,
		CreateDraftDesign:   false,
		RunAnalysis:         true,
		FetchImpactReport:   false,
		RequireAdminConsent: true,
	})
	if err != nil {
		t.Fatalf("UpdateMCPConfig returned error: %v", err)
	}
	if !updated.Enabled || updated.EndpointPath != "/stratum-mcp" || updated.CreateDraftDesign {
		t.Fatalf("updated mcp config = %#v", updated)
	}
}
