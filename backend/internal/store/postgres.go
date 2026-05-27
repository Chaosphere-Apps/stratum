package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

CREATE TABLE IF NOT EXISTS design_docs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	design_id TEXT NOT NULL REFERENCES designs(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	body TEXT NOT NULL,
	format TEXT NOT NULL DEFAULT 'html',
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_design_docs_design_updated ON design_docs(workspace_id, design_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL,
	status TEXT NOT NULL,
	last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
UPDATE users SET last_seen_at = COALESCE(last_seen_at, updated_at, created_at, NOW()) WHERE last_seen_at IS NULL;
ALTER TABLE users ALTER COLUMN last_seen_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_role_status ON users(role, status);

CREATE TABLE IF NOT EXISTS workspace_access (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	can_read BOOLEAN NOT NULL DEFAULT FALSE,
	can_create_design BOOLEAN NOT NULL DEFAULT FALSE,
	can_manage BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (workspace_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_workspace_access_user ON workspace_access(user_id);

CREATE TABLE IF NOT EXISTS design_access (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	design_id TEXT NOT NULL REFERENCES designs(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	can_read BOOLEAN NOT NULL DEFAULT FALSE,
	can_edit BOOLEAN NOT NULL DEFAULT FALSE,
	can_comment BOOLEAN NOT NULL DEFAULT FALSE,
	can_review BOOLEAN NOT NULL DEFAULT FALSE,
	can_manage BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (workspace_id, design_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_design_access_user ON design_access(user_id);

CREATE TABLE IF NOT EXISTS auth_sessions (
	token_hash TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL,
	last_seen_at TIMESTAMPTZ NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
UPDATE auth_sessions SET expires_at = created_at + INTERVAL '12 hours' WHERE expires_at IS NULL;
ALTER TABLE auth_sessions ALTER COLUMN expires_at SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires ON auth_sessions(expires_at);

CREATE TABLE IF NOT EXISTS sign_in_settings (
	id TEXT PRIMARY KEY,
	local_password_enabled BOOLEAN NOT NULL,
	sso_enabled BOOLEAN NOT NULL,
	provider TEXT NOT NULL,
	okta_domain TEXT NOT NULL DEFAULT '',
	issuer TEXT NOT NULL DEFAULT '',
	client_id TEXT NOT NULL DEFAULT '',
	client_secret TEXT NOT NULL DEFAULT '',
	redirect_uri TEXT NOT NULL DEFAULT '',
	post_logout_redirect_uri TEXT NOT NULL DEFAULT '',
	scopes TEXT NOT NULL DEFAULT 'openid profile email groups',
	groups_claim TEXT NOT NULL DEFAULT 'groups',
	admin_group TEXT NOT NULL DEFAULT '',
	reviewer_group TEXT NOT NULL DEFAULT '',
	jit_provisioning BOOLEAN NOT NULL DEFAULT TRUE,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_provider_settings (
	id TEXT PRIMARY KEY,
	enabled BOOLEAN NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	base_url TEXT NOT NULL DEFAULT '',
	api_key TEXT NOT NULL DEFAULT '',
	verified_at TIMESTAMPTZ,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS mcp_settings (
	id TEXT PRIMARY KEY,
	enabled BOOLEAN NOT NULL,
	endpoint_path TEXT NOT NULL,
	read_catalog BOOLEAN NOT NULL,
	read_designs BOOLEAN NOT NULL,
	create_draft_design BOOLEAN NOT NULL,
	run_analysis BOOLEAN NOT NULL,
	fetch_impact_report BOOLEAN NOT NULL,
	require_admin_consent BOOLEAN NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS catalog_assets (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	normalized_name TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL,
	owner TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	criticality TEXT NOT NULL DEFAULT 'medium',
	tags TEXT NOT NULL DEFAULT '',
	metadata TEXT NOT NULL DEFAULT '{}',
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_catalog_assets_type ON catalog_assets(type);
CREATE INDEX IF NOT EXISTS idx_catalog_assets_updated ON catalog_assets(updated_at DESC);

CREATE TABLE IF NOT EXISTS design_comments (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	design_id TEXT NOT NULL REFERENCES designs(id) ON DELETE CASCADE,
	author_id TEXT NOT NULL,
	body TEXT NOT NULL,
	component_id TEXT NOT NULL DEFAULT '',
	connector_id TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_design_comments_design_created ON design_comments(workspace_id, design_id, created_at DESC);

CREATE TABLE IF NOT EXISTS design_review_requests (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	design_id TEXT NOT NULL REFERENCES designs(id) ON DELETE CASCADE,
	requested_by TEXT NOT NULL,
	reviewer_id TEXT NOT NULL,
	status TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_design_review_requests_design_updated ON design_review_requests(workspace_id, design_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_design_review_requests_reviewer_updated ON design_review_requests(reviewer_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS notifications (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	workspace_id TEXT NOT NULL,
	design_id TEXT NOT NULL,
	type TEXT NOT NULL,
	title TEXT NOT NULL,
	body TEXT NOT NULL,
	read BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, read, created_at DESC);
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
		ID:            designID,
		WorkspaceID:   workspaceID,
		Name:          name,
		Access:        "private",
		Title:         name,
		Document:      storedDocument,
		VersionNumber: 1,
		CreatedBy:     createdBy,
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
			design.CreatedBy = r.firstUserID(ctx)
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

func (r *PostgresRepository) ListDesignDocs(ctx context.Context, workspaceID string, designID string) ([]domain.DesignDoc, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, design_id, title, body, format, created_by, created_at, updated_at
FROM design_docs
WHERE workspace_id = $1 AND design_id = $2
ORDER BY updated_at DESC
`, workspaceID, designID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []domain.DesignDoc{}
	for rows.Next() {
		doc, err := scanDesignDoc(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (r *PostgresRepository) GetDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) (domain.DesignDoc, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" || strings.TrimSpace(docID) == "" {
		return domain.DesignDoc{}, errors.New("workspace id, design id, and doc id are required")
	}
	row := r.pool.QueryRow(ctx, `
SELECT id, workspace_id, design_id, title, body, format, created_by, created_at, updated_at
FROM design_docs
WHERE workspace_id = $1 AND design_id = $2 AND id = $3
`, workspaceID, designID, docID)
	doc, err := scanDesignDoc(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignDoc{}, errors.New("design doc not found")
	}
	return doc, err
}

func (r *PostgresRepository) CreateDesignDoc(ctx context.Context, workspaceID string, designID string, title string, body string, format string) (domain.DesignDoc, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" {
		return domain.DesignDoc{}, errors.New("workspace id and design id are required")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignDoc{}, err
	}
	defer rollback(ctx, tx)

	if err := ensureDesignExists(ctx, tx, workspaceID, designID); err != nil {
		return domain.DesignDoc{}, err
	}
	now := r.clock().UTC()
	doc := domain.DesignDoc{
		ID:          fmt.Sprintf("doc_%d", now.UnixNano()),
		WorkspaceID: workspaceID,
		DesignID:    designID,
		Title:       normalizedDocTitle(title),
		Body:        body,
		Format:      normalizedDocFormat(format),
		CreatedBy:   r.firstUserID(ctx),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO design_docs (id, workspace_id, design_id, title, body, format, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
`, doc.ID, doc.WorkspaceID, doc.DesignID, doc.Title, doc.Body, doc.Format, doc.CreatedBy, now); err != nil {
		return domain.DesignDoc{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignDoc{}, err
	}
	return doc, nil
}

func (r *PostgresRepository) UpdateDesignDoc(ctx context.Context, workspaceID string, designID string, docID string, title string, body string, format string) (domain.DesignDoc, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" || strings.TrimSpace(docID) == "" {
		return domain.DesignDoc{}, errors.New("workspace id, design id, and doc id are required")
	}
	now := r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
UPDATE design_docs
SET title = $1, body = $2, format = $3, updated_at = $4
WHERE workspace_id = $5 AND design_id = $6 AND id = $7
RETURNING id, workspace_id, design_id, title, body, format, created_by, created_at, updated_at
`, normalizedDocTitle(title), body, normalizedDocFormat(format), now, workspaceID, designID, docID)
	doc, err := scanDesignDoc(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignDoc{}, errors.New("design doc not found")
	}
	return doc, err
}

func (r *PostgresRepository) DeleteDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" || strings.TrimSpace(docID) == "" {
		return errors.New("workspace id, design id, and doc id are required")
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM design_docs WHERE workspace_id = $1 AND design_id = $2 AND id = $3`, workspaceID, designID, docID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("design doc not found")
	}
	return nil
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

func (r *PostgresRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
FROM users
ORDER BY created_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []domain.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *PostgresRepository) GetUser(ctx context.Context, userID string) (domain.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = r.firstUserID(ctx)
	}
	row := r.pool.QueryRow(ctx, `
SELECT id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
FROM users
WHERE id = $1 AND status <> 'disabled'
`, userID)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("user not found")
	}
	return user, err
}

func (r *PostgresRepository) AuthenticateUser(ctx context.Context, email string, password string) (domain.User, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
FROM users
WHERE lower(email) = lower($1) AND status <> 'disabled'
`, strings.TrimSpace(email))
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) || !user.PasswordSet || !verifyPassword(password, user.PasswordHash) {
		return domain.User{}, errors.New("invalid email or password")
	}
	if err != nil {
		return domain.User{}, err
	}
	now := r.clock().UTC()
	row = r.pool.QueryRow(ctx, `
UPDATE users
SET last_seen_at = $1
WHERE id = $2
RETURNING id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
`, now, user.ID)
	return scanUser(row)
}

func (r *PostgresRepository) CreateSession(ctx context.Context, userID string, expiresAt time.Time) (string, error) {
	if _, err := r.GetUser(ctx, userID); err != nil {
		return "", err
	}
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	now := r.clock().UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(12 * time.Hour)
	}
	if _, err := r.pool.Exec(ctx, `
INSERT INTO auth_sessions (token_hash, user_id, created_at, last_seen_at, expires_at)
VALUES ($1, $2, $3, $3, $4)
`, tokenHash, userID, now, expiresAt.UTC()); err != nil {
		return "", err
	}
	return token, nil
}

func (r *PostgresRepository) GetUserBySessionToken(ctx context.Context, token string) (domain.User, error) {
	var userID string
	if err := r.pool.QueryRow(ctx, `
UPDATE auth_sessions
SET last_seen_at = $1
WHERE token_hash = $2 AND expires_at > $1
RETURNING user_id
`, r.clock().UTC(), sessionTokenHash(token)).Scan(&userID); errors.Is(err, pgx.ErrNoRows) {
		_, _ = r.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash = $1`, sessionTokenHash(token))
		return domain.User{}, errors.New("session not found")
	} else if err != nil {
		return domain.User{}, err
	}
	return r.GetUser(ctx, userID)
}

func (r *PostgresRepository) DeleteSession(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash = $1`, sessionTokenHash(token))
	return err
}

func (r *PostgresRepository) PasswordSetupRequired(ctx context.Context) (bool, error) {
	var userCount int
	var passwordCount int
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*), COUNT(*) FILTER (WHERE password_hash <> '')
FROM users
`).Scan(&userCount, &passwordCount); err != nil {
		return false, err
	}
	return userCount > 0 && passwordCount == 0, nil
}

func (r *PostgresRepository) SetInitialAdminPassword(ctx context.Context, email string, password string) (domain.User, error) {
	requiresSetup, err := r.PasswordSetupRequired(ctx)
	if err != nil {
		return domain.User{}, err
	}
	if !requiresSetup {
		return domain.User{}, errors.New("password setup is not available")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	row := r.pool.QueryRow(ctx, `
UPDATE users
SET password_hash = $1, updated_at = $2
WHERE lower(email) = lower($3) AND role = 'admin' AND status <> 'disabled'
RETURNING id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
`, passwordHash, r.clock().UTC(), strings.TrimSpace(email))
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("admin user not found")
	}
	return user, err
}

func (r *PostgresRepository) CreateFirstAdmin(ctx context.Context, displayName string, email string, password string) (domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer rollback(ctx, tx)

	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return domain.User{}, err
	}
	if count > 0 {
		return domain.User{}, errors.New("organization already has users")
	}
	user, err := createUserWithTx(ctx, tx, r.clock().UTC(), displayName, email, "admin", password)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return domain.User{}, errors.New("a user with this email already exists")
		}
		return domain.User{}, err
	}
	if err := claimLegacyGuestData(ctx, tx, user.ID); err != nil {
		return domain.User{}, err
	}
	if err := ensureDefaultSignInConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (r *PostgresRepository) CreateUser(ctx context.Context, displayName string, email string, role string, password string) (domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer rollback(ctx, tx)

	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return domain.User{}, err
	}
	if count == 0 {
		return domain.User{}, errors.New("create the first admin before inviting users")
	}
	user, err := createUserWithTx(ctx, tx, r.clock().UTC(), displayName, email, role, password)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return domain.User{}, errors.New("a user with this email already exists")
		}
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (r *PostgresRepository) UpdateUser(ctx context.Context, userID string, displayName string, email string, role string, status string, password string) (domain.User, error) {
	existing, err := r.GetUser(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	if strings.TrimSpace(displayName) != "" {
		existing.DisplayName = strings.TrimSpace(displayName)
	}
	if strings.TrimSpace(email) != "" {
		existing.Email = strings.ToLower(strings.TrimSpace(email))
	}
	if strings.TrimSpace(role) != "" {
		existing.Role = normalizedUserRole(role)
	}
	if strings.TrimSpace(status) != "" {
		existing.Status = normalizedUserStatus(status)
	}
	if strings.TrimSpace(password) != "" {
		passwordHash, err := hashPassword(password)
		if err != nil {
			return domain.User{}, err
		}
		existing.PasswordHash = passwordHash
		existing.PasswordSet = true
	}
	existing.UpdatedAt = r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
UPDATE users
SET display_name = $1, email = $2, password_hash = $3, role = $4, status = $5, updated_at = $6
WHERE id = $7
RETURNING id, display_name, email, password_hash <> '', password_hash, role, status, last_seen_at, created_at, updated_at
`, existing.DisplayName, existing.Email, existing.PasswordHash, existing.Role, existing.Status, existing.UpdatedAt, existing.ID)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("user not found")
	}
	if isUniqueViolation(err, "users_email_key") {
		return domain.User{}, errors.New("a user with this email already exists")
	}
	return user, err
}

func (r *PostgresRepository) DeleteUser(ctx context.Context, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	var role string
	if err := tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&role); errors.Is(err, pgx.ErrNoRows) {
		return errors.New("user not found")
	} else if err != nil {
		return err
	}
	if role == "admin" {
		var adminCount int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND status <> 'disabled'`).Scan(&adminCount); err != nil {
			return err
		}
		if adminCount <= 1 {
			return errors.New("at least one admin is required")
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) GetSignInConfig(ctx context.Context) (domain.SignInConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.SignInConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultSignInConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.SignInConfig{}, err
	}
	config, err := getSignInConfigWithTx(ctx, tx)
	if err != nil {
		return domain.SignInConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.SignInConfig{}, err
	}
	return config, nil
}

func (r *PostgresRepository) UpdateSignInConfig(ctx context.Context, config domain.SignInConfig, clientSecret string) (domain.SignInConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.SignInConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultSignInConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.SignInConfig{}, err
	}
	currentSecret := ""
	if err := tx.QueryRow(ctx, `SELECT client_secret FROM sign_in_settings WHERE id = 'default' FOR UPDATE`).Scan(&currentSecret); err != nil {
		return domain.SignInConfig{}, err
	}
	if strings.TrimSpace(clientSecret) != "" {
		currentSecret = strings.TrimSpace(clientSecret)
	}
	updatedAt := r.clock().UTC()
	row := tx.QueryRow(ctx, `
UPDATE sign_in_settings
SET local_password_enabled = $1,
	sso_enabled = $2,
	provider = $3,
	okta_domain = $4,
	issuer = $5,
	client_id = $6,
	client_secret = $7,
	redirect_uri = $8,
	post_logout_redirect_uri = $9,
	scopes = $10,
	groups_claim = $11,
	admin_group = $12,
	reviewer_group = $13,
	jit_provisioning = $14,
	updated_at = $15
WHERE id = 'default'
RETURNING local_password_enabled, sso_enabled, provider, okta_domain, issuer, client_id, client_secret <> '', redirect_uri, post_logout_redirect_uri, scopes, groups_claim, admin_group, reviewer_group, jit_provisioning, updated_at
`, config.LocalPasswordEnabled, config.SSOEnabled, strings.TrimSpace(config.Provider), strings.TrimSpace(config.OktaDomain), strings.TrimSpace(config.Issuer), strings.TrimSpace(config.ClientID), currentSecret, strings.TrimSpace(config.RedirectURI), strings.TrimSpace(config.PostLogoutRedirectURI), normalizedScopes(config.Scopes), strings.TrimSpace(config.GroupsClaim), strings.TrimSpace(config.AdminGroup), strings.TrimSpace(config.ReviewerGroup), config.JITProvisioning, updatedAt)
	next, err := scanSignInConfig(row)
	if err != nil {
		return domain.SignInConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.SignInConfig{}, err
	}
	return next, nil
}

func (r *PostgresRepository) GetAIProviderConfig(ctx context.Context) (domain.AIProviderConfig, error) {
	config, err := r.getAIProviderConfig(ctx, false)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	return config, nil
}

func (r *PostgresRepository) GetAIProviderConfigWithSecret(ctx context.Context) (domain.AIProviderConfig, error) {
	return r.getAIProviderConfig(ctx, true)
}

func (r *PostgresRepository) getAIProviderConfig(ctx context.Context, includeSecret bool) (domain.AIProviderConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultAIProviderConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.AIProviderConfig{}, err
	}
	row := tx.QueryRow(ctx, `
SELECT enabled, provider, model, base_url, api_key, api_key <> '', verified_at, updated_at
FROM ai_provider_settings
WHERE id = 'default'
`)
	config, err := scanAIProviderConfig(row)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AIProviderConfig{}, err
	}
	if !includeSecret {
		config.APIKey = ""
	}
	return config, nil
}

func (r *PostgresRepository) UpdateAIProviderConfig(ctx context.Context, config domain.AIProviderConfig, apiKey string) (domain.AIProviderConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultAIProviderConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.AIProviderConfig{}, err
	}
	currentSecret := ""
	var currentVerifiedAt sql.NullTime
	if err := tx.QueryRow(ctx, `SELECT api_key, verified_at FROM ai_provider_settings WHERE id = 'default' FOR UPDATE`).Scan(&currentSecret, &currentVerifiedAt); err != nil {
		return domain.AIProviderConfig{}, err
	}
	verifiedAt := time.Time{}
	if currentVerifiedAt.Valid {
		verifiedAt = currentVerifiedAt.Time
	}
	if strings.TrimSpace(apiKey) != "" {
		currentSecret = strings.TrimSpace(apiKey)
		verifiedAt = r.clock().UTC()
	}
	provider := normalizedAIProvider(config.Provider)
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = defaultAIModel(provider)
	}
	updatedAt := r.clock().UTC()
	row := tx.QueryRow(ctx, `
UPDATE ai_provider_settings
SET enabled = $1,
	provider = $2,
	model = $3,
	base_url = $4,
	api_key = $5,
	verified_at = $6,
	updated_at = $7
WHERE id = 'default'
RETURNING enabled, provider, model, base_url, api_key, api_key <> '', verified_at, updated_at
`, config.Enabled, provider, model, strings.TrimSpace(config.BaseURL), currentSecret, nullableTime(verifiedAt), updatedAt)
	next, err := scanAIProviderConfig(row)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AIProviderConfig{}, err
	}
	return sanitizeAIProviderConfig(next), nil
}

func (r *PostgresRepository) GetMCPConfig(ctx context.Context) (domain.MCPConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.MCPConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultMCPConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.MCPConfig{}, err
	}
	row := tx.QueryRow(ctx, `
SELECT enabled, endpoint_path, read_catalog, read_designs, create_draft_design, run_analysis, fetch_impact_report, require_admin_consent, updated_at
FROM mcp_settings
WHERE id = 'default'
`)
	config, err := scanMCPConfig(row)
	if err != nil {
		return domain.MCPConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.MCPConfig{}, err
	}
	return config, nil
}

