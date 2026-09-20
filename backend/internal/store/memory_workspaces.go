package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"sort"
	"strings"
)

func (r *MemoryRepository) GetOrCreateGuestWorkspace(ctx context.Context) (domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if workspace, ok := r.workspaces[domain.GuestWorkspaceID]; ok {
		return workspace, nil
	}

	{
		now := r.clock().UTC()
		workspace := domain.NewGuestWorkspace(now)
		r.workspaces[workspace.ID] = workspace
	}

	return r.workspaces[domain.GuestWorkspaceID], nil
}

func (r *MemoryRepository) ListWorkspaces(ctx context.Context) ([]domain.Workspace, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.Workspace, PageInfo, error) {
		return r.ListWorkspacesPage(ctx, options)
	})
}

func (r *MemoryRepository) ListWorkspacesPage(ctx context.Context, options PageOptions) ([]domain.Workspace, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	query := strings.ToLower(options.Query)

	r.mu.RLock()
	defer r.mu.RUnlock()

	workspaces := make([]domain.Workspace, 0, len(r.workspaces))
	for _, workspace := range r.workspaces {
		if query != "" && !strings.Contains(strings.ToLower(workspace.Name), query) {
			continue
		}
		workspaces = append(workspaces, workspace)
	}
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt)
	})
	page, info := PageFromSlice(workspaces, options)
	return page, info, nil
}

func (r *MemoryRepository) ListAccessibleWorkspacesPage(ctx context.Context, scope AccessScope, options PageOptions) ([]domain.Workspace, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	query := strings.ToLower(options.Query)
	groups := stringSet(scope.GroupIDs)
	r.mu.RLock()
	defer r.mu.RUnlock()
	workspaces := make([]domain.Workspace, 0, len(r.workspaces))
	for _, workspace := range r.workspaces {
		if query != "" && !strings.Contains(strings.ToLower(workspace.Name), query) {
			continue
		}
		allowed := scope.IsAdmin || workspace.OwnerID == scope.UserID
		if access, ok := r.workspaceACL[accessKey(workspace.ID, scope.UserID)]; ok {
			allowed = allowed || access.CanRead || access.CanCreateDesign || access.CanManage
		}
		for _, access := range r.workspaceGACL {
			_, member := groups[access.GroupID]
			if access.WorkspaceID == workspace.ID && member && (access.CanRead || access.CanCreateDesign || access.CanManage) {
				allowed = true
				break
			}
		}
		if allowed {
			workspaces = append(workspaces, workspace)
		}
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt) })
	page, info := PageFromSlice(workspaces, options)
	return page, info, nil
}

func (r *MemoryRepository) GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}
	if workspaceID == domain.GuestWorkspaceID {
		return r.GetOrCreateGuestWorkspace(ctx)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	workspace, ok := r.workspaces[workspaceID]
	if !ok {
		return domain.Workspace{}, errors.New("workspace not found")
	}
	return workspace, nil
}

func (r *MemoryRepository) CreateWorkspace(ctx context.Context, name string, ownerID string) (domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Workspace{}, errors.New("workspace name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock().UTC()
	workspace := domain.Workspace{
		ID:        fmt.Sprintf("workspace_%d", now.UnixNano()),
		Name:      name,
		OwnerID:   strings.TrimSpace(ownerID),
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.workspaces[workspace.ID] = workspace
	return workspace, nil
}

func (r *MemoryRepository) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return errors.New("workspace id is required")
	}
	if workspaceID == domain.GuestWorkspaceID {
		return errors.New("default workspace cannot be deleted")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.workspaces[workspaceID]; !ok {
		return errors.New("workspace not found")
	}
	for _, design := range r.designs {
		if design.WorkspaceID == workspaceID {
			return errors.New("workspace is not empty")
		}
	}
	delete(r.workspaces, workspaceID)
	return nil
}
