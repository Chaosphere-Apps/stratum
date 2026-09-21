package app

import (
	"context"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
	"strings"
	"unicode/utf8"
)

type DesignService struct {
	provider RepositoryProvider
}

func (s DesignService) repo() store.Repository { return s.provider.Repository() }

func (s DesignService) List(ctx context.Context, workspaceID string) ([]domain.Design, error) {
	return s.repo().ListDesigns(ctx, workspaceID)
}

func (s DesignService) ListPage(ctx context.Context, workspaceID string, options store.PageOptions) ([]domain.Design, store.PageInfo, error) {
	return s.repo().ListDesignsPage(ctx, workspaceID, options)
}

func (s DesignService) ListAccessiblePage(ctx context.Context, workspaceID string, user domain.User, options store.PageOptions) ([]domain.Design, store.PageInfo, error) {
	groupIDs, err := s.repo().ListUserAccessGroupIDs(ctx, user.ID)
	if err != nil {
		return nil, store.PageInfo{}, err
	}
	return s.repo().ListAccessibleDesignsPage(ctx, workspaceID, store.AccessScope{UserID: user.ID, IsAdmin: user.Role == "admin", GroupIDs: groupIDs}, options)
}

func (s DesignService) Get(ctx context.Context, workspaceID string, designID string) (domain.Design, error) {
	return s.repo().GetDesign(ctx, workspaceID, designID)
}

func (s DesignService) Create(ctx context.Context, workspaceID string, name string, document []byte, createdBy string) (domain.Design, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Design{}, fmt.Errorf("design name is required")
	}
	if utf8.RuneCountInString(name) > domain.MaxDesignNameLength {
		return domain.Design{}, fmt.Errorf("design name must be %d characters or fewer", domain.MaxDesignNameLength)
	}
	return s.repo().CreateDesign(ctx, workspaceID, name, document, createdBy)
}

func (s DesignService) UpdateMetadata(ctx context.Context, workspaceID string, designID string, name string, access string) (domain.Design, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > domain.MaxDesignNameLength {
		return domain.Design{}, fmt.Errorf("design name must be %d characters or fewer", domain.MaxDesignNameLength)
	}
	return s.repo().UpdateDesignMetadata(ctx, workspaceID, designID, name, access)
}

func (s DesignService) Upsert(ctx context.Context, design domain.Design) (domain.Design, error) {
	return s.repo().UpsertDesign(ctx, design)
}

func (s DesignService) UpdateDocument(ctx context.Context, workspaceID string, designID string, document []byte, canvasSnapshot []byte, expectedRevision string) (domain.Design, error) {
	return s.repo().UpdateDesignDocument(ctx, workspaceID, designID, document, canvasSnapshot, expectedRevision)
}

func (s DesignService) Delete(ctx context.Context, workspaceID string, designID string) error {
	return s.repo().DeleteDesign(ctx, workspaceID, designID)
}
