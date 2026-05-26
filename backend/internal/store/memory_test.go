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
	if len(versions) != 1 {
		t.Fatalf("version count = %d, want 1", len(versions))
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
