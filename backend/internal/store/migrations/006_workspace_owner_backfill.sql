-- Existing installations may have applied the ownership migration before the
-- built-in guest workspace was included. Assign every remaining orphaned
-- workspace to the first active administrator so authorization is fail-closed.
UPDATE workspaces
SET owner_id = (
    SELECT id
    FROM users
    WHERE role = 'admin' AND status = 'active'
    ORDER BY created_at ASC, id ASC
    LIMIT 1
)
WHERE owner_id IS NULL
  AND EXISTS (
      SELECT 1
      FROM users
      WHERE role = 'admin' AND status = 'active'
  );
