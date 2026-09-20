package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
)

func (r *PostgresRepository) ListAccessGroups(ctx context.Context) ([]domain.AccessGroup, error) {
	rows, err := r.pool.Query(ctx, `
SELECT g.id, g.name, g.description, g.okta_group_name, COUNT(m.user_id), g.created_at, g.updated_at
FROM access_groups g
LEFT JOIN access_group_members m ON m.group_id = g.id
GROUP BY g.id
ORDER BY lower(g.name)
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []domain.AccessGroup{}
	for rows.Next() {
		group, err := scanAccessGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (r *PostgresRepository) CreateAccessGroup(ctx context.Context, group domain.AccessGroup) (domain.AccessGroup, error) {
	name := strings.TrimSpace(group.Name)
	if name == "" {
		return domain.AccessGroup{}, errors.New("group name is required")
	}
	now := r.clock().UTC()
	groupID := fmt.Sprintf("grp_%d", now.UnixNano())
	row := r.pool.QueryRow(ctx, `
INSERT INTO access_groups (id, name, description, okta_group_name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING id, name, description, okta_group_name, 0, created_at, updated_at
`, groupID, name, strings.TrimSpace(group.Description), strings.TrimSpace(group.OktaGroupName), now)
	return scanAccessGroup(row)
}

func (r *PostgresRepository) UpdateAccessGroup(ctx context.Context, groupID string, group domain.AccessGroup) (domain.AccessGroup, error) {
	name := strings.TrimSpace(group.Name)
	if name == "" {
		return domain.AccessGroup{}, errors.New("group name is required")
	}
	now := r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
WITH updated AS (
	UPDATE access_groups
	SET name = $1, description = $2, okta_group_name = $3, updated_at = $4
	WHERE id = $5
	RETURNING id, name, description, okta_group_name, created_at, updated_at
)
SELECT updated.id, updated.name, updated.description, updated.okta_group_name, COUNT(m.user_id), updated.created_at, updated.updated_at
FROM updated
LEFT JOIN access_group_members m ON m.group_id = updated.id
GROUP BY updated.id, updated.name, updated.description, updated.okta_group_name, updated.created_at, updated.updated_at
`, name, strings.TrimSpace(group.Description), strings.TrimSpace(group.OktaGroupName), now, strings.TrimSpace(groupID))
	updated, err := scanAccessGroup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AccessGroup{}, errors.New("group not found")
	}
	return updated, err
}

func (r *PostgresRepository) DeleteAccessGroup(ctx context.Context, groupID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM access_groups WHERE id = $1`, strings.TrimSpace(groupID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("group not found")
	}
	return nil
}

func (r *PostgresRepository) ListAccessGroupMembers(ctx context.Context, groupID string) ([]domain.AccessGroupMember, error) {
	groupID = strings.TrimSpace(groupID)
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM access_groups WHERE id = $1)`, groupID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("group not found")
	}
	rows, err := r.pool.Query(ctx, `
SELECT group_id, user_id, added_at
FROM access_group_members
WHERE group_id = $1
ORDER BY user_id
`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []domain.AccessGroupMember{}
	for rows.Next() {
		member, err := scanAccessGroupMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *PostgresRepository) ReplaceAccessGroupMembers(ctx context.Context, groupID string, userIDs []string) ([]domain.AccessGroupMember, error) {
	groupID = strings.TrimSpace(groupID)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM access_groups WHERE id = $1)`, groupID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("group not found")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM access_group_members WHERE group_id = $1`, groupID); err != nil {
		return nil, err
	}
	now := r.clock().UTC()
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO access_group_members (group_id, user_id, added_at)
VALUES ($1, $2, $3)
ON CONFLICT (group_id, user_id) DO NOTHING
`, groupID, userID, now); err != nil {
			return nil, err
		}
	}
	rows, err := tx.Query(ctx, `
SELECT group_id, user_id, added_at
FROM access_group_members
WHERE group_id = $1
ORDER BY user_id
`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []domain.AccessGroupMember{}
	for rows.Next() {
		member, err := scanAccessGroupMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE access_groups SET updated_at = $1 WHERE id = $2`, now, groupID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return members, nil
}

