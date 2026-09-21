package app

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

type ACLService struct {
	provider RepositoryProvider
}

func (s ACLService) repo() store.Repository { return s.provider.Repository() }

func (s ACLService) ListWorkspaceAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceAccess, error) {
	return s.repo().ListWorkspaceAccess(ctx, workspaceID)
}

func (s ACLService) GrantWorkspaceAccess(ctx context.Context, access domain.WorkspaceAccess) (domain.WorkspaceAccess, error) {
	return s.repo().GrantWorkspaceAccess(ctx, access)
}

func (s ACLService) RevokeWorkspaceAccess(ctx context.Context, workspaceID string, userID string) error {
	return s.repo().RevokeWorkspaceAccess(ctx, workspaceID, userID)
}

func (s ACLService) ListWorkspaceGroupAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceGroupAccess, error) {
	return s.repo().ListWorkspaceGroupAccess(ctx, workspaceID)
}

func (s ACLService) GrantWorkspaceGroupAccess(ctx context.Context, access domain.WorkspaceGroupAccess) (domain.WorkspaceGroupAccess, error) {
	return s.repo().GrantWorkspaceGroupAccess(ctx, access)
}

func (s ACLService) RevokeWorkspaceGroupAccess(ctx context.Context, workspaceID string, groupID string) error {
	return s.repo().RevokeWorkspaceGroupAccess(ctx, workspaceID, groupID)
}

func (s ACLService) ListDesignAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignAccess, error) {
	return s.repo().ListDesignAccess(ctx, workspaceID, designID)
}

func (s ACLService) GrantDesignAccess(ctx context.Context, access domain.DesignAccess) (domain.DesignAccess, error) {
	return s.repo().GrantDesignAccess(ctx, access)
}

func (s ACLService) RevokeDesignAccess(ctx context.Context, workspaceID string, designID string, userID string) error {
	return s.repo().RevokeDesignAccess(ctx, workspaceID, designID, userID)
}

func (s ACLService) ListDesignGroupAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignGroupAccess, error) {
	return s.repo().ListDesignGroupAccess(ctx, workspaceID, designID)
}

func (s ACLService) GrantDesignGroupAccess(ctx context.Context, access domain.DesignGroupAccess) (domain.DesignGroupAccess, error) {
	return s.repo().GrantDesignGroupAccess(ctx, access)
}

func (s ACLService) RevokeDesignGroupAccess(ctx context.Context, workspaceID string, designID string, groupID string) error {
	return s.repo().RevokeDesignGroupAccess(ctx, workspaceID, designID, groupID)
}

func (s ACLService) ListGroups(ctx context.Context) ([]domain.AccessGroup, error) {
	return s.repo().ListAccessGroups(ctx)
}

func (s ACLService) CreateGroup(ctx context.Context, group domain.AccessGroup) (domain.AccessGroup, error) {
	return s.repo().CreateAccessGroup(ctx, group)
}

func (s ACLService) UpdateGroup(ctx context.Context, groupID string, group domain.AccessGroup) (domain.AccessGroup, error) {
	return s.repo().UpdateAccessGroup(ctx, groupID, group)
}

func (s ACLService) DeleteGroup(ctx context.Context, groupID string) error {
	return s.repo().DeleteAccessGroup(ctx, groupID)
}

func (s ACLService) ListGroupMembers(ctx context.Context, groupID string) ([]domain.AccessGroupMember, error) {
	return s.repo().ListAccessGroupMembers(ctx, groupID)
}

func (s ACLService) ReplaceGroupMembers(ctx context.Context, groupID string, userIDs []string) ([]domain.AccessGroupMember, error) {
	return s.repo().ReplaceAccessGroupMembers(ctx, groupID, userIDs)
}

func (s ACLService) ListUserGroupIDs(ctx context.Context, userID string) ([]string, error) {
	return s.repo().ListUserAccessGroupIDs(ctx, userID)
}