func (r *PostgresRepository) UpdateMCPConfig(ctx context.Context, config domain.MCPConfig) (domain.MCPConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.MCPConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultMCPConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.MCPConfig{}, err
	}
	row := tx.QueryRow(ctx, `
UPDATE mcp_settings
SET enabled = $1,
	endpoint_path = $2,
	read_catalog = $3,
	read_designs = $4,
	create_draft_design = $5,
	run_analysis = $6,
	fetch_impact_report = $7,
	require_admin_consent = $8,
	updated_at = $9
WHERE id = 'default'
RETURNING enabled, endpoint_path, read_catalog, read_designs, create_draft_design, run_analysis, fetch_impact_report, require_admin_consent, updated_at
`, config.Enabled, normalizedMCPPath(config.EndpointPath), config.ReadCatalog, config.ReadDesigns, config.CreateDraftDesign, config.RunAnalysis, config.FetchImpactReport, config.RequireAdminConsent, r.clock().UTC())
	next, err := scanMCPConfig(row)
	if err != nil {
		return domain.MCPConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.MCPConfig{}, err
	}
	return next, nil
}

func (r *PostgresRepository) ListDesignComments(ctx context.Context, workspaceID string, designID string) ([]domain.DesignComment, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, design_id, author_id, body, component_id, connector_id, created_at
FROM design_comments
WHERE workspace_id = $1 AND design_id = $2
ORDER BY created_at DESC
`, workspaceID, designID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []domain.DesignComment{}
	for rows.Next() {
		comment, err := scanDesignComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (r *PostgresRepository) CreateDesignComment(ctx context.Context, workspaceID string, designID string, authorID string, body string, componentID string, connectorID string) (domain.DesignComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return domain.DesignComment{}, errors.New("comment body is required")
	}
	if authorID == "" {
		authorID = r.firstUserID(ctx)
	}
	author, err := r.GetUser(ctx, authorID)
	if err != nil {
		return domain.DesignComment{}, err
	}
	authorID = author.ID

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignComment{}, err
	}
	defer rollback(ctx, tx)

	design, err := selectDesignForUpdate(ctx, tx, workspaceID, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignComment{}, errors.New("design not found")
	}
	if err != nil {
		return domain.DesignComment{}, err
	}

	now := r.clock().UTC()
	comment := domain.DesignComment{
		ID:          fmt.Sprintf("comment_%d", now.UnixNano()),
		WorkspaceID: workspaceID,
		DesignID:    designID,
		AuthorID:    authorID,
		Body:        body,
		ComponentID: strings.TrimSpace(componentID),
		ConnectorID: strings.TrimSpace(connectorID),
		CreatedAt:   now,
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO design_comments (id, workspace_id, design_id, author_id, body, component_id, connector_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`, comment.ID, comment.WorkspaceID, comment.DesignID, comment.AuthorID, comment.Body, comment.ComponentID, comment.ConnectorID, comment.CreatedAt); err != nil {
		return domain.DesignComment{}, err
	}
	if design.CreatedBy != "" && design.CreatedBy != authorID {
		if err := insertNotification(ctx, tx, notificationFor(design.CreatedBy, workspaceID, designID, "comment_added", "New comment", author.DisplayName+" commented on "+design.Name, now)); err != nil {
			return domain.DesignComment{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignComment{}, err
	}
	return comment, nil
}

func (r *PostgresRepository) ListDesignReviewRequests(ctx context.Context, workspaceID string, designID string) ([]domain.DesignReviewRequest, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, design_id, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at
FROM design_review_requests
WHERE workspace_id = $1 AND design_id = $2
ORDER BY updated_at DESC
`, workspaceID, designID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reviews := []domain.DesignReviewRequest{}
	for rows.Next() {
		review, err := scanDesignReviewRequest(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (r *PostgresRepository) CreateDesignReviewRequest(ctx context.Context, workspaceID string, designID string, requestedBy string, reviewerID string, message string) (domain.DesignReviewRequest, error) {
	reviewerID = strings.TrimSpace(reviewerID)
	if reviewerID == "" {
		return domain.DesignReviewRequest{}, errors.New("reviewer id is required")
	}
	reviewer, err := r.GetUser(ctx, reviewerID)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	reviewerID = reviewer.ID
	if requestedBy == "" {
		requestedBy = r.firstUserID(ctx)
	}
	requester, err := r.GetUser(ctx, requestedBy)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	requestedBy = requester.ID

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	defer rollback(ctx, tx)

	design, err := selectDesignForUpdate(ctx, tx, workspaceID, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignReviewRequest{}, errors.New("design not found")
	}
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}

	now := r.clock().UTC()
	review := domain.DesignReviewRequest{
		ID:          fmt.Sprintf("review_%d", now.UnixNano()),
		WorkspaceID: workspaceID,
		DesignID:    designID,
		RequestedBy: requestedBy,
		ReviewerID:  reviewerID,
		Status:      "requested",
		Message:     strings.TrimSpace(message),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO design_review_requests (id, workspace_id, design_id, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, '', $8, $8, NULL)
`, review.ID, review.WorkspaceID, review.DesignID, review.RequestedBy, review.ReviewerID, review.Status, review.Message, now); err != nil {
		return domain.DesignReviewRequest{}, err
	}
	if err := insertNotification(ctx, tx, notificationFor(reviewerID, workspaceID, designID, "review_requested", "Review requested", requester.DisplayName+" requested your review on "+design.Name, now)); err != nil {
		return domain.DesignReviewRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignReviewRequest{}, err
	}
	return review, nil
}

