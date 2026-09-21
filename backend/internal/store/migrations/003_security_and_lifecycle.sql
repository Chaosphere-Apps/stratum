ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS owner_id TEXT;

UPDATE workspaces AS workspace
SET owner_id = (
    SELECT access.user_id
    FROM workspace_access AS access
    WHERE access.workspace_id = workspace.id AND access.can_manage = TRUE
    ORDER BY access.created_at ASC
    LIMIT 1
)
WHERE workspace.owner_id IS NULL;

UPDATE workspaces
SET owner_id = (
    SELECT id FROM users WHERE role = 'admin' ORDER BY created_at ASC LIMIT 1
)
WHERE owner_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_workspaces_owner_updated
    ON workspaces(owner_id, updated_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_design_versions_one_live
    ON design_versions(design_id)
    WHERE status = 'live';

DELETE FROM design_review_requests AS duplicate
USING design_review_requests AS keeper
WHERE duplicate.workspace_id = keeper.workspace_id
  AND duplicate.design_id = keeper.design_id
  AND duplicate.version_id = keeper.version_id
  AND duplicate.reviewer_id = keeper.reviewer_id
  AND duplicate.created_at > keeper.created_at;

CREATE UNIQUE INDEX IF NOT EXISTS idx_design_review_requests_unique_reviewer
    ON design_review_requests(workspace_id, design_id, version_id, reviewer_id);
