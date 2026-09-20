package app

import (
	"context"
	"testing"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

func TestWorkspaceServiceCreateWithAccess(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemoryRepository()
	owner, err := repo.CreateFirstAdmin(ctx, "Owner", "owner@example.com", "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	viewer, err := repo.CreateUser(ctx, "Reviewer", "reviewer@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	group, err := repo.CreateAccessGroup(ctx, domain.AccessGroup{Name: "Architecture"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	service := WorkspaceService{provider: StaticRepositoryProvider{Repo: repo}}

	workspace, err := service.CreateWithAccess(ctx, "Payments", owner.ID, []WorkspaceShare{
		{PrincipalType: "user", PrincipalID: viewer.ID, AccessLevel: "viewer"},
		{PrincipalType: "group", PrincipalID: group.ID, AccessLevel: "contributor"},
	})
	if err != nil {
		t.Fatalf("CreateWithAccess: %v", err)
	}

	userAccess, err := repo.ListWorkspaceAccess(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("list user access: %v", err)
	}
	if len(userAccess) != 2 {
		t.Fatalf("user access count = %d, want 2", len(userAccess))
	}
	for _, access := range userAccess {
		switch access.UserID {
		case owner.ID:
			if !access.CanRead || !access.CanCreateDesign || !access.CanManage {
				t.Fatalf("owner access = %+v, want manager", access)
			}
		case viewer.ID:
			if !access.CanRead || access.CanCreateDesign || access.CanManage {
				t.Fatalf("viewer access = %+v, want read only", access)
			}
		}
	}
	groupAccess, err := repo.ListWorkspaceGroupAccess(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("list group access: %v", err)
	}
	if len(groupAccess) != 1 || !groupAccess[0].CanRead || !groupAccess[0].CanCreateDesign || groupAccess[0].CanManage {
		t.Fatalf("group access = %+v, want contributor", groupAccess)
	}
}

func TestWorkspaceServiceCreateWithAccessRollsBackInvalidShare(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemoryRepository()
	owner, err := repo.CreateFirstAdmin(ctx, "Owner", "owner@example.com", "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	service := WorkspaceService{provider: StaticRepositoryProvider{Repo: repo}}

	_, err = service.CreateWithAccess(ctx, "Should rollback", owner.ID, []WorkspaceShare{
		{PrincipalType: "group", PrincipalID: "missing-group", AccessLevel: "viewer"},
	})
	if err == nil {
		t.Fatal("CreateWithAccess returned nil error for missing group")
	}
	workspaces, listErr := repo.ListWorkspaces(ctx)
	if listErr != nil {
		t.Fatalf("list workspaces: %v", listErr)
	}
	for _, workspace := range workspaces {
		if workspace.Name == "Should rollback" {
			t.Fatalf("rolled back workspace still exists: %+v", workspace)
		}
	}
}
