package store

import (
	"context"
	"encoding/json"
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
		CreatedBy:   domain.GuestUserID,
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
		CreatedBy:   domain.GuestUserID,
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

	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Initial", nil)
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

func TestMemoryRepositoryCreateDesignPreservesProvidedDocumentBytes(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}

	document := json.RawMessage("{\n  \"schemaVersion\": \"sde-ui/v0.1\",\n  \"components\": []\n}")
	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Exact JSON", document)
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

	created, err := repo.CreateDesign(context.Background(), workspace.ID, "Generated", json.RawMessage("null"))
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
