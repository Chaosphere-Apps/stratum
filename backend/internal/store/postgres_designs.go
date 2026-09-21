package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
	"time"
)

func (r *PostgresRepository) ListDesigns(ctx context.Context, workspaceID string) ([]domain.Design, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.Design, PageInfo, error) {
		return r.ListDesignsPage(ctx, workspaceID, options)
	})
}

func (r *PostgresRepository) ListDesignsPage(ctx context.Context, workspaceID string, options PageOptions) ([]domain.Design, PageInfo, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, PageInfo{}, errors.New("workspace id is required")
	}
	if workspaceID == domain.GuestWorkspaceID {
		if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
			return nil, PageInfo{}, err
		}
	}
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	likeQuery := "%" + strings.ToLower(options.Query) + "%"

	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at
FROM designs
WHERE workspace_id = $1 AND ($2 = '' OR lower(name) LIKE $3 OR lower(title) LIKE $3)
ORDER BY updated_at DESC
LIMIT $4 OFFSET $5
`, workspaceID, options.Query, likeQuery, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()

	designs := []domain.Design{}
	for rows.Next() {
		design, err := scanDesign(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		designs = append(designs, design)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(designs, options, offset)
	return items, page, nil
}

func (r *PostgresRepository) ListAccessibleDesignsPage(ctx context.Context, workspaceID string, scope AccessScope, options PageOptions) ([]domain.Design, PageInfo, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, PageInfo{}, errors.New("workspace id is required")
	}
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	likeQuery := "%" + strings.ToLower(options.Query) + "%"
	rows, err := r.pool.Query(ctx, `
SELECT design.id, design.workspace_id, design.name, design.access, design.title, design.document,
       design.canvas_snapshot, design.version_number, design.created_by, design.created_at, design.updated_at,
       CASE WHEN $4 OR design.created_by = $5 OR workspace.owner_id = $5
         OR EXISTS (SELECT 1 FROM workspace_access AS editable WHERE editable.workspace_id = design.workspace_id AND editable.user_id = $5 AND editable.can_manage)
         OR EXISTS (SELECT 1 FROM workspace_group_access AS editable WHERE editable.workspace_id = design.workspace_id AND editable.group_id = ANY($6::text[]) AND editable.can_manage)
         OR EXISTS (SELECT 1 FROM design_access AS editable WHERE editable.workspace_id = design.workspace_id AND editable.design_id = design.id AND editable.user_id = $5 AND (editable.can_edit OR editable.can_manage))
         OR EXISTS (SELECT 1 FROM design_group_access AS editable WHERE editable.workspace_id = design.workspace_id AND editable.design_id = design.id AND editable.group_id = ANY($6::text[]) AND (editable.can_edit OR editable.can_manage))
       THEN 'edit' ELSE 'read' END AS effective_access
FROM designs AS design
JOIN workspaces AS workspace ON workspace.id = design.workspace_id
WHERE design.workspace_id = $1
  AND ($2 = '' OR lower(design.name) LIKE $3 OR lower(design.title) LIKE $3)
  AND (
    $4 OR design.created_by = $5 OR workspace.owner_id = $5 OR design.access = 'public'
    OR EXISTS (
      SELECT 1 FROM workspace_access AS access
      WHERE access.workspace_id = design.workspace_id AND access.user_id = $5 AND access.can_manage
    )
    OR EXISTS (
      SELECT 1 FROM workspace_group_access AS access
      WHERE access.workspace_id = design.workspace_id AND access.group_id = ANY($6::text[]) AND access.can_manage
    )
    OR (design.access = 'workspace' AND (
      EXISTS (
        SELECT 1 FROM workspace_access AS access
        WHERE access.workspace_id = design.workspace_id AND access.user_id = $5
          AND (access.can_read OR access.can_create_design OR access.can_manage)
      )
      OR EXISTS (
        SELECT 1 FROM workspace_group_access AS access
        WHERE access.workspace_id = design.workspace_id AND access.group_id = ANY($6::text[])
          AND (access.can_read OR access.can_create_design OR access.can_manage)
      )
    ))
    OR EXISTS (
      SELECT 1 FROM design_access AS access
      WHERE access.workspace_id = design.workspace_id AND access.design_id = design.id AND access.user_id = $5
        AND (access.can_read OR access.can_edit OR access.can_comment OR access.can_review OR access.can_manage)
    )
    OR EXISTS (
      SELECT 1 FROM design_group_access AS access
      WHERE access.workspace_id = design.workspace_id AND access.design_id = design.id
        AND access.group_id = ANY($6::text[])
        AND (access.can_read OR access.can_edit OR access.can_comment OR access.can_review OR access.can_manage)
    )
  )
