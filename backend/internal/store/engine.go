package store

import (
	"context"
	"errors"
	"strings"
	"sync"
)

const (
	StorageModeStateless = "stateless"
	StorageModeDatabase  = "database"
)

type StorageStatus struct {
	Mode                 string `json:"mode"`
	Stateless            bool   `json:"stateless"`
	DatabaseConfigured   bool   `json:"databaseConfigured"`
	DatabaseConnected    bool   `json:"databaseConnected"`
	DatabasePending      bool   `json:"databasePending"`
	CurrentDatabaseURL   string `json:"currentDatabaseUrl,omitempty"`
	PendingDatabaseURL   string `json:"pendingDatabaseUrl,omitempty"`
	CanMigrateUsers      bool   `json:"canMigrateUsers"`
	CacheUserCount       int    `json:"cacheUserCount"`
	DatabaseUserCount    int    `json:"databaseUserCount"`
	Warning              string `json:"warning,omitempty"`
	PersistentConfigHint string `json:"persistentConfigHint,omitempty"`
}

type StorageEngine struct {
	mu sync.RWMutex

	cache        *MemoryRepository
	active       Repository
	activeCloser func()
	mode         string

	databaseURL string
	pendingDB   *PostgresRepository
	pendingURL  string
	postgres    PostgresOptions
}

func NewStorageEngine(ctx context.Context, databaseURL string) (*StorageEngine, error) {
	return NewStorageEngineWithOptions(ctx, databaseURL, DefaultPostgresOptions())
}

func NewStorageEngineWithOptions(ctx context.Context, databaseURL string, postgresOptions PostgresOptions) (*StorageEngine, error) {
	cache := NewMemoryRepository()
	engine := &StorageEngine{
		cache:        cache,
		active:       cache,
		activeCloser: func() {},
		mode:         StorageModeStateless,
		postgres:     postgresOptions,
	}
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return engine, nil
	}
	repo, err := NewPostgresRepositoryWithOptions(ctx, databaseURL, postgresOptions)
	if err != nil {
		return nil, err
	}
	engine.active = repo
	engine.activeCloser = repo.Close
	engine.mode = StorageModeDatabase
	engine.databaseURL = databaseURL
	return engine, nil
}

func (e *StorageEngine) Repository() Repository {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.active
}

func (e *StorageEngine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeCloser != nil {
		e.activeCloser()
	}
	if e.pendingDB != nil {
		e.pendingDB.Close()
	}
}

func (e *StorageEngine) Status(ctx context.Context) StorageStatus {
	e.mu.RLock()
	mode := e.mode
	active := e.active
	cache := e.cache
	pending := e.pendingDB
	activeURL := e.databaseURL
	pendingURL := e.pendingURL
	databaseConfigured := e.databaseURL != "" || e.pendingURL != ""
	databaseConnected := false
	if mode == StorageModeDatabase {
		if postgres, ok := active.(*PostgresRepository); ok {
			databaseConnected = postgres.Ping(ctx) == nil
		}
	} else if pending != nil {
		databaseConnected = pending.Ping(ctx) == nil
	}
	e.mu.RUnlock()

	status := StorageStatus{
		Mode:                 mode,
		Stateless:            mode == StorageModeStateless,
		DatabaseConfigured:   databaseConfigured,
		DatabaseConnected:    databaseConnected,
		DatabasePending:      mode == StorageModeStateless && pending != nil,
		CurrentDatabaseURL:   redactDatabaseURL(activeURL),
		PendingDatabaseURL:   redactDatabaseURL(pendingURL),
		PersistentConfigHint: "Set DATABASE_URL in the deployment environment to keep database configuration across restarts.",
	}
	if status.Stateless {
		status.Warning = "Stratum is running in stateless mode. Data is stored in process cache and will be lost when the backend restarts."
	}
	if users, err := cache.ListUsers(ctx); err == nil {
		status.CacheUserCount = len(users)
	}
	if mode == StorageModeDatabase {
		if users, err := active.ListUsers(ctx); err == nil {
			status.DatabaseUserCount = len(users)
		}
	} else if pending != nil {
		if users, err := pending.ListUsers(ctx); err == nil {
			status.DatabaseUserCount = len(users)
		}
	}
	status.CanMigrateUsers = status.Stateless && status.DatabasePending && status.CacheUserCount > 0 && status.DatabaseUserCount == 0
	return status
}

func redactDatabaseURL(databaseURL string) string {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return ""
	}
	parts := strings.Split(databaseURL, "@")
	if len(parts) < 2 {
		return databaseURL
	}
	prefix := parts[0]
	schemeParts := strings.SplitN(prefix, "://", 2)
	if len(schemeParts) != 2 {
		return databaseURL
	}
	credential := schemeParts[1]
	if !strings.Contains(credential, ":") {
		return databaseURL
	}
	return schemeParts[0] + "://" + strings.SplitN(credential, ":", 2)[0] + ":****@" + strings.Join(parts[1:], "@")
}

func (e *StorageEngine) TestDatabase(ctx context.Context, databaseURL string) error {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return errors.New("database url is required")
	}
	return TestPostgresConnection(ctx, databaseURL, e.postgres)
}

func (e *StorageEngine) ConfigureDatabase(ctx context.Context, databaseURL string) (StorageStatus, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return StorageStatus{}, errors.New("database url is required")
	}
	repo, err := NewPostgresRepositoryWithOptions(ctx, databaseURL, e.postgres)
	if err != nil {
		return StorageStatus{}, err
	}

	e.mu.Lock()
	if e.pendingDB != nil {
		e.pendingDB.Close()
	}
	if e.mode == StorageModeDatabase {
		oldCloser := e.activeCloser
		e.active = repo
		e.activeCloser = repo.Close
		e.databaseURL = databaseURL
		e.mu.Unlock()
		if oldCloser != nil {
			oldCloser()
		}
		return e.Status(ctx), nil
	}
	e.pendingDB = repo
	e.pendingURL = databaseURL
	e.mu.Unlock()
	return e.Status(ctx), nil
}

func (e *StorageEngine) MigrateUsersToDatabase(ctx context.Context) (StorageStatus, int, error) {
	e.mu.RLock()
	pending := e.pendingDB
	cache := e.cache
	pendingURL := e.pendingURL
	mode := e.mode
	e.mu.RUnlock()

	if mode == StorageModeDatabase {
		return e.Status(ctx), 0, errors.New("storage is already using database mode")
	}
	if pending == nil {
		return e.Status(ctx), 0, errors.New("configure and test a database before migrating users")
	}
	existingUsers, err := pending.ListUsers(ctx)
	if err != nil {
		return e.Status(ctx), 0, err
	}
	if len(existingUsers) > 0 {
		return e.Status(ctx), 0, errors.New("database already has users; user migration only supports an empty database")
	}
	users, err := cache.ListUsers(ctx)
	if err != nil {
		return e.Status(ctx), 0, err
	}
	if len(users) == 0 {
		return e.Status(ctx), 0, errors.New("no cache users are available to migrate")
	}
	migrated, err := pending.ImportUsers(ctx, users)
	if err != nil {
		return e.Status(ctx), 0, err
	}

	e.mu.Lock()
	e.active = pending
	e.activeCloser = pending.Close
	e.mode = StorageModeDatabase
	e.databaseURL = pendingURL
	e.pendingDB = nil
	e.pendingURL = ""
	e.mu.Unlock()

	return e.Status(ctx), migrated, nil
}
