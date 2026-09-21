package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"sort"
	"strings"
)

func (r *MemoryRepository) ListCatalogAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.CatalogAsset, PageInfo, error) {
		options.Query = query
		return r.ListCatalogAssetsPage(ctx, options)
	})
}

func (r *MemoryRepository) ListCatalogAssetsPage(ctx context.Context, options PageOptions) ([]domain.CatalogAsset, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	normalizedQuery := normalizedCatalogName(options.Query)
	r.mu.RLock()
	defer r.mu.RUnlock()

	assets := make([]domain.CatalogAsset, 0, len(r.catalogAssets))
	for _, asset := range r.catalogAssets {
		if normalizedQuery != "" && !strings.Contains(asset.NormalizedName, normalizedQuery) && !strings.Contains(strings.ToLower(asset.Name), strings.ToLower(strings.TrimSpace(options.Query))) {
			continue
		}
		asset.UsedInDesignCount = r.catalogAssetUsageCountLocked(asset.ID)
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool {
		if normalizedQuery != "" {
			leftExact := assets[i].NormalizedName == normalizedQuery
			rightExact := assets[j].NormalizedName == normalizedQuery
			if leftExact != rightExact {
				return leftExact
			}
		}
		return assets[i].Name < assets[j].Name
	})
	page, info := PageFromSlice(assets, options)
	return page, info, nil
}

func (r *MemoryRepository) CreateCatalogAsset(ctx context.Context, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	if err := ctx.Err(); err != nil {
		return domain.CatalogAsset{}, err
	}
	asset.Name = strings.TrimSpace(asset.Name)
	if asset.Name == "" {
		return domain.CatalogAsset{}, errors.New("catalog asset name is required")
	}
	asset.NormalizedName = normalizedCatalogName(asset.Name)
	asset.Kind = normalizedCatalogAssetKind(asset.Kind)
	asset.Status = normalizedCatalogAssetStatus(asset.Status)
	asset.Aliases = normalizedStringList(asset.Aliases)
	asset.ReplacementAssetID = strings.TrimSpace(asset.ReplacementAssetID)
	asset.UpdateMessage = strings.TrimSpace(asset.UpdateMessage)
	if asset.Type == "" {
		asset.Type = "compute.service"
	}
	if asset.Criticality == "" {
		asset.Criticality = "medium"
	}
	if len(asset.Metadata) == 0 {
		asset.Metadata = json.RawMessage(`{}`)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.catalogAssets {
		if existing.NormalizedName == asset.NormalizedName {
			return domain.CatalogAsset{}, errors.New("catalog asset already exists")
		}
	}
	now := r.clock().UTC()
	asset.ID = fmt.Sprintf("asset_%d", now.UnixNano())
	if strings.TrimSpace(asset.CreatedBy) == "" {
		asset.CreatedBy = r.firstUserIDLocked()
	}
	asset.CreatedAt = now
	asset.UpdatedAt = now
	r.catalogAssets[asset.ID] = asset
	return asset, nil
}

func (r *MemoryRepository) UpdateCatalogAsset(ctx context.Context, assetID string, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	if err := ctx.Err(); err != nil {
		return domain.CatalogAsset{}, err
	}
	asset.Name = strings.TrimSpace(asset.Name)
	if asset.Name == "" {
		return domain.CatalogAsset{}, errors.New("catalog asset name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.catalogAssets[assetID]
	if !ok {
		return domain.CatalogAsset{}, errors.New("catalog asset not found")
	}
	normalized := normalizedCatalogName(asset.Name)
	for _, other := range r.catalogAssets {
		if other.ID != assetID && other.NormalizedName == normalized {
			return domain.CatalogAsset{}, errors.New("catalog asset already exists")
		}
	}
	existing.Name = asset.Name
	existing.NormalizedName = normalized
	existing.Type = strings.TrimSpace(asset.Type)
	existing.Kind = normalizedCatalogAssetKind(asset.Kind)
	existing.Status = normalizedCatalogAssetStatus(asset.Status)
	existing.Aliases = normalizedStringList(asset.Aliases)
	existing.ReplacementAssetID = strings.TrimSpace(asset.ReplacementAssetID)
	existing.UpdateMessage = strings.TrimSpace(asset.UpdateMessage)
	if existing.Type == "" {
		existing.Type = "compute.service"
	}
	existing.Owner = strings.TrimSpace(asset.Owner)
	existing.Description = strings.TrimSpace(asset.Description)
	existing.Criticality = strings.TrimSpace(asset.Criticality)
	if existing.Criticality == "" {
		existing.Criticality = "medium"
	}
	existing.Tags = normalizedTags(asset.Tags)
	if len(asset.Metadata) > 0 {
		existing.Metadata = asset.Metadata
	}
	existing.UpdatedAt = r.clock().UTC()
	r.catalogAssets[assetID] = existing
	existing.UsedInDesignCount = r.catalogAssetUsageCountLocked(assetID)
	return existing, nil
}

func (r *MemoryRepository) DeleteCatalogAsset(ctx context.Context, assetID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.catalogAssets[assetID]; !ok {
		return errors.New("catalog asset not found")
	}
	if r.catalogAssetUsageCountLocked(assetID) > 0 {
		return errors.New("catalog asset is linked to one or more designs")
	}
	delete(r.catalogAssets, assetID)
	return nil
}

func normalizedCatalogName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(name)), " "))
}

func normalizedCatalogAssetKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "system":
		return "system"
	default:
		return "component"
	}
}

func normalizedCatalogAssetStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "deprecated":
		return "deprecated"
	case "retired":
		return "retired"
	default:
		return "active"
	}
}

func normalizedTags(tags []string) []string {
	seen := map[string]struct{}{}
	normalized := []string{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}

func (r *MemoryRepository) catalogAssetUsageCountLocked(assetID string) int {
	designIDs := map[string]struct{}{}
	needle := fmt.Sprintf(`"assetId":"%s"`, assetID)
	for _, design := range r.designs {
		if strings.Contains(string(design.Document), needle) {
			designIDs[design.ID] = struct{}{}
		}
	}
	return len(designIDs)
}