ORDER BY design.updated_at DESC
LIMIT $7 OFFSET $8
`, workspaceID, options.Query, likeQuery, scope.IsAdmin, scope.UserID, scope.GroupIDs, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()
	designs := []domain.Design{}
	for rows.Next() {
		design, err := scanDesignWithAccess(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		designs = append(designs, design)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(designs, options, offset)
	return items, page, nil
}

func (r *PostgresRepository) GetDesign(ctx context.Context, workspaceID string, designID string) (domain.Design, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" {
		return domain.Design{}, errors.New("workspace id and design id are required")
	}

	row := r.pool.QueryRow(ctx, `
SELECT id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at
FROM designs
WHERE workspace_id = $1 AND id = $2
`, workspaceID, designID)
	design, err := scanDesign(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Design{}, errors.New("design not found")
	}
	return design, err
}

func (r *PostgresRepository) CreateDesign(ctx context.Context, workspaceID string, name string, document []byte, createdBy string) (domain.Design, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return domain.Design{}, errors.New("workspace id is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Untitled system design"
	}

	now := r.clock().UTC()
	if strings.TrimSpace(createdBy) == "" {
		createdBy = r.firstUserID(ctx)
	}
	designID := fmt.Sprintf("design_%d", now.UnixNano())
	storedDocument, err := initialDesignDocument(document, designID, name, now)
	if err != nil {
		return domain.Design{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Design{}, err
	}
	defer rollback(ctx, tx)

	if workspaceID == domain.GuestWorkspaceID {
		if _, err := tx.Exec(ctx, `
