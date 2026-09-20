package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
	"testing"
	"time"
)

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

func TestMemoryRepositoryUpdateDesignDocumentRejectsStaleRevision(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	design, err := repo.CreateDesign(ctx, domain.GuestWorkspaceID, "Payments", []byte(`{"id":"design-1","title":"Payments","components":[]}`), "user-1")
	if err != nil {
		t.Fatal(err)
	}

	first, err := repo.UpdateDesignDocument(
		ctx,
		domain.GuestWorkspaceID,
		design.ID,
		[]byte(`{"id":"design-1","title":"Payments","components":[{"id":"api"}]}`),
		nil,
		design.DocumentRevision,
	)
	if err != nil {
		t.Fatalf("first conditional update returned error: %v", err)
	}
	if !first.UpdatedAt.After(design.UpdatedAt) {
		t.Fatalf("updated timestamp = %s, want after %s", first.UpdatedAt, design.UpdatedAt)
	}

	_, err = repo.UpdateDesignDocument(
		ctx,
		domain.GuestWorkspaceID,
		design.ID,
		[]byte(`{"id":"design-1","title":"Payments","components":[{"id":"stale"}]}`),
		nil,
		design.DocumentRevision,
	)
	if !errors.Is(err, ErrDesignConflict) {
		t.Fatalf("stale conditional update error = %v, want ErrDesignConflict", err)
	}

	stored, err := repo.GetDesign(ctx, domain.GuestWorkspaceID, design.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stored.Document), `"api"`) || strings.Contains(string(stored.Document), `"stale"`) {
		t.Fatalf("stored document was overwritten by stale editor: %s", stored.Document)
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

func TestMemoryRepositoryWorkingDesignChangeDoesNotMutateSavedVersion(t *testing.T) {
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
	if len(versions) != 1 || versions[0].Status != "pending_review" {
		t.Fatalf("versions = %#v, want one immutable pending-review version", versions)
	}
	if string(versions[0].Document) == `{"id":"changed","title":"Changed","components":[]}` {
		t.Fatal("working-document save mutated the saved version snapshot")
	}
	if _, err := repo.UpdateDraftDesignVersion(context.Background(), workspace.ID, created.ID, version.ID, json.RawMessage(`{"id":"changed"}`), nil, ""); err == nil {
		t.Fatal("pending-review version accepted a draft override")
	}
}

func TestMemoryRepositoryExplicitDraftOverrideUpdatesSnapshot(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Initial", nil, "user_test")
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	version, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, "user_test", "")
	if err != nil {
		t.Fatalf("CreateDesignVersion returned error: %v", err)
	}
	updated, err := repo.UpdateDraftDesignVersion(context.Background(), workspace.ID, design.ID, version.ID, json.RawMessage(`{"id":"changed"}`), json.RawMessage(`{"camera":{"x":1}}`), "refined")
	if err != nil {
		t.Fatalf("UpdateDraftDesignVersion returned error: %v", err)
	}
	if string(updated.Document) != `{"id":"changed"}` || updated.Remarks != "refined" {
		t.Fatalf("updated version = %#v", updated)
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
