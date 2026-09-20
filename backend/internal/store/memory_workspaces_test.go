package store

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"testing"
	"time"
)

func TestFirstAdminClaimsGuestWorkspaceOwnership(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.GetOrCreateGuestWorkspace(context.Background()); err != nil {
		t.Fatal(err)
	}
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := repo.GetWorkspace(context.Background(), domain.GuestWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.OwnerID != admin.ID {
		t.Fatalf("owner = %q, want %q", workspace.OwnerID, admin.ID)
	}
}

func TestAccessiblePaginationFiltersBeforeApplyingLimit(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, time.August, 28, 10, 0, 0, 0, time.UTC)
	repo.clock = func() time.Time { return now }
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	member, err := repo.CreateUser(context.Background(), "Member", "member@example.com", "member", "")
	if err != nil {
		t.Fatal(err)
	}
	accessible, err := repo.CreateWorkspace(context.Background(), "Accessible", admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantWorkspaceAccess(context.Background(), domain.WorkspaceAccess{WorkspaceID: accessible.ID, UserID: member.ID, CanRead: true}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := repo.CreateWorkspace(context.Background(), "Newer private", admin.ID); err != nil {
		t.Fatal(err)
	}
	items, page, err := repo.ListAccessibleWorkspacesPage(context.Background(), AccessScope{UserID: member.ID}, PageOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != accessible.ID || page.HasMore {
		t.Fatalf("page = %#v %#v, want only accessible workspace", items, page)
	}
}

func TestMemoryRepositoryRejectsWorkspaceDeleteWhenNotEmpty(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.CreateWorkspace(context.Background(), "Disposable Workspace", "")
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