INSERT INTO workspaces (id, name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
ON CONFLICT (id) DO NOTHING
`, domain.GuestWorkspaceID, "Guest Workspace", now); err != nil {
			return domain.Design{}, err
		}
	} else if err := ensureWorkspaceExists(ctx, tx, workspaceID); err != nil {
		return domain.Design{}, err
	}

	design := domain.Design{
		ID:               designID,
		WorkspaceID:      workspaceID,
		Name:             name,
		Access:           "private",
		Title:            name,
		Document:         storedDocument,
		DocumentRevision: domain.DesignRevision(storedDocument),
		VersionNumber:    0,
		CreatedBy:        createdBy,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO designs (id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, $8, $9, $10)
`, design.ID, design.WorkspaceID, design.Name, design.Access, design.Title, string(design.Document), design.VersionNumber, design.CreatedBy, design.CreatedAt, design.UpdatedAt); err != nil {
		return domain.Design{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE workspaces SET updated_at = $1 WHERE id = $2`, now, workspaceID); err != nil {
		return domain.Design{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Design{}, err
	}
	return design, nil
}

func (r *PostgresRepository) UpdateDesignMetadata(ctx context.Context, workspaceID string, designID string, name string, access string) (domain.Design, error) {
	existing, err := r.GetDesign(ctx, workspaceID, designID)
	if err != nil {
		return domain.Design{}, err
	}
	if trimmedName := strings.TrimSpace(name); trimmedName != "" {
		existing.Name = trimmedName
		existing.Title = trimmedName
	}
	if trimmedAccess := strings.TrimSpace(access); trimmedAccess != "" {
		existing.Access = trimmedAccess
	}
	if existing.Access == "" {
		existing.Access = "private"
	}
	existing.UpdatedAt = r.clock().UTC()

	row := r.pool.QueryRow(ctx, `
UPDATE designs
SET name = $1, access = $2, title = $3, updated_at = $4
WHERE workspace_id = $5 AND id = $6
RETURNING id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at
`, existing.Name, existing.Access, existing.Title, existing.UpdatedAt, workspaceID, designID)
	updated, err := scanDesign(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Design{}, errors.New("design not found")
	}
	return updated, err
}

func (r *PostgresRepository) UpsertDesign(ctx context.Context, design domain.Design) (domain.Design, error) {
	if strings.TrimSpace(design.ID) == "" {
		return domain.Design{}, errors.New("design id is required")
	}
	if strings.TrimSpace(design.WorkspaceID) == "" {
		return domain.Design{}, errors.New("workspace id is required")
	}
	if len(design.Document) == 0 || !json.Valid(design.Document) {
		return domain.Design{}, errors.New("design document is required")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Design{}, err
	}
	defer rollback(ctx, tx)

	now := r.clock().UTC()
	existing, err := selectDesignForUpdate(ctx, tx, design.WorkspaceID, design.ID)
	if err == nil {
		design.CreatedAt = existing.CreatedAt
		design.CreatedBy = existing.CreatedBy
		design.Name = existing.Name
		design.Access = existing.Access
		design.Title = existing.Title
		design.VersionNumber = existing.VersionNumber
	} else if errors.Is(err, pgx.ErrNoRows) {
		if design.CreatedBy == "" {
			design.CreatedBy = r.firstUserID(ctx)
		}
		if design.Name == "" {
			design.Name = design.Title
		}
		if design.Access == "" {
			design.Access = "private"
		}
		design.CreatedAt = now
		design.VersionNumber = 0
	} else {
		return domain.Design{}, err
	}
	if design.Title == "" {
		design.Title = design.Name
	}
	design.UpdatedAt = now
	design.DocumentRevision = domain.DesignRevision(design.Document)

	canvasSnapshot := nullableJSONText(design.CanvasSnapshot)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		if design.WorkspaceID == domain.GuestWorkspaceID {
			if _, err := tx.Exec(ctx, `
INSERT INTO workspaces (id, name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
ON CONFLICT (id) DO NOTHING
`, domain.GuestWorkspaceID, "Guest Workspace", now); err != nil {
				return domain.Design{}, err
			}
		} else if err := ensureWorkspaceExists(ctx, tx, design.WorkspaceID); err != nil {
			return domain.Design{}, err
		}
		_, err = tx.Exec(ctx, `
INSERT INTO designs (id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`, design.ID, design.WorkspaceID, design.Name, design.Access, design.Title, string(design.Document), canvasSnapshot, design.VersionNumber, design.CreatedBy, design.CreatedAt, design.UpdatedAt)
	} else {
		_, err = tx.Exec(ctx, `
UPDATE designs
SET document = $1, canvas_snapshot = $2, version_number = $3, updated_at = $4
WHERE workspace_id = $5 AND id = $6
`, string(design.Document), canvasSnapshot, design.VersionNumber, design.UpdatedAt, design.WorkspaceID, design.ID)
	}
	if err != nil {
		return domain.Design{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE workspaces SET updated_at = $1 WHERE id = $2`, now, design.WorkspaceID); err != nil {
		return domain.Design{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Design{}, err
	}
	return design, nil
}

func (r *PostgresRepository) UpdateDesignDocument(ctx context.Context, workspaceID string, designID string, document []byte, canvasSnapshot []byte, expectedRevision string) (domain.Design, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" {
		return domain.Design{}, errors.New("workspace id and design id are required")
	}
	if len(document) == 0 || !json.Valid(document) {
		return domain.Design{}, errors.New("design document must be valid JSON")
	}
	if expectedRevision == "" {
		return domain.Design{}, ErrDesignConflict
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Design{}, err
	}
	defer rollback(ctx, tx)
	existing, err := selectDesignForUpdate(ctx, tx, workspaceID, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Design{}, errors.New("design not found")
	}
	if err != nil {
		return domain.Design{}, err
	}
	if domain.DesignRevision(existing.Document) != expectedRevision {
		return domain.Design{}, ErrDesignConflict
	}

	now := r.clock().UTC()
	if !now.After(existing.UpdatedAt) {
		now = existing.UpdatedAt.Add(time.Nanosecond)
	}
	existing.Document = append(json.RawMessage(nil), document...)
	existing.DocumentRevision = domain.DesignRevision(existing.Document)
	if domain.ValidCanvasSnapshot(canvasSnapshot) {
		existing.CanvasSnapshot = append(json.RawMessage(nil), canvasSnapshot...)
	}
	existing.UpdatedAt = now
	if _, err := tx.Exec(ctx, `
UPDATE designs
SET document = $1, canvas_snapshot = $2, updated_at = $3
WHERE workspace_id = $4 AND id = $5
`, string(existing.Document), nullableJSONText(existing.CanvasSnapshot), now, workspaceID, designID); err != nil {
		return domain.Design{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE workspaces SET updated_at = $1 WHERE id = $2`, now, workspaceID); err != nil {
		return domain.Design{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Design{}, err
	}
	return existing, nil
}

func (r *PostgresRepository) DeleteDesign(ctx context.Context, workspaceID string, designID string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" {
		return errors.New("workspace id and design id are required")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	tag, err := tx.Exec(ctx, `DELETE FROM designs WHERE workspace_id = $1 AND id = $2`, workspaceID, designID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("design not found")
	}
	if _, err := tx.Exec(ctx, `UPDATE workspaces SET updated_at = $1 WHERE id = $2`, r.clock().UTC(), workspaceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListDesignVersions(ctx context.Context, workspaceID string, designID string) ([]domain.DesignVersion, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.DesignVersion, PageInfo, error) {
		return r.ListDesignVersionsPage(ctx, workspaceID, designID, options)
	})
}

func (r *PostgresRepository) ListDesignVersionsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignVersion, PageInfo, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	rows, err := r.pool.Query(ctx, `
SELECT id, design_id, workspace_id, version_number, status, remarks, document, canvas_snapshot, created_by, created_at, updated_at
FROM design_versions
WHERE workspace_id = $1 AND design_id = $2
ORDER BY version_number DESC
LIMIT $3 OFFSET $4
`, workspaceID, designID, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()

	versions := []domain.DesignVersion{}
	for rows.Next() {
		version, err := scanDesignVersion(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(versions, options, offset)
	return items, page, nil
}

func (r *PostgresRepository) CreateDesignVersion(ctx context.Context, workspaceID string, designID string, createdBy string, remarks string) (domain.DesignVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignVersion{}, err
	}
	defer rollback(ctx, tx)

	design, err := selectDesignForUpdate(ctx, tx, workspaceID, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignVersion{}, errors.New("design not found")
	}
	if err != nil {
		return domain.DesignVersion{}, err
	}
	if strings.TrimSpace(createdBy) == "" {
		createdBy = design.CreatedBy
	}

	var versionNumber int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version_number), 0) + 1 FROM design_versions WHERE workspace_id = $1 AND design_id = $2`, workspaceID, designID).Scan(&versionNumber); err != nil {
		return domain.DesignVersion{}, err
	}
	now := r.clock().UTC()
	versionID := fmt.Sprintf("%s_v%d", design.ID, versionNumber)
	row := tx.QueryRow(ctx, `
INSERT INTO design_versions (id, design_id, workspace_id, version_number, status, remarks, document, canvas_snapshot, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, 'draft', $5, $6, $7, $8, $9, $9)
RETURNING id, design_id, workspace_id, version_number, status, remarks, document, canvas_snapshot, created_by, created_at, updated_at
`, versionID, design.ID, design.WorkspaceID, versionNumber, strings.TrimSpace(remarks), string(design.Document), nullableJSONText(design.CanvasSnapshot), createdBy, now)
	version, err := scanDesignVersion(row)
	if err != nil {
		return domain.DesignVersion{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE designs SET version_number = $1, updated_at = $2 WHERE workspace_id = $3 AND id = $4`, versionNumber, now, workspaceID, designID); err != nil {
		return domain.DesignVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignVersion{}, err
	}
	return version, nil
}

func (r *PostgresRepository) UpdateDraftDesignVersion(ctx context.Context, workspaceID string, designID string, versionID string, document []byte, canvasSnapshot []byte, remarks string) (domain.DesignVersion, error) {
	if !json.Valid(document) {
		return domain.DesignVersion{}, errors.New("design document must be valid JSON")
	}
	row := r.pool.QueryRow(ctx, `
UPDATE design_versions
SET document = $1,
    canvas_snapshot = $2,
    remarks = CASE WHEN $3 <> '' THEN $3 ELSE remarks END,
    updated_at = $4
WHERE workspace_id = $5 AND design_id = $6 AND id = $7 AND status = 'draft'
RETURNING id, design_id, workspace_id, version_number, status, remarks, document, canvas_snapshot, created_by, created_at, updated_at
`, string(document), nullableJSONText(canvasSnapshot), strings.TrimSpace(remarks), r.clock().UTC(), workspaceID, designID, versionID)
	version, err := scanDesignVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		var status string
		lookupErr := r.pool.QueryRow(ctx, `SELECT status FROM design_versions WHERE workspace_id = $1 AND design_id = $2 AND id = $3`, workspaceID, designID, versionID).Scan(&status)
		if errors.Is(lookupErr, pgx.ErrNoRows) {
			return domain.DesignVersion{}, errors.New("version not found")
		}
		if lookupErr != nil {
			return domain.DesignVersion{}, lookupErr
		}
		return domain.DesignVersion{}, errors.New("only draft versions can be overwritten")
	}
	return version, err
}

func (r *PostgresRepository) UpdateDesignVersionStatus(ctx context.Context, workspaceID string, designID string, versionID string, status string) (domain.DesignVersion, error) {
	requestedStatus := status
	status = NormalizeDesignVersionStatus(status)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignVersion{}, err
	}
	defer rollback(ctx, tx)

	if _, err := selectDesignForUpdate(ctx, tx, workspaceID, designID); errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignVersion{}, errors.New("design not found")
	} else if err != nil {
		return domain.DesignVersion{}, err
	}
	var currentStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM design_versions WHERE workspace_id = $1 AND design_id = $2 AND id = $3`, workspaceID, designID, versionID).Scan(&currentStatus); errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignVersion{}, errors.New("version not found")
	} else if err != nil {
		return domain.DesignVersion{}, err
	}
	if err := ValidateDesignVersionStatusTransition(currentStatus, requestedStatus); err != nil {
		return domain.DesignVersion{}, err
	}
	now := r.clock().UTC()
	version, err := updateDesignVersionStatusTx(ctx, tx, workspaceID, designID, versionID, status, now)
	if err != nil {
		return domain.DesignVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignVersion{}, err
	}
	return version, nil
}

func (r *PostgresRepository) DeleteDesignVersion(ctx context.Context, workspaceID string, designID string, versionID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	if _, err := selectDesignForUpdate(ctx, tx, workspaceID, designID); errors.Is(err, pgx.ErrNoRows) {
		return errors.New("design not found")
	} else if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
DELETE FROM design_review_requests
WHERE workspace_id = $1 AND design_id = $2 AND version_id = $3
`, workspaceID, designID, versionID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
DELETE FROM design_versions
WHERE workspace_id = $1 AND design_id = $2 AND id = $3
`, workspaceID, designID, versionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("version not found")
	}
	now := r.clock().UTC()
	if _, err := tx.Exec(ctx, `
UPDATE designs
SET version_number = COALESCE((
	SELECT MAX(version_number)
	FROM design_versions
	WHERE workspace_id = $1 AND design_id = $2
), 0), updated_at = $3
WHERE workspace_id = $1 AND id = $2
`, workspaceID, designID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func resolveDesignVersionTx(ctx context.Context, tx pgx.Tx, workspaceID string, designID string, versionID string, fallbackVersionNumber int) (domain.DesignVersion, error) {
	versionID = strings.TrimSpace(versionID)
	var row pgx.Row
	if versionID != "" {
		row = tx.QueryRow(ctx, `
SELECT id, design_id, workspace_id, version_number, status, remarks, document, canvas_snapshot, created_by, created_at, updated_at
FROM design_versions
WHERE workspace_id = $1 AND design_id = $2 AND id = $3
`, workspaceID, designID, versionID)
	} else if fallbackVersionNumber > 0 {
		row = tx.QueryRow(ctx, `
SELECT id, design_id, workspace_id, version_number, status, remarks, document, canvas_snapshot, created_by, created_at, updated_at
FROM design_versions
WHERE workspace_id = $1 AND design_id = $2 AND version_number = $3
`, workspaceID, designID, fallbackVersionNumber)
	} else {
		return domain.DesignVersion{}, errors.New("design version is required before requesting review")
	}
	version, err := scanDesignVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignVersion{}, errors.New("version not found")
	}
	return version, err
}

func updateDesignVersionStatusTx(ctx context.Context, tx pgx.Tx, workspaceID string, designID string, versionID string, status string, now time.Time) (domain.DesignVersion, error) {
	if status == "live" {
		if _, err := tx.Exec(ctx, `
UPDATE design_versions
SET status = 'reviewed', updated_at = $1
WHERE workspace_id = $2 AND design_id = $3 AND status = 'live' AND id <> $4
`, now, workspaceID, designID, versionID); err != nil {
			return domain.DesignVersion{}, err
		}
	}
	row := tx.QueryRow(ctx, `
UPDATE design_versions
SET status = $1, updated_at = $2
WHERE workspace_id = $3 AND design_id = $4 AND id = $5
RETURNING id, design_id, workspace_id, version_number, status, remarks, document, canvas_snapshot, created_by, created_at, updated_at
`, status, now, workspaceID, designID, versionID)
	version, err := scanDesignVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignVersion{}, errors.New("version not found")
	}
	return version, err
}