func (r *PostgresRepository) ListUserAccessGroupIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT group_id FROM access_group_members WHERE user_id = $1 ORDER BY group_id`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groupIDs := []string{}
	for rows.Next() {
		var groupID string
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}
	return groupIDs, rows.Err()
}

func (r *PostgresRepository) ListWorkspaceAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceAccess, error) {
	rows, err := r.pool.Query(ctx, `
SELECT workspace_id, user_id, can_read, can_create_design, can_manage, created_at, updated_at
FROM workspace_access
WHERE workspace_id = $1
ORDER BY user_id
`, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	access := []domain.WorkspaceAccess{}
	for rows.Next() {
		entry, err := scanWorkspaceAccess(rows)
		if err != nil {
			return nil, err
		}
		access = append(access, entry)
	}
	return access, rows.Err()
}

func (r *PostgresRepository) GrantWorkspaceAccess(ctx context.Context, access domain.WorkspaceAccess) (domain.WorkspaceAccess, error) {
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.UserID = strings.TrimSpace(access.UserID)
	if access.WorkspaceID == "" || access.UserID == "" {
		return domain.WorkspaceAccess{}, errors.New("workspace id and user id are required")
	}
	now := r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
INSERT INTO workspace_access (workspace_id, user_id, can_read, can_create_design, can_manage, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET can_read = EXCLUDED.can_read, can_create_design = EXCLUDED.can_create_design, can_manage = EXCLUDED.can_manage, updated_at = EXCLUDED.updated_at
RETURNING workspace_id, user_id, can_read, can_create_design, can_manage, created_at, updated_at
`, access.WorkspaceID, access.UserID, access.CanRead, access.CanCreateDesign, access.CanManage, now)
	return scanWorkspaceAccess(row)
}

func (r *PostgresRepository) RevokeWorkspaceAccess(ctx context.Context, workspaceID string, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM workspace_access WHERE workspace_id = $1 AND user_id = $2`, strings.TrimSpace(workspaceID), strings.TrimSpace(userID))
	return err
}

func (r *PostgresRepository) ListWorkspaceGroupAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceGroupAccess, error) {
	rows, err := r.pool.Query(ctx, `
SELECT workspace_id, group_id, can_read, can_create_design, can_manage, created_at, updated_at
FROM workspace_group_access
WHERE workspace_id = $1
ORDER BY group_id
`, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	access := []domain.WorkspaceGroupAccess{}
	for rows.Next() {
		entry, err := scanWorkspaceGroupAccess(rows)
		if err != nil {
			return nil, err
		}
		access = append(access, entry)
	}
	return access, rows.Err()
}

func (r *PostgresRepository) GrantWorkspaceGroupAccess(ctx context.Context, access domain.WorkspaceGroupAccess) (domain.WorkspaceGroupAccess, error) {
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.GroupID = strings.TrimSpace(access.GroupID)
	if access.WorkspaceID == "" || access.GroupID == "" {
		return domain.WorkspaceGroupAccess{}, errors.New("workspace id and group id are required")
	}
	now := r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
INSERT INTO workspace_group_access (workspace_id, group_id, can_read, can_create_design, can_manage, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (workspace_id, group_id)
DO UPDATE SET can_read = EXCLUDED.can_read, can_create_design = EXCLUDED.can_create_design, can_manage = EXCLUDED.can_manage, updated_at = EXCLUDED.updated_at
RETURNING workspace_id, group_id, can_read, can_create_design, can_manage, created_at, updated_at
`, access.WorkspaceID, access.GroupID, access.CanRead, access.CanCreateDesign, access.CanManage, now)
	return scanWorkspaceGroupAccess(row)
}

