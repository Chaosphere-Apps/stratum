package policy

import (
	"testing"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

type testProvider struct {
	repo store.Repository
}

func (p testProvider) Repository() store.Repository {
	return p.repo
}

func TestRoleAllows(t *testing.T) {
	for _, test := range []struct {
		actual  string
		minimum string
		want    bool
	}{
		{"admin", "member", true},
		{"architect", "reviewer", true},
		{"reviewer", "architect", false},
		{"member", "member", true},
		{"unknown", "member", false},
		{" ADMIN ", "architect", true},
	} {
		if got := RoleAllows(test.actual, test.minimum); got != test.want {
			t.Fatalf("RoleAllows(%q, %q) = %v, want %v", test.actual, test.minimum, got, test.want)
		}
	}
}

func TestWorkspacePolicyWithRolesUserGrantsAndGroupGrants(t *testing.T) {
	ctx := t.Context()
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(ctx, "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	architect, err := repo.CreateUser(ctx, "Architect", "architect@example.com", "architect", "password123")
	if err != nil {
		t.Fatalf("CreateUser architect returned error: %v", err)
	}
	member, err := repo.CreateUser(ctx, "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser member returned error: %v", err)
	}
	groupMember, err := repo.CreateUser(ctx, "Group Member", "group@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser group member returned error: %v", err)
	}
	workspace, err := repo.CreateWorkspace(ctx, "Platform", architect.ID)
	if err != nil {
		t.Fatalf("CreateWorkspace returned error: %v", err)
	}
	authz := NewAuthorizer(testProvider{repo: repo})

	if !authz.CanAccessWorkspace(ctx, admin, workspace.ID, WorkspaceManage) {
		t.Fatal("admin should manage workspace")
	}
	if !authz.CanAccessWorkspace(ctx, architect, workspace.ID, WorkspaceCreateDesign) {
		t.Fatal("workspace owner should create designs")
	}
	if authz.CanAccessWorkspace(ctx, member, workspace.ID, WorkspaceRead) {
		t.Fatal("member should not read workspace before grant")
	}

	if _, err := repo.GrantWorkspaceAccess(ctx, domain.WorkspaceAccess{
		WorkspaceID: workspace.ID,
		UserID:      member.ID,
		CanRead:     true,
	}); err != nil {
		t.Fatalf("GrantWorkspaceAccess returned error: %v", err)
	}
	if !authz.CanAccessWorkspace(ctx, member, workspace.ID, WorkspaceRead) {
		t.Fatal("user read grant should allow workspace read")
	}
	if authz.CanAccessWorkspace(ctx, member, workspace.ID, WorkspaceCreateDesign) {
		t.Fatal("read grant should not allow design creation")
	}

	group, err := repo.CreateAccessGroup(ctx, domain.AccessGroup{Name: "Reviewers", OktaGroupName: "stratum-reviewers"})
	if err != nil {
		t.Fatalf("CreateAccessGroup returned error: %v", err)
	}
	if _, err := repo.ReplaceAccessGroupMembers(ctx, group.ID, []string{groupMember.ID}); err != nil {
		t.Fatalf("ReplaceAccessGroupMembers returned error: %v", err)
	}
	if _, err := repo.GrantWorkspaceGroupAccess(ctx, domain.WorkspaceGroupAccess{
		WorkspaceID:     workspace.ID,
		GroupID:         group.ID,
		CanCreateDesign: true,
	}); err != nil {
		t.Fatalf("GrantWorkspaceGroupAccess returned error: %v", err)
	}
	if !authz.CanAccessWorkspace(ctx, groupMember, workspace.ID, WorkspaceCreateDesign) {
		t.Fatal("group create grant should allow workspace design creation")
	}
}

func TestDesignPolicyWithPublicWorkspaceUserAndGroupGrants(t *testing.T) {
	ctx := t.Context()
	repo := store.NewMemoryRepository()
	owner, err := repo.CreateFirstAdmin(ctx, "Owner", "owner@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(ctx, "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser member returned error: %v", err)
	}
	reviewer, err := repo.CreateUser(ctx, "Reviewer", "reviewer@example.com", "reviewer", "password123")
	if err != nil {
		t.Fatalf("CreateUser reviewer returned error: %v", err)
	}
	groupMember, err := repo.CreateUser(ctx, "Group Member", "group@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser group member returned error: %v", err)
	}
	workspace, err := repo.CreateWorkspace(ctx, "Platform", owner.ID)
	if err != nil {
		t.Fatalf("CreateWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(ctx, workspace.ID, "Payments", []byte(`{"id":"payments","title":"Payments"}`), owner.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	authz := NewAuthorizer(testProvider{repo: repo})

	if !authz.CanAccessDesign(ctx, owner, design, DesignManage) {
		t.Fatal("creator should manage design")
	}
	if authz.CanAccessDesign(ctx, member, design, DesignRead) {
		t.Fatal("private design should deny member before grants")
	}

	publicDesign := design
	publicDesign.Access = "public"
	if !authz.CanAccessDesign(ctx, member, publicDesign, DesignRead) {
		t.Fatal("public design should allow read")
	}
	if authz.CanAccessDesign(ctx, member, publicDesign, DesignEdit) {
		t.Fatal("public design should not allow edit")
	}

	if _, err := repo.GrantDesignAccess(ctx, domain.DesignAccess{
		WorkspaceID: workspace.ID,
		DesignID:    design.ID,
		UserID:      reviewer.ID,
		CanReview:   true,
	}); err != nil {
		t.Fatalf("GrantDesignAccess returned error: %v", err)
	}
	if !authz.CanAccessDesign(ctx, reviewer, design, DesignReview) {
		t.Fatal("review grant should allow review")
	}
	if !authz.CanAccessDesign(ctx, reviewer, design, DesignComment) {
		t.Fatal("review grant should allow comments")
	}
	if authz.CanAccessDesign(ctx, reviewer, design, DesignEdit) {
		t.Fatal("review grant should not allow edit")
	}

	group, err := repo.CreateAccessGroup(ctx, domain.AccessGroup{Name: "Editors"})
	if err != nil {
		t.Fatalf("CreateAccessGroup returned error: %v", err)
	}
	if _, err := repo.ReplaceAccessGroupMembers(ctx, group.ID, []string{groupMember.ID}); err != nil {
		t.Fatalf("ReplaceAccessGroupMembers returned error: %v", err)
	}
	if _, err := repo.GrantDesignGroupAccess(ctx, domain.DesignGroupAccess{
		WorkspaceID: workspace.ID,
		DesignID:    design.ID,
		GroupID:     group.ID,
		CanEdit:     true,
	}); err != nil {
		t.Fatalf("GrantDesignGroupAccess returned error: %v", err)
	}
	if !authz.CanAccessDesign(ctx, groupMember, design, DesignEdit) {
		t.Fatal("group edit grant should allow edit")
	}
}
