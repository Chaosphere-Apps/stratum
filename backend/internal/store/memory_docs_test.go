package store

import (
	"context"
	"testing"
)

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
