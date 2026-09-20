package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
)

func (r *PostgresRepository) GetOrCreateGuestWorkspace(ctx context.Context) (domain.Workspace, error) {
	now := r.clock().UTC()
	_, err := r.pool.Exec(ctx, `
INSERT INTO workspaces (id, name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
ON CONFLICT (id) DO NOTHING
`, domain.GuestWorkspaceID, "Guest Workspace", now)
	if err != nil {
		return domain.Workspace{}, err
	}
	return r.GetWorkspace(ctx, domain.GuestWorkspaceID)
}

func (r *PostgresRepository) ListWorkspaces(ctx context.Context) ([]domain.Workspace, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.Workspace, PageInfo, error) {
		return r.ListWorkspacesPage(ctx, options)
	})
}

func (r *PostgresRepository) ListWorkspacesPage(ctx context.Context, options PageOptions) ([]domain.Workspace, PageInfo, error) {
	if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	likeQuery := "%" + strings.ToLower(options.Query) + "%"
	rows, err := r.pool.Query(ctx, `
SELECT id, name, COALESCE(owner_id, ''), created_at, updated_at
FROM workspaces
WHERE $1 = '' OR lower(name) LIKE $2
ORDER BY updated_at DESC
LIMIT $3 OFFSET $4
`, options.Query, likeQuery, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()

	workspaces := []domain.Workspace{}
	for rows.Next() {
		workspace, err := scanWorkspace(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		workspaces = append(workspaces, workspace)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(workspaces, options, offset)
	return items, page, nil
}

func (r *PostgresRepository) ListAccessibleWorkspacesPage(ctx context.Context, scope AccessScope, options PageOptions) ([]domain.Workspace, PageInfo, error) {
	if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	likeQuery := "%" + strings.ToLower(options.Query) + "%"
	rows, err := r.pool.Query(ctx, `
SELECT workspace.id, workspace.name, COALESCE(workspace.owner_id, ''), workspace.created_at, workspace.updated_at
FROM workspaces AS workspace
WHERE ($1 = '' OR lower(workspace.name) LIKE $2)
  AND (
    $3 OR workspace.owner_id = $4
    OR EXISTS (
      SELECT 1 FROM workspace_access AS access
      WHERE access.workspace_id = workspace.id AND access.user_id = $4
        AND (access.can_read OR access.can_create_design OR access.can_manage)
    )
    OR EXISTS (
      SELECT 1 FROM workspace_group_access AS access
      WHERE access.workspace_id = workspace.id AND access.group_id = ANY($5::text[])
        AND (access.can_read OR access.can_create_design OR access.can_manage)
    )
  )
ORDER BY workspace.updated_at DESC
LIMIT $6 OFFSET $7
`, options.Query, likeQuery, scope.IsAdmin, scope.UserID, scope.GroupIDs, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()
	workspaces := []domain.Workspace{}
	for rows.Next() {
		workspace, err := scanWorkspace(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		workspaces = append(workspaces, workspace)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(workspaces, options, offset)
	return items, page, nil
}

func (r *PostgresRepository) GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return domain.Workspace{}, errors.New("workspace id is required")
	}
	row := r.pool.QueryRow(ctx, `
SELECT id, name, COALESCE(owner_id, ''), created_at, updated_at
FROM workspaces
WHERE id = $1
`, workspaceID)
	workspace, err := scanWorkspace(row)
	if errors.Is(err, pgx.ErrNoRows) && workspaceID == domain.GuestWorkspaceID {
		return r.GetOrCreateGuestWorkspace(ctx)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, errors.New("workspace not found")
	}
	return workspace, err
}

func (r *PostgresRepository) CreateWorkspace(ctx context.Context, name string, ownerID string) (domain.Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Workspace{}, errors.New("workspace name is required")
	}

	now := r.clock().UTC()
	workspace := domain.Workspace{
		ID:        fmt.Sprintf("workspace_%d", now.UnixNano()),
		Name:      name,
		OwnerID:   strings.TrimSpace(ownerID),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO workspaces (id, name, owner_id, created_at, updated_at)
VALUES ($1, $2, NULLIF($3, ''), $4, $5)
`, workspace.ID, workspace.Name, workspace.OwnerID, workspace.CreatedAt, workspace.UpdatedAt)
	if err != nil {
		return domain.Workspace{}, err
	}
	return workspace, nil
}

func (r *PostgresRepository) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return errors.New("workspace id is required")
	}
	if workspaceID == domain.GuestWorkspaceID {
		return errors.New("default workspace cannot be deleted")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	var lockedWorkspaceID string
	if err := tx.QueryRow(ctx, `
SELECT id
FROM workspaces
WHERE id = $1
FOR UPDATE
`, workspaceID).Scan(&lockedWorkspaceID); errors.Is(err, pgx.ErrNoRows) {
		return errors.New("workspace not found")
	} else if err != nil {
		return err
	}

	var designCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM designs WHERE workspace_id = $1`, workspaceID).Scan(&designCount); err != nil {
		return err
	}
	if designCount > 0 {
		return errors.New("workspace is not empty")
	}

	tag, err := tx.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("workspace not found")
	}
	return tx.Commit(ctx)
}
