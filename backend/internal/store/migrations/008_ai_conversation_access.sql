ALTER TABLE ai_conversations
    ADD COLUMN IF NOT EXISTS access_mode TEXT NOT NULL DEFAULT 'read'
    CHECK (access_mode IN ('read', 'read_write'));
