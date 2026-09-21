package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/store"
)

type AnalysisService struct {
	provider RepositoryProvider
	analyzer analysis.Analyzer
}

func (s AnalysisService) AnalyzeDocument(raw json.RawMessage) (analysis.Report, error) {
	return s.analyzer.Analyze(raw)
}

type StorageAdminService struct {
	storage StorageBackend
}

func (s StorageAdminService) Configurable() bool {
	return s.storage != nil
}

func (s StorageAdminService) Status(ctx context.Context) store.StorageStatus {
	if s.storage == nil {
		return store.StorageStatus{Mode: store.StorageModeDatabase, DatabaseConnected: true}
	}
	return s.storage.Status(ctx)
}

func (s StorageAdminService) TestDatabase(ctx context.Context, databaseURL string) error {
	if s.storage == nil {
		return errors.New("storage engine is not configurable")
	}
	return s.storage.TestDatabase(ctx, databaseURL)
}

func (s StorageAdminService) ConfigureDatabase(ctx context.Context, databaseURL string) (store.StorageStatus, error) {
	if s.storage == nil {
		return store.StorageStatus{}, errors.New("storage engine is not configurable")
	}
	return s.storage.ConfigureDatabase(ctx, databaseURL)
}

func (s StorageAdminService) MigrateUsersToDatabase(ctx context.Context) (store.StorageStatus, int, error) {
	if s.storage == nil {
		return store.StorageStatus{}, 0, errors.New("storage engine is not configurable")
	}
	return s.storage.MigrateUsersToDatabase(ctx)
}
