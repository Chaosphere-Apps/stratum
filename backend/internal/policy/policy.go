package policy

import (
	"context"
	"strings"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

const (
	WorkspaceRead         = "read"
	WorkspaceCreateDesign = "create_design"
	WorkspaceManage       = "manage"

	DesignRead    = "read"
	DesignEdit    = "edit"
	DesignComment = "comment"
	DesignReview  = "review"
	DesignManage  = "manage"
)

type RepositoryProvider interface {
	Repository() store.Repository
}

type Authorizer struct {
	provider RepositoryProvider
}

func NewAuthorizer(provider RepositoryProvider) Authorizer {
	return Authorizer{provider: provider}
}

func (a Authorizer) CanAccessWorkspace(ctx context.Context, user domain.User, workspaceID string, permission string) bool {
	return CanAccessWorkspace(ctx, a.provider.Repository(), user, workspaceID, permission)
}

func (a Authorizer) CanAccessDesign(ctx context.Context, user domain.User, design domain.Design, permission string) bool {
	return CanAccessDesign(ctx, a.provider.Repository(), user, design, permission)
}

func (a Authorizer) CanEditDesignDocument(ctx context.Context, user domain.User, workspaceID string, designID string) bool {
	if RoleAllows(user.Role, "admin") {
		return true
	}
	repo := a.provider.Repository()
	design, err := repo.GetDesign(ctx, workspaceID, designID)
	if err != nil {
		return false
	}
	return CanAccessDesign(ctx, repo, user, design, DesignEdit)
}

func CanAccessWorkspace(ctx context.Context, repo store.Repository, user domain.User, workspaceID string, permission string) bool {
	if RoleAllows(user.Role, "admin") {
		return true
	}
	workspace, err := repo.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return false
	}
	if workspace.OwnerID != "" && workspace.OwnerID == user.ID {
		return true
	}
	entries, err := repo.ListWorkspaceAccess(ctx, workspaceID)
	if err != nil {
		return false
	}
	groupEntries, err := repo.ListWorkspaceGroupAccess(ctx, workspaceID)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.UserID == user.ID && WorkspaceAccessAllows(entry, permission) {
			return true
		}
	}
	groupIDs, err := repo.ListUserAccessGroupIDs(ctx, user.ID)
	if err != nil {
		return false
	}
	userGroups := stringSet(groupIDs)
	for _, entry := range groupEntries {
		if _, ok := userGroups[entry.GroupID]; ok && WorkspaceGroupAccessAllows(entry, permission) {
			return true
		}
	}
	return false
}

func CanAccessDesign(ctx context.Context, repo store.Repository, user domain.User, design domain.Design, permission string) bool {
	if RoleAllows(user.Role, "admin") || design.CreatedBy == user.ID {
		return true
	}
	if permission == DesignRead && strings.EqualFold(design.Access, "public") {
		return true
	}
	if CanAccessWorkspace(ctx, repo, user, design.WorkspaceID, WorkspaceManage) {
		return true
	}
	if permission == DesignRead && strings.EqualFold(design.Access, "workspace") && CanAccessWorkspace(ctx, repo, user, design.WorkspaceID, WorkspaceRead) {
		return true
	}
	entries, err := repo.ListDesignAccess(ctx, design.WorkspaceID, design.ID)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.UserID == user.ID && DesignAccessAllows(entry, permission) {
			return true
		}
	}
	groupEntries, err := repo.ListDesignGroupAccess(ctx, design.WorkspaceID, design.ID)
	if err != nil {
		return false
	}
	groupIDs, err := repo.ListUserAccessGroupIDs(ctx, user.ID)
	if err != nil {
		return false
	}
	userGroups := stringSet(groupIDs)
	for _, entry := range groupEntries {
		if _, ok := userGroups[entry.GroupID]; ok && DesignGroupAccessAllows(entry, permission) {
			return true
		}
	}
	return false
}

func RoleAllows(actualRole string, minimumRole string) bool {
	return roleRank(actualRole) >= roleRank(minimumRole)
}

func WorkspaceAccessAllows(access domain.WorkspaceAccess, permission string) bool {
	if access.CanManage {
		return true
	}
	switch permission {
	case WorkspaceRead:
		return access.CanRead || access.CanCreateDesign
	case WorkspaceCreateDesign:
		return access.CanCreateDesign
	case WorkspaceManage:
		return access.CanManage
	default:
		return false
	}
}

func WorkspaceGroupAccessAllows(access domain.WorkspaceGroupAccess, permission string) bool {
	if access.CanManage {
		return true
	}
	switch permission {
	case WorkspaceRead:
		return access.CanRead || access.CanCreateDesign
	case WorkspaceCreateDesign:
		return access.CanCreateDesign
	case WorkspaceManage:
		return access.CanManage
	default:
		return false
	}
}

func DesignAccessAllows(access domain.DesignAccess, permission string) bool {
	if access.CanManage {
		return true
	}
	switch permission {
	case DesignRead:
		return access.CanRead || access.CanEdit || access.CanComment || access.CanReview
	case DesignEdit:
		return access.CanEdit
	case DesignComment:
		return access.CanComment || access.CanEdit || access.CanReview
	case DesignReview:
		return access.CanReview || access.CanEdit
	case DesignManage:
		return access.CanManage
	default:
		return false
	}
}

func DesignGroupAccessAllows(access domain.DesignGroupAccess, permission string) bool {
	if access.CanManage {
		return true
	}
	switch permission {
	case DesignRead:
		return access.CanRead || access.CanEdit || access.CanComment || access.CanReview
	case DesignEdit:
		return access.CanEdit
	case DesignComment:
		return access.CanComment || access.CanEdit || access.CanReview
	case DesignReview:
		return access.CanReview || access.CanEdit
	case DesignManage:
		return access.CanManage
	default:
		return false
	}
}

func roleRank(role string) int {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "admin":
		return 4
	case "architect":
		return 3
	case "reviewer":
		return 2
	case "member":
		return 1
	default:
		return 0
	}
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}