func (r *PostgresRepository) UpdateDesignReviewRequest(ctx context.Context, workspaceID string, designID string, reviewID string, status string, summary string) (domain.DesignReviewRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	defer rollback(ctx, tx)

	design, err := selectDesignForUpdate(ctx, tx, workspaceID, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignReviewRequest{}, errors.New("design not found")
	}
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}

	row := tx.QueryRow(ctx, `
SELECT id, workspace_id, design_id, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at
FROM design_review_requests
WHERE workspace_id = $1 AND design_id = $2 AND id = $3
FOR UPDATE
`, workspaceID, designID, reviewID)
	existing, err := scanDesignReviewRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignReviewRequest{}, errors.New("review request not found")
	}
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}

	now := r.clock().UTC()
	nextStatus := normalizedReviewStatus(status, existing.Status)
	reviewer, err := r.GetUser(ctx, existing.ReviewerID)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	var completedAt *time.Time
	if nextStatus == "approved" || nextStatus == "changes_requested" {
		completedAt = &now
	}
	row = tx.QueryRow(ctx, `
UPDATE design_review_requests
SET status = $1, summary = $2, updated_at = $3, completed_at = $4
WHERE workspace_id = $5 AND design_id = $6 AND id = $7
RETURNING id, workspace_id, design_id, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at
`, nextStatus, strings.TrimSpace(summary), now, completedAt, workspaceID, designID, reviewID)
	review, err := scanDesignReviewRequest(row)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	if design.CreatedBy != "" && design.CreatedBy != review.ReviewerID {
		if err := insertNotification(ctx, tx, notificationFor(design.CreatedBy, workspaceID, designID, "review_updated", "Review updated", reviewer.DisplayName+" marked review as "+strings.ReplaceAll(nextStatus, "_", " "), now)); err != nil {
			return domain.DesignReviewRequest{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignReviewRequest{}, err
	}
	return review, nil
}

func (r *PostgresRepository) ListNotifications(ctx context.Context, userID string) ([]domain.Notification, error) {
	if strings.TrimSpace(userID) == "" {
		userID = r.firstUserID(ctx)
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, user_id, workspace_id, design_id, type, title, body, read, created_at
FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 80
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := []domain.Notification{}
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (r *PostgresRepository) MarkNotificationRead(ctx context.Context, userID string, notificationID string) error {
	if strings.TrimSpace(userID) == "" {
		userID = r.firstUserID(ctx)
	}
	tag, err := r.pool.Exec(ctx, `UPDATE notifications SET read = TRUE WHERE user_id = $1 AND id = $2`, userID, notificationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("notification not found")
	}
	return nil
}

func (r *PostgresRepository) ListCatalogAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error) {
	normalizedQuery := normalizedCatalogName(query)
	likeQuery := "%" + normalizedQuery + "%"
	rows, err := r.pool.Query(ctx, `
SELECT
	ca.id, ca.name, ca.normalized_name, ca.type, ca.owner, ca.description, ca.criticality, ca.tags, ca.metadata,
	ca.created_by, ca.created_at, ca.updated_at,
	COUNT(DISTINCT d.id)::INT AS used_in_design_count
FROM catalog_assets ca
LEFT JOIN designs d ON d.document LIKE '%"assetId":"' || ca.id || '"%'
WHERE $1 = '' OR ca.normalized_name LIKE $2
GROUP BY ca.id
ORDER BY (ca.normalized_name = $1) DESC, ca.name ASC
`, normalizedQuery, likeQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assets := []domain.CatalogAsset{}
	for rows.Next() {
		asset, err := scanCatalogAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (r *PostgresRepository) CreateCatalogAsset(ctx context.Context, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	asset.Name = strings.TrimSpace(asset.Name)
	if asset.Name == "" {
		return domain.CatalogAsset{}, errors.New("catalog asset name is required")
	}
	asset.NormalizedName = normalizedCatalogName(asset.Name)
	if strings.TrimSpace(asset.Type) == "" {
		asset.Type = "compute.service"
	}
	if strings.TrimSpace(asset.Criticality) == "" {
		asset.Criticality = "medium"
	}
	if len(asset.Metadata) == 0 {
		asset.Metadata = json.RawMessage(`{}`)
	}
	now := r.clock().UTC()
	asset.ID = fmt.Sprintf("asset_%d", now.UnixNano())
	asset.CreatedBy = strings.TrimSpace(asset.CreatedBy)
	if asset.CreatedBy == "" {
		asset.CreatedBy = r.firstUserID(ctx)
	}
	row := r.pool.QueryRow(ctx, `
INSERT INTO catalog_assets (id, name, normalized_name, type, owner, description, criticality, tags, metadata, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
RETURNING id, name, normalized_name, type, owner, description, criticality, tags, metadata, created_by, created_at, updated_at, 0
`, asset.ID, asset.Name, asset.NormalizedName, strings.TrimSpace(asset.Type), strings.TrimSpace(asset.Owner), strings.TrimSpace(asset.Description), strings.TrimSpace(asset.Criticality), strings.Join(normalizedTags(asset.Tags), ","), string(asset.Metadata), asset.CreatedBy, now)
	created, err := scanCatalogAsset(row)
	if isUniqueViolation(err, "catalog_assets_normalized_name_key") {
		return domain.CatalogAsset{}, errors.New("catalog asset already exists")
	}
	return created, err
}

func (r *PostgresRepository) UpdateCatalogAsset(ctx context.Context, assetID string, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	asset.Name = strings.TrimSpace(asset.Name)
	if asset.Name == "" {
		return domain.CatalogAsset{}, errors.New("catalog asset name is required")
	}
	normalized := normalizedCatalogName(asset.Name)
	if strings.TrimSpace(asset.Type) == "" {
		asset.Type = "compute.service"
	}
	if strings.TrimSpace(asset.Criticality) == "" {
		asset.Criticality = "medium"
	}
	metadata := asset.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	row := r.pool.QueryRow(ctx, `
WITH updated AS (
	UPDATE catalog_assets
	SET name = $2,
		normalized_name = $3,
		type = $4,
		owner = $5,
		description = $6,
		criticality = $7,
		tags = $8,
		metadata = $9,
		updated_at = $10
	WHERE id = $1
	RETURNING id, name, normalized_name, type, owner, description, criticality, tags, metadata, created_by, created_at, updated_at
)
SELECT updated.*, COUNT(DISTINCT d.id)::INT AS used_in_design_count
FROM updated
LEFT JOIN designs d ON d.document LIKE '%"assetId":"' || updated.id || '"%'
GROUP BY updated.id, updated.name, updated.normalized_name, updated.type, updated.owner, updated.description, updated.criticality, updated.tags, updated.metadata, updated.created_by, updated.created_at, updated.updated_at
`, assetID, asset.Name, normalized, strings.TrimSpace(asset.Type), strings.TrimSpace(asset.Owner), strings.TrimSpace(asset.Description), strings.TrimSpace(asset.Criticality), strings.Join(normalizedTags(asset.Tags), ","), string(metadata), r.clock().UTC())
	updated, err := scanCatalogAsset(row)
	if isUniqueViolation(err, "catalog_assets_normalized_name_key") {
		return domain.CatalogAsset{}, errors.New("catalog asset already exists")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CatalogAsset{}, errors.New("catalog asset not found")
	}
	return updated, err
}

func (r *PostgresRepository) DeleteCatalogAsset(ctx context.Context, assetID string) error {
	var usageCount int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT id)::INT FROM designs WHERE document LIKE '%"assetId":"' || $1 || '"%'`, assetID).Scan(&usageCount); err != nil {
		return err
	}
	if usageCount > 0 {
		return errors.New("catalog asset is linked to one or more designs")
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM catalog_assets WHERE id = $1`, assetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("catalog asset not found")
	}
	return nil
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

func scanDesignDoc(row rowScanner) (domain.DesignDoc, error) {
	var doc domain.DesignDoc
	err := row.Scan(
		&doc.ID,
		&doc.WorkspaceID,
		&doc.DesignID,
		&doc.Title,
		&doc.Body,
		&doc.Format,
		&doc.CreatedBy,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	return doc, err
}

func scanUser(row rowScanner) (domain.User, error) {
	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.DisplayName,
		&user.Email,
		&user.PasswordSet,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.LastSeenAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func scanSignInConfig(row rowScanner) (domain.SignInConfig, error) {
	var config domain.SignInConfig
	err := row.Scan(
		&config.LocalPasswordEnabled,
		&config.SSOEnabled,
		&config.Provider,
		&config.OktaDomain,
		&config.Issuer,
		&config.ClientID,
		&config.ClientSecretSet,
		&config.RedirectURI,
		&config.PostLogoutRedirectURI,
		&config.Scopes,
		&config.GroupsClaim,
		&config.AdminGroup,
		&config.ReviewerGroup,
		&config.JITProvisioning,
		&config.UpdatedAt,
	)
	return config, err
}

func scanAIProviderConfig(row rowScanner) (domain.AIProviderConfig, error) {
	var config domain.AIProviderConfig
	var verifiedAt sql.NullTime
	err := row.Scan(
		&config.Enabled,
		&config.Provider,
		&config.Model,
		&config.BaseURL,
		&config.APIKey,
		&config.APIKeySet,
		&verifiedAt,
		&config.UpdatedAt,
	)
	if verifiedAt.Valid {
		config.VerifiedAt = verifiedAt.Time
	}
	return config, err
}

func scanWorkspaceAccess(row rowScanner) (domain.WorkspaceAccess, error) {
	var access domain.WorkspaceAccess
	err := row.Scan(
		&access.WorkspaceID,
		&access.UserID,
		&access.CanRead,
		&access.CanCreateDesign,
		&access.CanManage,
		&access.CreatedAt,
		&access.UpdatedAt,
	)
	return access, err
}

func scanDesignAccess(row rowScanner) (domain.DesignAccess, error) {
	var access domain.DesignAccess
	err := row.Scan(
		&access.WorkspaceID,
		&access.DesignID,
		&access.UserID,
		&access.CanRead,
		&access.CanEdit,
		&access.CanComment,
		&access.CanReview,
		&access.CanManage,
		&access.CreatedAt,
		&access.UpdatedAt,
	)
	return access, err
}

func scanMCPConfig(row rowScanner) (domain.MCPConfig, error) {
	var config domain.MCPConfig
	err := row.Scan(
		&config.Enabled,
		&config.EndpointPath,
		&config.ReadCatalog,
		&config.ReadDesigns,
		&config.CreateDraftDesign,
		&config.RunAnalysis,
		&config.FetchImpactReport,
		&config.RequireAdminConsent,
		&config.UpdatedAt,
	)
	return config, err
}

func scanDesignComment(row rowScanner) (domain.DesignComment, error) {
	var comment domain.DesignComment
	err := row.Scan(
		&comment.ID,
		&comment.WorkspaceID,
		&comment.DesignID,
		&comment.AuthorID,
		&comment.Body,
		&comment.ComponentID,
		&comment.ConnectorID,
		&comment.CreatedAt,
	)
	return comment, err
}

func scanDesignReviewRequest(row rowScanner) (domain.DesignReviewRequest, error) {
	var review domain.DesignReviewRequest
	err := row.Scan(
		&review.ID,
		&review.WorkspaceID,
		&review.DesignID,
		&review.RequestedBy,
		&review.ReviewerID,
		&review.Status,
		&review.Message,
		&review.Summary,
		&review.CreatedAt,
		&review.UpdatedAt,
		&review.CompletedAt,
	)
	return review, err
}

func scanNotification(row rowScanner) (domain.Notification, error) {
	var notification domain.Notification
	err := row.Scan(
		&notification.ID,
		&notification.UserID,
		&notification.WorkspaceID,
		&notification.DesignID,
		&notification.Type,
		&notification.Title,
		&notification.Body,
		&notification.Read,
		&notification.CreatedAt,
	)
	return notification, err
}

func scanCatalogAsset(row rowScanner) (domain.CatalogAsset, error) {
	var asset domain.CatalogAsset
	var tags string
	var metadata string
	err := row.Scan(
		&asset.ID,
		&asset.Name,
		&asset.NormalizedName,
		&asset.Type,
		&asset.Owner,
		&asset.Description,
		&asset.Criticality,
		&tags,
		&metadata,
		&asset.CreatedBy,
		&asset.CreatedAt,
		&asset.UpdatedAt,
		&asset.UsedInDesignCount,
	)
	if err != nil {
		return domain.CatalogAsset{}, err
	}
	asset.Tags = splitStoredTags(tags)
	asset.Metadata = json.RawMessage(metadata)
	return asset, nil
}

func splitStoredTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tags = append(tags, part)
		}
	}
	return tags
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

func ensureDesignExists(ctx context.Context, tx pgx.Tx, workspaceID string, designID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM designs WHERE workspace_id = $1 AND id = $2)`, workspaceID, designID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("design not found")
	}
	return nil
}

func createUserWithTx(ctx context.Context, tx pgx.Tx, now time.Time, displayName string, email string, role string, password string) (domain.User, error) {
	displayName = strings.TrimSpace(displayName)
	email = strings.ToLower(strings.TrimSpace(email))
	if displayName == "" {
		return domain.User{}, errors.New("display name is required")
	}
	if email == "" {
		return domain.User{}, errors.New("email is required")
	}
	passwordHash := ""
	if strings.TrimSpace(password) != "" {
		var err error
		passwordHash, err = hashPassword(password)
		if err != nil {
			return domain.User{}, err
		}
	}
	user := domain.User{
		ID:           fmt.Sprintf("user_%d", now.UnixNano()),
		DisplayName:  displayName,
		Email:        email,
		PasswordSet:  passwordHash != "",
		PasswordHash: passwordHash,
		Role:         normalizedUserRole(role),
		Status:       "active",
		LastSeenAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO users (id, display_name, email, password_hash, role, status, last_seen_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $7)
`, user.ID, user.DisplayName, user.Email, user.PasswordHash, user.Role, user.Status, now); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func claimLegacyGuestData(ctx context.Context, tx pgx.Tx, userID string) error {
	legacyIDs := []string{"", "guest-user"}
	for _, legacyID := range legacyIDs {
		if _, err := tx.Exec(ctx, `UPDATE designs SET created_by = $1 WHERE created_by = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE design_versions SET created_by = $1 WHERE created_by = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE design_docs SET created_by = $1 WHERE created_by = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE design_comments SET author_id = $1 WHERE author_id = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE design_review_requests SET requested_by = $1 WHERE requested_by = $2`, userID, legacyID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE notifications SET user_id = $1 WHERE user_id = $2`, userID, legacyID); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultSignInConfig(ctx context.Context, tx pgx.Tx, now time.Time) error {
	config := defaultSignInConfig(now)
	_, err := tx.Exec(ctx, `
INSERT INTO sign_in_settings (
	id, local_password_enabled, sso_enabled, provider, okta_domain, issuer, client_id, client_secret,
	redirect_uri, post_logout_redirect_uri, scopes, groups_claim, admin_group, reviewer_group, jit_provisioning, updated_at
)
VALUES ('default', $1, $2, $3, '', '', '', '', '', '', $4, $5, '', '', $6, $7)
ON CONFLICT (id) DO NOTHING
`, config.LocalPasswordEnabled, config.SSOEnabled, config.Provider, config.Scopes, config.GroupsClaim, config.JITProvisioning, config.UpdatedAt)
	return err
}

func ensureDefaultAIProviderConfig(ctx context.Context, tx pgx.Tx, now time.Time) error {
	config := defaultAIProviderConfig(now)
	_, err := tx.Exec(ctx, `
INSERT INTO ai_provider_settings (id, enabled, provider, model, base_url, api_key, verified_at, updated_at)
VALUES ('default', $1, $2, $3, $4, '', NULL, $5)
ON CONFLICT (id) DO NOTHING
`, config.Enabled, config.Provider, config.Model, config.BaseURL, config.UpdatedAt)
	return err
}

func ensureDefaultMCPConfig(ctx context.Context, tx pgx.Tx, now time.Time) error {
	config := defaultMCPConfig(now)
	_, err := tx.Exec(ctx, `
INSERT INTO mcp_settings (
	id, enabled, endpoint_path, read_catalog, read_designs, create_draft_design,
	run_analysis, fetch_impact_report, require_admin_consent, updated_at
)
VALUES ('default', $1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO NOTHING
`, config.Enabled, config.EndpointPath, config.ReadCatalog, config.ReadDesigns, config.CreateDraftDesign, config.RunAnalysis, config.FetchImpactReport, config.RequireAdminConsent, config.UpdatedAt)
	return err
}

func getSignInConfigWithTx(ctx context.Context, tx pgx.Tx) (domain.SignInConfig, error) {
	row := tx.QueryRow(ctx, `
SELECT local_password_enabled, sso_enabled, provider, okta_domain, issuer, client_id, client_secret <> '', redirect_uri,
	post_logout_redirect_uri, scopes, groups_claim, admin_group, reviewer_group, jit_provisioning, updated_at
FROM sign_in_settings
WHERE id = 'default'
`)
	return scanSignInConfig(row)
}

func (r *PostgresRepository) firstUserID(ctx context.Context) string {
	var userID string
	_ = r.pool.QueryRow(ctx, `
SELECT id
FROM users
WHERE status <> 'disabled'
ORDER BY created_at ASC
LIMIT 1
`).Scan(&userID)
	return userID
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func isUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && (constraintName == "" || pgErr.ConstraintName == constraintName)
}

func nullableJSONText(value json.RawMessage) *string {
	if len(value) == 0 {
		return nil
	}
	text := string(value)
	return &text
}

func nullableTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func notificationFor(userID string, workspaceID string, designID string, notificationType string, title string, body string, now time.Time) domain.Notification {
	return domain.Notification{
		ID:          fmt.Sprintf("notification_%d_%s", now.UnixNano(), userID),
		UserID:      userID,
		WorkspaceID: workspaceID,
		DesignID:    designID,
		Type:        notificationType,
		Title:       title,
		Body:        body,
		Read:        false,
		CreatedAt:   now,
	}
}

func insertNotification(ctx context.Context, tx pgx.Tx, notification domain.Notification) error {
	if strings.TrimSpace(notification.UserID) == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
INSERT INTO notifications (id, user_id, workspace_id, design_id, type, title, body, read, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, notification.ID, notification.UserID, notification.WorkspaceID, notification.DesignID, notification.Type, notification.Title, notification.Body, notification.Read, notification.CreatedAt)
	return err
}
