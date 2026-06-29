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
	status TEXT NOT NULL DEFAULT 'draft',
	remarks TEXT NOT NULL DEFAULT '',
	document TEXT NOT NULL,
	canvas_snapshot TEXT,
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE(design_id, version_number)
);

ALTER TABLE design_versions ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'draft';
ALTER TABLE design_versions ADD COLUMN IF NOT EXISTS remarks TEXT NOT NULL DEFAULT '';
ALTER TABLE design_versions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE INDEX IF NOT EXISTS idx_design_versions_design_version ON design_versions(design_id, version_number DESC);
CREATE INDEX IF NOT EXISTS idx_design_versions_status ON design_versions(design_id, status);

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

CREATE TABLE IF NOT EXISTS password_reset_tokens (
	token_hash TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens(user_id, expires_at DESC);

CREATE TABLE IF NOT EXISTS access_groups (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	okta_group_name TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_access_groups_okta ON access_groups(okta_group_name);

CREATE TABLE IF NOT EXISTS access_group_members (
	group_id TEXT NOT NULL REFERENCES access_groups(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	added_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (group_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_access_group_members_user ON access_group_members(user_id);

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

CREATE TABLE IF NOT EXISTS workspace_group_access (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	group_id TEXT NOT NULL REFERENCES access_groups(id) ON DELETE CASCADE,
	can_read BOOLEAN NOT NULL DEFAULT FALSE,
	can_create_design BOOLEAN NOT NULL DEFAULT FALSE,
	can_manage BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (workspace_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_workspace_group_access_group ON workspace_group_access(group_id);

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

CREATE TABLE IF NOT EXISTS design_group_access (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	design_id TEXT NOT NULL REFERENCES designs(id) ON DELETE CASCADE,
	group_id TEXT NOT NULL REFERENCES access_groups(id) ON DELETE CASCADE,
	can_read BOOLEAN NOT NULL DEFAULT FALSE,
	can_edit BOOLEAN NOT NULL DEFAULT FALSE,
	can_comment BOOLEAN NOT NULL DEFAULT FALSE,
	can_review BOOLEAN NOT NULL DEFAULT FALSE,
	can_manage BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (workspace_id, design_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_design_group_access_group ON design_group_access(group_id);

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

CREATE TABLE IF NOT EXISTS telemetry_integration_settings (
	id TEXT PRIMARY KEY,
	enabled BOOLEAN NOT NULL,
	provider TEXT NOT NULL,
	display_name TEXT NOT NULL,
	base_url TEXT NOT NULL DEFAULT '',
	auth_mode TEXT NOT NULL DEFAULT 'none',
	secret TEXT NOT NULL DEFAULT '',
	custom_header_name TEXT NOT NULL DEFAULT '',
	query_window TEXT NOT NULL DEFAULT '5m',
	request_total_metric TEXT NOT NULL DEFAULT '',
	request_failed_metric TEXT NOT NULL DEFAULT '',
	server_latency_metric TEXT NOT NULL DEFAULT '',
	client_latency_metric TEXT NOT NULL DEFAULT '',
	filters TEXT NOT NULL DEFAULT '{}',
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
	version_id TEXT NOT NULL DEFAULT '',
	version_number INTEGER NOT NULL DEFAULT 0,
	requested_by TEXT NOT NULL,
	reviewer_id TEXT NOT NULL,
	status TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	completed_at TIMESTAMPTZ
);

ALTER TABLE design_review_requests ADD COLUMN IF NOT EXISTS version_id TEXT NOT NULL DEFAULT '';
ALTER TABLE design_review_requests ADD COLUMN IF NOT EXISTS version_number INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_design_review_requests_design_updated ON design_review_requests(workspace_id, design_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_design_review_requests_reviewer_updated ON design_review_requests(reviewer_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_design_review_requests_version ON design_review_requests(workspace_id, design_id, version_id);

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
