ALTER TABLE catalog_assets ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'component';
ALTER TABLE catalog_assets ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE catalog_assets ADD COLUMN IF NOT EXISTS aliases TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_assets ADD COLUMN IF NOT EXISTS replacement_asset_id TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_assets ADD COLUMN IF NOT EXISTS update_message TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_catalog_assets_kind_status ON catalog_assets(kind, status);
