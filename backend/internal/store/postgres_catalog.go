package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
)

func (r *PostgresRepository) ListCatalogAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.CatalogAsset, PageInfo, error) {
		options.Query = query
		return r.ListCatalogAssetsPage(ctx, options)
	})
}

func (r *PostgresRepository) ListCatalogAssetsPage(ctx context.Context, options PageOptions) ([]domain.CatalogAsset, PageInfo, error) {
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	normalizedQuery := normalizedCatalogName(options.Query)
	likeQuery := "%" + normalizedQuery + "%"
	rows, err := r.pool.Query(ctx, `
SELECT
	ca.id, ca.name, ca.normalized_name, ca.kind, ca.type, ca.owner, ca.description, ca.criticality,
	ca.status, ca.aliases, ca.replacement_asset_id, ca.update_message, ca.tags, ca.metadata,
	ca.created_by, ca.created_at, ca.updated_at,
	COUNT(DISTINCT d.id)::INT AS used_in_design_count
FROM catalog_assets ca
LEFT JOIN designs d ON d.document LIKE '%"assetId":"' || ca.id || '"%'
WHERE $1 = '' OR ca.normalized_name LIKE $2
GROUP BY ca.id
ORDER BY (ca.normalized_name = $1) DESC, ca.name ASC
LIMIT $3 OFFSET $4
`, normalizedQuery, likeQuery, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()

	assets := []domain.CatalogAsset{}
	for rows.Next() {
		asset, err := scanCatalogAsset(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(assets, options, offset)
	return items, page, nil
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
	asset.Kind = normalizedCatalogAssetKind(asset.Kind)
	if strings.TrimSpace(asset.Criticality) == "" {
		asset.Criticality = "medium"
	}
	asset.Status = normalizedCatalogAssetStatus(asset.Status)
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
INSERT INTO catalog_assets (
	id, name, normalized_name, kind, type, owner, description, criticality, status, aliases,
	replacement_asset_id, update_message, tags, metadata, created_by, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $16)
RETURNING
	id, name, normalized_name, kind, type, owner, description, criticality, status, aliases,
	replacement_asset_id, update_message, tags, metadata, created_by, created_at, updated_at, 0
`, asset.ID, asset.Name, asset.NormalizedName, asset.Kind, strings.TrimSpace(asset.Type), strings.TrimSpace(asset.Owner), strings.TrimSpace(asset.Description), strings.TrimSpace(asset.Criticality), asset.Status, strings.Join(normalizedTags(asset.Aliases), ","), strings.TrimSpace(asset.ReplacementAssetID), strings.TrimSpace(asset.UpdateMessage), strings.Join(normalizedTags(asset.Tags), ","), string(asset.Metadata), asset.CreatedBy, now)
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
	asset.Kind = normalizedCatalogAssetKind(asset.Kind)
	if strings.TrimSpace(asset.Criticality) == "" {
		asset.Criticality = "medium"
	}
	asset.Status = normalizedCatalogAssetStatus(asset.Status)
	metadata := asset.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	row := r.pool.QueryRow(ctx, `
WITH updated AS (
	UPDATE catalog_assets
		SET name = $2,
		normalized_name = $3,
		kind = $4,
		type = $5,
		owner = $6,
		description = $7,
		criticality = $8,
		status = $9,
		aliases = $10,
		replacement_asset_id = $11,
		update_message = $12,
		tags = $13,
		metadata = $14,
		updated_at = $15
	WHERE id = $1
	RETURNING id, name, normalized_name, kind, type, owner, description, criticality, status, aliases,
		replacement_asset_id, update_message, tags, metadata, created_by, created_at, updated_at
)
SELECT updated.*, COUNT(DISTINCT d.id)::INT AS used_in_design_count
FROM updated
LEFT JOIN designs d ON d.document LIKE '%"assetId":"' || updated.id || '"%'
GROUP BY updated.id, updated.name, updated.normalized_name, updated.kind, updated.type, updated.owner,
	updated.description, updated.criticality, updated.status, updated.aliases, updated.replacement_asset_id,
	updated.update_message, updated.tags, updated.metadata, updated.created_by, updated.created_at, updated.updated_at
`, assetID, asset.Name, normalized, asset.Kind, strings.TrimSpace(asset.Type), strings.TrimSpace(asset.Owner), strings.TrimSpace(asset.Description), strings.TrimSpace(asset.Criticality), asset.Status, strings.Join(normalizedTags(asset.Aliases), ","), strings.TrimSpace(asset.ReplacementAssetID), strings.TrimSpace(asset.UpdateMessage), strings.Join(normalizedTags(asset.Tags), ","), string(metadata), r.clock().UTC())
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
