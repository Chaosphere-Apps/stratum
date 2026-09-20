package app

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/store"
)

type RepositoryProvider interface {
	Repository() store.Repository
}

type StorageBackend interface {
	Repository() store.Repository
	Status(ctx context.Context) store.StorageStatus
	TestDatabase(ctx context.Context, databaseURL string) error
	ConfigureDatabase(ctx context.Context, databaseURL string) (store.StorageStatus, error)
	MigrateUsersToDatabase(ctx context.Context) (store.StorageStatus, int, error)
}

type StaticRepositoryProvider struct {
	Repo store.Repository
}

func (p StaticRepositoryProvider) Repository() store.Repository {
	return p.Repo
}

type Services struct {
	Identity   IdentityService
	Workspaces WorkspaceService
	Designs    DesignService
	ACL        ACLService
	Catalog    CatalogService
	Analysis   AnalysisService
	AIChat     AIChatService
	Storage    StorageAdminService
	Docs       DocsService
	Reviews    ReviewService
	Versions   VersionService
}

func NewServices(provider RepositoryProvider, storage StorageBackend) Services {
	return Services{
		Identity:   IdentityService{provider: provider},
		Workspaces: WorkspaceService{provider: provider},
		Designs:    DesignService{provider: provider},
		ACL:        ACLService{provider: provider},
		Catalog:    CatalogService{provider: provider},
		Analysis:   AnalysisService{provider: provider, analyzer: analysis.New()},
		AIChat:     AIChatService{provider: provider},
		Storage:    StorageAdminService{storage: storage},
		Docs:       DocsService{provider: provider},
		Reviews:    ReviewService{provider: provider},
		Versions:   VersionService{provider: provider},
	}
}
