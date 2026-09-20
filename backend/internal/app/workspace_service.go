package app

import (
	"context"
	"errors"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
	"strings"
)

type WorkspaceShare struct {
	PrincipalType string
	PrincipalID   string
	AccessLevel   string
}

type WorkspaceService struct {
	provider RepositoryProvider
}

func (s WorkspaceService) repo() store.Repository { return s.provider.Repository() }

func (s WorkspaceService) List(ctx context.Context) ([]domain.Workspace, error) {
	return s.repo().ListWorkspaces(ctx)
}

func (s WorkspaceService) ListPage(ctx context.Context, options store.PageOptions) ([]domain.Workspace, store.PageInfo, error) {
	return s.repo().ListWorkspacesPage(ctx, options)
}

func (s WorkspaceService) ListAccessiblePage(ctx context.Context, user domain.User, options store.PageOptions) ([]domain.Workspace, store.PageInfo, error) {
	groupIDs, err := s.repo().ListUserAccessGroupIDs(ctx, user.ID)
	if err != nil {
		return nil, store.PageInfo{}, err
	}
	return s.repo().ListAccessibleWorkspacesPage(ctx, store.AccessScope{UserID: user.ID, IsAdmin: user.Role == "admin", GroupIDs: groupIDs}, options)
}

func (s WorkspaceService) Get(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	return s.repo().GetWorkspace(ctx, workspaceID)
}

func (s WorkspaceService) GetOrCreateGuest(ctx context.Context) (domain.Workspace, error) {
	return s.repo().GetOrCreateGuestWorkspace(ctx)
}

func (s WorkspaceService) Create(ctx context.Context, name string, ownerID ...string) (domain.Workspace, error) {
	owner := ""
	if len(ownerID) > 0 {
		owner = ownerID[0]
	}
	return s.repo().CreateWorkspace(ctx, name, owner)
}

func (s WorkspaceService) CreateWithAccess(ctx context.Context, name string, ownerID string, shares []WorkspaceShare) (domain.Workspace, error) {
	workspace, err := s.repo().CreateWorkspace(ctx, name, ownerID)
	if err != nil {
		return domain.Workspace{}, err
	}
	rollback := func(cause error) (domain.Workspace, error) {
		if rollbackErr := s.repo().DeleteWorkspace(ctx, workspace.ID); rollbackErr != nil {
			return domain.Workspace{}, errors.Join(cause, rollbackErr)
		}
		return domain.Workspace{}, cause
	}
	if strings.TrimSpace(ownerID) != "" {
		if _, err := s.repo().GrantWorkspaceAccess(ctx, domain.WorkspaceAccess{WorkspaceID: workspace.ID, UserID: ownerID, CanRead: true, CanCreateDesign: true, CanManage: true}); err != nil {
			return rollback(err)
		}
	}
	for _, share := range shares {
		canCreate, canManage, err := workspaceAccessLevel(share.AccessLevel)
		if err != nil {
			return rollback(err)
		}
		switch strings.ToLower(strings.TrimSpace(share.PrincipalType)) {
		case "user":
			if _, err := s.repo().GetUser(ctx, share.PrincipalID); err != nil {
				return rollback(err)
			}
			_, err = s.repo().GrantWorkspaceAccess(ctx, domain.WorkspaceAccess{WorkspaceID: workspace.ID, UserID: share.PrincipalID, CanRead: true, CanCreateDesign: canCreate, CanManage: canManage})
		case "group":
			groups, listErr := s.repo().ListAccessGroups(ctx)
			if listErr != nil {
				return rollback(listErr)
			}
			found := false
			for _, group := range groups {
				found = found || group.ID == share.PrincipalID
			}
			if !found {
				return rollback(errors.New("access group not found"))
			}
			_, err = s.repo().GrantWorkspaceGroupAccess(ctx, domain.WorkspaceGroupAccess{WorkspaceID: workspace.ID, GroupID: share.PrincipalID, CanRead: true, CanCreateDesign: canCreate, CanManage: canManage})
		default:
			err = errors.New("workspace share principal type is invalid")
		}
		if err != nil {
			return rollback(err)
		}
	}
	return workspace, nil
}

func workspaceAccessLevel(level string) (canCreate bool, canManage bool, err error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "viewer":
		return false, false, nil
	case "contributor":
		return true, false, nil
	case "manager":
		return true, true, nil
	default:
		return false, false, errors.New("workspace access level is invalid")
	}
}

func (s WorkspaceService) Delete(ctx context.Context, workspaceID string) error {
	return s.repo().DeleteWorkspace(ctx, workspaceID)
}
