package app

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

type CatalogService struct {
	provider RepositoryProvider
}

func (s CatalogService) repo() store.Repository { return s.provider.Repository() }

func (s CatalogService) ListAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error) {
	return s.repo().ListCatalogAssets(ctx, query)
}

func (s CatalogService) ListAssetsPage(ctx context.Context, options store.PageOptions) ([]domain.CatalogAsset, store.PageInfo, error) {
	return s.repo().ListCatalogAssetsPage(ctx, options)
}

func (s CatalogService) CreateAsset(ctx context.Context, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	return s.repo().CreateCatalogAsset(ctx, asset)
}

func (s CatalogService) UpdateAsset(ctx context.Context, assetID string, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	return s.repo().UpdateCatalogAsset(ctx, assetID, asset)
}

func (s CatalogService) DeleteAsset(ctx context.Context, assetID string) error {
	return s.repo().DeleteCatalogAsset(ctx, assetID)
}