func (r *PostgresRepository) RevokeWorkspaceGroupAccess(ctx context.Context, workspaceID string, groupID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM workspace_group_access WHERE workspace_id = $1 AND group_id = $2`, strings.TrimSpace(workspaceID), strings.TrimSpace(groupID))
	return err
}

func (r *PostgresRepository) ListDesignAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignAccess, error) {
	rows, err := r.pool.Query(ctx, `
SELECT workspace_id, design_id, user_id, can_read, can_edit, can_comment, can_review, can_manage, created_at, updated_at
FROM design_access
WHERE workspace_id = $1 AND design_id = $2
ORDER BY user_id
`, strings.TrimSpace(workspaceID), strings.TrimSpace(designID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	access := []domain.DesignAccess{}
	for rows.Next() {
		entry, err := scanDesignAccess(rows)
		if err != nil {
			return nil, err
		}
		access = append(access, entry)
	}
	return access, rows.Err()
}

func (r *PostgresRepository) GrantDesignAccess(ctx context.Context, access domain.DesignAccess) (domain.DesignAccess, error) {
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.DesignID = strings.TrimSpace(access.DesignID)
	access.UserID = strings.TrimSpace(access.UserID)
	if access.WorkspaceID == "" || access.DesignID == "" || access.UserID == "" {
		return domain.DesignAccess{}, errors.New("workspace id, design id, and user id are required")
	}
	now := r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
INSERT INTO design_access (workspace_id, design_id, user_id, can_read, can_edit, can_comment, can_review, can_manage, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
ON CONFLICT (workspace_id, design_id, user_id)
DO UPDATE SET can_read = EXCLUDED.can_read, can_edit = EXCLUDED.can_edit, can_comment = EXCLUDED.can_comment, can_review = EXCLUDED.can_review, can_manage = EXCLUDED.can_manage, updated_at = EXCLUDED.updated_at
RETURNING workspace_id, design_id, user_id, can_read, can_edit, can_comment, can_review, can_manage, created_at, updated_at
`, access.WorkspaceID, access.DesignID, access.UserID, access.CanRead, access.CanEdit, access.CanComment, access.CanReview, access.CanManage, now)
	return scanDesignAccess(row)
}

func (r *PostgresRepository) RevokeDesignAccess(ctx context.Context, workspaceID string, designID string, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM design_access WHERE workspace_id = $1 AND design_id = $2 AND user_id = $3`, strings.TrimSpace(workspaceID), strings.TrimSpace(designID), strings.TrimSpace(userID))
	return err
}

func (r *PostgresRepository) ListDesignGroupAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignGroupAccess, error) {
	rows, err := r.pool.Query(ctx, `
SELECT workspace_id, design_id, group_id, can_read, can_edit, can_comment, can_review, can_manage, created_at, updated_at
FROM design_group_access
WHERE workspace_id = $1 AND design_id = $2
ORDER BY group_id
`, strings.TrimSpace(workspaceID), strings.TrimSpace(designID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	access := []domain.DesignGroupAccess{}
	for rows.Next() {
		entry, err := scanDesignGroupAccess(rows)
		if err != nil {
			return nil, err
		}
		access = append(access, entry)
	}
	return access, rows.Err()
}

func (r *PostgresRepository) GrantDesignGroupAccess(ctx context.Context, access domain.DesignGroupAccess) (domain.DesignGroupAccess, error) {
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.DesignID = strings.TrimSpace(access.DesignID)
	access.GroupID = strings.TrimSpace(access.GroupID)
	if access.WorkspaceID == "" || access.DesignID == "" || access.GroupID == "" {
		return domain.DesignGroupAccess{}, errors.New("workspace id, design id, and group id are required")
	}
	now := r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
INSERT INTO design_group_access (workspace_id, design_id, group_id, can_read, can_edit, can_comment, can_review, can_manage, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
ON CONFLICT (workspace_id, design_id, group_id)
DO UPDATE SET can_read = EXCLUDED.can_read, can_edit = EXCLUDED.can_edit, can_comment = EXCLUDED.can_comment, can_review = EXCLUDED.can_review, can_manage = EXCLUDED.can_manage, updated_at = EXCLUDED.updated_at
RETURNING workspace_id, design_id, group_id, can_read, can_edit, can_comment, can_review, can_manage, created_at, updated_at
`, access.WorkspaceID, access.DesignID, access.GroupID, access.CanRead, access.CanEdit, access.CanComment, access.CanReview, access.CanManage, now)
	return scanDesignGroupAccess(row)
}

func (r *PostgresRepository) RevokeDesignGroupAccess(ctx context.Context, workspaceID string, designID string, groupID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM design_group_access WHERE workspace_id = $1 AND design_id = $2 AND group_id = $3`, strings.TrimSpace(workspaceID), strings.TrimSpace(designID), strings.TrimSpace(groupID))
	return err
}
