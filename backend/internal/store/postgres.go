package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/system-design-evaluator/backend/internal/domain"
)

type PostgresRepository struct {
	pool  *pgxpool.Pool
	clock func() time.Time
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	repo := &PostgresRepository{pool: pool, clock: time.Now}
	if err := repo.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return repo, nil
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

func (r *PostgresRepository) migrate(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS workspaces (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS designs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	access TEXT NOT NULL DEFAULT 'private',
	title TEXT NOT NULL,
	document TEXT NOT NULL,
	canvas_snapshot TEXT,
	version_number INTEGER NOT NULL,
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_designs_workspace_updated ON designs(workspace_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS design_versions (
	id TEXT PRIMARY KEY,
	design_id TEXT NOT NULL REFERENCES designs(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	version_number INTEGER NOT NULL,
	document TEXT NOT NULL,
	canvas_snapshot TEXT,
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	UNIQUE(design_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_design_versions_design_version ON design_versions(design_id, version_number DESC);
`)
	return err
}

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
	if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, name, created_at, updated_at
FROM workspaces
ORDER BY updated_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workspaces := []domain.Workspace{}
	for rows.Next() {
		workspace, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, rows.Err()
}

func (r *PostgresRepository) GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return domain.Workspace{}, errors.New("workspace id is required")
	}
	row := r.pool.QueryRow(ctx, `
SELECT id, name, created_at, updated_at
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

func (r *PostgresRepository) CreateWorkspace(ctx context.Context, name string) (domain.Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Workspace{}, errors.New("workspace name is required")
	}

	now := r.clock().UTC()
	workspace := domain.Workspace{
		ID:        fmt.Sprintf("workspace_%d", now.UnixNano()),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := r.pool.Exec(ctx, `
INSERT INTO workspaces (id, name, created_at, updated_at)
VALUES ($1, $2, $3, $4)
`, workspace.ID, workspace.Name, workspace.CreatedAt, workspace.UpdatedAt)
	if err != nil {
		return domain.Workspace{}, err
	}
	return workspace, nil
}

func (r *PostgresRepository) ListDesigns(ctx context.Context, workspaceID string) ([]domain.Design, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("workspace id is required")
	}
	if workspaceID == domain.GuestWorkspaceID {
		if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
			return nil, err
		}
	}

	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at
FROM designs
WHERE workspace_id = $1
ORDER BY updated_at DESC
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	designs := []domain.Design{}
	for rows.Next() {
		design, err := scanDesign(rows)
		if err != nil {
			return nil, err
		}
		designs = append(designs, design)
	}
	return designs, rows.Err()
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

func (r *PostgresRepository) CreateDesign(ctx context.Context, workspaceID string, name string, document []byte) (domain.Design, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return domain.Design{}, errors.New("workspace id is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Untitled system design"
	}

	now := r.clock().UTC()
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
		ID:            designID,
		WorkspaceID:   workspaceID,
		Name:          name,
		Access:        "private",
		Title:         name,
		Document:      storedDocument,
		VersionNumber: 1,
		CreatedBy:     domain.GuestUserID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO designs (id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, $8, $9, $10)
`, design.ID, design.WorkspaceID, design.Name, design.Access, design.Title, string(design.Document), design.VersionNumber, design.CreatedBy, design.CreatedAt, design.UpdatedAt); err != nil {
		return domain.Design{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO design_versions (id, design_id, workspace_id, version_number, document, canvas_snapshot, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, NULL, $6, $7)
`, fmt.Sprintf("%s_v1", design.ID), design.ID, design.WorkspaceID, design.VersionNumber, string(design.Document), design.CreatedBy, now); err != nil {
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
	versionNumber := 1
	if err == nil {
		design.CreatedAt = existing.CreatedAt
		design.CreatedBy = existing.CreatedBy
		design.Name = existing.Name
		design.Access = existing.Access
		design.Title = existing.Title
		versionNumber = existing.VersionNumber + 1
	} else if errors.Is(err, pgx.ErrNoRows) {
		if design.CreatedBy == "" {
			design.CreatedBy = domain.GuestUserID
		}
		if design.Name == "" {
			design.Name = design.Title
		}
		if design.Access == "" {
			design.Access = "private"
		}
		design.CreatedAt = now
	} else {
		return domain.Design{}, err
	}
	if design.Title == "" {
		design.Title = design.Name
	}
	design.VersionNumber = versionNumber
	design.UpdatedAt = now

	canvasSnapshot := nullableJSONText(design.CanvasSnapshot)
	if versionNumber == 1 {
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

	versionID := fmt.Sprintf("%s_v%d", design.ID, design.VersionNumber)
	if _, err := tx.Exec(ctx, `
INSERT INTO design_versions (id, design_id, workspace_id, version_number, document, canvas_snapshot, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`, versionID, design.ID, design.WorkspaceID, design.VersionNumber, string(design.Document), canvasSnapshot, design.CreatedBy, now); err != nil {
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

func (r *PostgresRepository) ListDesignVersions(ctx context.Context, workspaceID string, designID string) ([]domain.DesignVersion, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, design_id, workspace_id, version_number, document, canvas_snapshot, created_by, created_at
FROM design_versions
WHERE workspace_id = $1 AND design_id = $2
ORDER BY version_number DESC
`, workspaceID, designID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := []domain.DesignVersion{}
	for rows.Next() {
		version, err := scanDesignVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWorkspace(row rowScanner) (domain.Workspace, error) {
	var workspace domain.Workspace
	err := row.Scan(&workspace.ID, &workspace.Name, &workspace.CreatedAt, &workspace.UpdatedAt)
	return workspace, err
}

func scanDesign(row rowScanner) (domain.Design, error) {
	var design domain.Design
	var document string
	var canvasSnapshot *string
	err := row.Scan(
		&design.ID,
		&design.WorkspaceID,
		&design.Name,
		&design.Access,
		&design.Title,
		&document,
		&canvasSnapshot,
		&design.VersionNumber,
		&design.CreatedBy,
		&design.CreatedAt,
		&design.UpdatedAt,
	)
	if err != nil {
		return domain.Design{}, err
	}
	design.Document = json.RawMessage(document)
	if canvasSnapshot != nil {
		design.CanvasSnapshot = json.RawMessage(*canvasSnapshot)
	}
	return design, nil
}

func scanDesignVersion(row rowScanner) (domain.DesignVersion, error) {
	var version domain.DesignVersion
	var document string
	var canvasSnapshot *string
	err := row.Scan(
		&version.ID,
		&version.DesignID,
		&version.WorkspaceID,
		&version.VersionNumber,
		&document,
		&canvasSnapshot,
		&version.CreatedBy,
		&version.CreatedAt,
	)
	if err != nil {
		return domain.DesignVersion{}, err
	}
	version.Document = json.RawMessage(document)
	if canvasSnapshot != nil {
		version.CanvasSnapshot = json.RawMessage(*canvasSnapshot)
	}
	return version, nil
}

func ensureWorkspaceExists(ctx context.Context, tx pgx.Tx, workspaceID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workspaces WHERE id = $1)`, workspaceID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("workspace not found")
	}
	return nil
}

func selectDesignForUpdate(ctx context.Context, tx pgx.Tx, workspaceID string, designID string) (domain.Design, error) {
	row := tx.QueryRow(ctx, `
SELECT id, workspace_id, name, access, title, document, canvas_snapshot, version_number, created_by, created_at, updated_at
FROM designs
WHERE workspace_id = $1 AND id = $2
FOR UPDATE
`, workspaceID, designID)
	return scanDesign(row)
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func nullableJSONText(value json.RawMessage) *string {
	if len(value) == 0 {
		return nil
	}
	text := string(value)
	return &text
}
