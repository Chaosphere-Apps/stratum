package app

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

type VersionService struct {
	provider RepositoryProvider
}

func (s VersionService) repo() store.Repository { return s.provider.Repository() }

func (s VersionService) List(ctx context.Context, workspaceID string, designID string) ([]domain.DesignVersion, error) {
	return s.repo().ListDesignVersions(ctx, workspaceID, designID)
}

func (s VersionService) ListPage(ctx context.Context, workspaceID string, designID string, options store.PageOptions) ([]domain.DesignVersion, store.PageInfo, error) {
	return s.repo().ListDesignVersionsPage(ctx, workspaceID, designID, options)
}

func (s VersionService) Create(ctx context.Context, workspaceID string, designID string, createdBy string, remarks string) (domain.DesignVersion, error) {
	return s.repo().CreateDesignVersion(ctx, workspaceID, designID, createdBy, remarks)
}

func (s VersionService) UpdateDraft(ctx context.Context, workspaceID string, designID string, versionID string, document []byte, canvasSnapshot []byte, remarks string) (domain.DesignVersion, error) {
	return s.repo().UpdateDraftDesignVersion(ctx, workspaceID, designID, versionID, document, canvasSnapshot, remarks)
}

func (s VersionService) UpdateStatus(ctx context.Context, workspaceID string, designID string, versionID string, status string) (domain.DesignVersion, error) {
	return s.repo().UpdateDesignVersionStatus(ctx, workspaceID, designID, versionID, status)
}

func (s VersionService) Delete(ctx context.Context, workspaceID string, designID string, versionID string) error {
	return s.repo().DeleteDesignVersion(ctx, workspaceID, designID, versionID)
}
