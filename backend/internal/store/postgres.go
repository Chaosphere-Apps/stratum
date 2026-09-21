package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOptions struct {
	AutoMigrate       bool
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	Migration         MigrationOptions
}

func DefaultPostgresOptions() PostgresOptions {
	return PostgresOptions{
		AutoMigrate:       true,
		MaxConns:          20,
		MinConns:          2,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		Migration:         DefaultMigrationOptions(),
	}
}

type PostgresRepository struct {
	pool  *pgxpool.Pool
	clock func() time.Time
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	return NewPostgresRepositoryWithOptions(ctx, databaseURL, DefaultPostgresOptions())
}

func NewPostgresRepositoryWithOptions(ctx context.Context, databaseURL string, options PostgresOptions) (*PostgresRepository, error) {
	pool, err := openPostgresPool(ctx, databaseURL, options)
	if err != nil {
		return nil, err
	}
	repo := &PostgresRepository{pool: pool, clock: time.Now}
	if options.AutoMigrate {
		err = ApplyPostgresMigrations(ctx, pool, options.Migration)
	} else {
		var status MigrationStatus
		status, err = InspectPostgresMigrations(ctx, pool)
		if err == nil && len(status.Pending) > 0 {
			err = errors.New("database schema has pending migrations; run the Stratum migrate command before starting the server")
		} else if err == nil && len(status.Unknown) > 0 {
			err = errors.New("database schema is newer than this Stratum binary; deploy a compatible release")
		}
	}
	if err != nil {
		pool.Close()
		return nil, err
	}
	return repo, nil
}

func openPostgresPool(ctx context.Context, databaseURL string, options PostgresOptions) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	options = normalizePostgresOptions(options)
	config.MaxConns = options.MaxConns
	config.MinConns = options.MinConns
	config.MaxConnLifetime = options.MaxConnLifetime
	config.MaxConnIdleTime = options.MaxConnIdleTime
	config.HealthCheckPeriod = options.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func normalizePostgresOptions(options PostgresOptions) PostgresOptions {
	defaults := DefaultPostgresOptions()
	if options.MaxConns <= 0 {
		options.MaxConns = defaults.MaxConns
	}
	if options.MinConns < 0 {
		options.MinConns = 0
	}
	if options.MinConns > options.MaxConns {
		options.MinConns = options.MaxConns
	}
	if options.MaxConnLifetime <= 0 {
		options.MaxConnLifetime = defaults.MaxConnLifetime
	}
	if options.MaxConnIdleTime <= 0 {
		options.MaxConnIdleTime = defaults.MaxConnIdleTime
	}
	if options.HealthCheckPeriod <= 0 {
		options.HealthCheckPeriod = defaults.HealthCheckPeriod
	}
	options.Migration = normalizeMigrationOptions(options.Migration)
	return options
}

func TestPostgresConnection(ctx context.Context, databaseURL string, options PostgresOptions) error {
	pool, err := openPostgresPool(ctx, databaseURL, options)
	if err != nil {
		return err
	}
	pool.Close()
	return nil
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *PostgresRepository) migrate(ctx context.Context) error {
	return ApplyPostgresMigrations(ctx, r.pool, DefaultMigrationOptions())
}
