package store

import (
	"context"
	"strings"

	"github.com/system-design-evaluator/backend/internal/domain"
)

type WorkspaceRepository interface {
	GetOrCreateGuestWorkspace(ctx context.Context) (domain.Workspace, error)
	ListWorkspaces(ctx context.Context) ([]domain.Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error)
	CreateWorkspace(ctx context.Context, name string) (domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
}

type DesignRepository interface {
	ListDesigns(ctx context.Context, workspaceID string) ([]domain.Design, error)
	GetDesign(ctx context.Context, workspaceID string, designID string) (domain.Design, error)
	CreateDesign(ctx context.Context, workspaceID string, name string, document []byte) (domain.Design, error)
	UpdateDesignMetadata(ctx context.Context, workspaceID string, designID string, name string, access string) (domain.Design, error)
	UpsertDesign(ctx context.Context, design domain.Design) (domain.Design, error)
	DeleteDesign(ctx context.Context, workspaceID string, designID string) error
	ListDesignVersions(ctx context.Context, workspaceID string, designID string) ([]domain.DesignVersion, error)
}

type DesignDocRepository interface {
	ListDesignDocs(ctx context.Context, workspaceID string, designID string) ([]domain.DesignDoc, error)
	GetDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) (domain.DesignDoc, error)
	CreateDesignDoc(ctx context.Context, workspaceID string, designID string, title string, body string, format string) (domain.DesignDoc, error)
	UpdateDesignDoc(ctx context.Context, workspaceID string, designID string, docID string, title string, body string, format string) (domain.DesignDoc, error)
	DeleteDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) error
}

type Repository interface {
	WorkspaceRepository
	DesignRepository
	DesignDocRepository
}

func NewRepository(ctx context.Context, databaseURL string) (Repository, func(), error) {
	if strings.TrimSpace(databaseURL) == "" {
		return NewMemoryRepository(), func() {}, nil
	}

	repo, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		return nil, func() {}, err
	}
	return repo, repo.Close, nil
}
