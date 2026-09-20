package store

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var postgresMigrations embed.FS

const postgresMigrationLockKey int64 = 0x5354524154554D

type MigrationOptions struct {
	LockTimeout      time.Duration
	StatementTimeout time.Duration
}

type MigrationRecord struct {
	Version   string
	Checksum  string
	AppliedAt time.Time
}

type MigrationStatus struct {
	Applied []MigrationRecord
	Pending []MigrationRecord
	Unknown []MigrationRecord
}

func (status MigrationStatus) IsCurrent() bool {
	return len(status.Pending) == 0 && len(status.Unknown) == 0
}

type migrationFile struct {
	name     string
	checksum string
	sql      string
}

func DefaultMigrationOptions() MigrationOptions {
	return MigrationOptions{LockTimeout: 30 * time.Second, StatementTimeout: 5 * time.Minute}
}

func normalizeMigrationOptions(options MigrationOptions) MigrationOptions {
	defaults := DefaultMigrationOptions()
	if options.LockTimeout <= 0 {
		options.LockTimeout = defaults.LockTimeout
	}
	if options.StatementTimeout <= 0 {
		options.StatementTimeout = defaults.StatementTimeout
	}
	return options
}

func loadPostgresMigrations() ([]migrationFile, error) {
	entries, err := postgresMigrations.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	files := make([]migrationFile, 0, len(names))
	for _, name := range names {
		if !migrationFilenamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid migration filename %q; expected NNN_description.sql", name)
		}
		contents, err := postgresMigrations.ReadFile("migrations/" + name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		sum := sha256.Sum256(contents)
		files = append(files, migrationFile{name: name, checksum: hex.EncodeToString(sum[:]), sql: string(contents)})
	}
	return files, nil
}

var migrationFilenamePattern = regexp.MustCompile(`^[0-9]{3}_[a-z0-9_]+\.sql$`)

func ApplyPostgresMigrations(ctx context.Context, pool *pgxpool.Pool, options MigrationOptions) error {
	if pool == nil {
		return errors.New("postgres pool is required")
	}
	options = normalizeMigrationOptions(options)
	files, err := loadPostgresMigrations()
	if err != nil {
		return err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()
	if err := acquireMigrationLock(ctx, conn, options.LockTimeout); err != nil {
		return err
	}
	defer releaseMigrationLock(conn)

	if err := ensureMigrationTable(ctx, conn); err != nil {
		return err
	}
	known := make(map[string]struct{}, len(files))
	for _, file := range files {
		known[file.name] = struct{}{}
	}
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("list applied migration versions: %w", err)
	}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("scan applied migration version: %w", err)
		}
		if _, ok := known[version]; !ok {
			rows.Close()
			return fmt.Errorf("database contains migration %s unknown to this Stratum binary", version)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("list applied migration versions: %w", err)
	}
	rows.Close()
	for _, file := range files {
		var checksum string
		err := conn.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE version = $1`, file.name).Scan(&checksum)
		if err == nil {
			if checksum == "" {
				if _, err := conn.Exec(ctx, `UPDATE schema_migrations SET checksum = $2 WHERE version = $1 AND checksum = ''`, file.name, file.checksum); err != nil {
					return fmt.Errorf("backfill migration checksum %s: %w", file.name, err)
				}
				continue
			}
			if checksum != file.checksum {
				return fmt.Errorf("migration %s checksum mismatch; applied migrations must never be edited", file.name)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("read migration state %s: %w", file.name, err)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", file.name, err)
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout', $1, true)`, durationMilliseconds(options.LockTimeout)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("configure migration %s lock timeout: %w", file.name, err)
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, durationMilliseconds(options.StatementTimeout)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("configure migration %s statement timeout: %w", file.name, err)
		}
		if _, err := tx.Exec(ctx, file.sql); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", file.name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`, file.name, file.checksum); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", file.name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", file.name, err)
		}
	}
	return nil
}

func InspectPostgresMigrations(ctx context.Context, pool *pgxpool.Pool) (MigrationStatus, error) {
	if pool == nil {
		return MigrationStatus{}, errors.New("postgres pool is required")
	}
	files, err := loadPostgresMigrations()
	if err != nil {
		return MigrationStatus{}, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return MigrationStatus{}, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()
	var migrationTableExists bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&migrationTableExists); err != nil {
		return MigrationStatus{}, fmt.Errorf("inspect schema migration table: %w", err)
	}
	if !migrationTableExists {
		status := MigrationStatus{Pending: make([]MigrationRecord, 0, len(files))}
		for _, file := range files {
			status.Pending = append(status.Pending, MigrationRecord{Version: file.name, Checksum: file.checksum})
		}
		return status, nil
	}

	var checksumColumnExists bool
	if err := conn.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'schema_migrations'
      AND column_name = 'checksum'
)`).Scan(&checksumColumnExists); err != nil {
		return MigrationStatus{}, fmt.Errorf("inspect schema migration checksum support: %w", err)
	}
	query := `SELECT version, '' AS checksum, applied_at FROM schema_migrations ORDER BY version`
	if checksumColumnExists {
		query = `SELECT version, checksum, applied_at FROM schema_migrations ORDER BY version`
	}
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return MigrationStatus{}, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[string]MigrationRecord)
	status := MigrationStatus{}
	for rows.Next() {
		var record MigrationRecord
		if err := rows.Scan(&record.Version, &record.Checksum, &record.AppliedAt); err != nil {
			return MigrationStatus{}, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[record.Version] = record
		status.Applied = append(status.Applied, record)
	}
	if err := rows.Err(); err != nil {
		return MigrationStatus{}, fmt.Errorf("list applied migrations: %w", err)
	}

	for _, file := range files {
		record, ok := applied[file.name]
		if !ok {
			status.Pending = append(status.Pending, MigrationRecord{Version: file.name, Checksum: file.checksum})
			continue
		}
		if record.Checksum != "" && record.Checksum != file.checksum {
			return MigrationStatus{}, fmt.Errorf("migration %s checksum mismatch; applied migrations must never be edited", file.name)
		}
	}
	known := make(map[string]struct{}, len(files))
	for _, file := range files {
		known[file.name] = struct{}{}
	}
	for _, record := range status.Applied {
		if _, ok := known[record.Version]; !ok {
			status.Unknown = append(status.Unknown, record)
		}
	}
	return status, nil
}

func RunPostgresMigrations(ctx context.Context, databaseURL string, postgresOptions PostgresOptions, migrationOptions MigrationOptions) (MigrationStatus, error) {
	pool, err := openPostgresPool(ctx, databaseURL, postgresOptions)
	if err != nil {
		return MigrationStatus{}, err
	}
	defer pool.Close()
	if err := ApplyPostgresMigrations(ctx, pool, migrationOptions); err != nil {
		return MigrationStatus{}, err
	}
	return InspectPostgresMigrations(ctx, pool)
}

func CheckPostgresMigrations(ctx context.Context, databaseURL string, postgresOptions PostgresOptions) (MigrationStatus, error) {
	pool, err := openPostgresPool(ctx, databaseURL, postgresOptions)
	if err != nil {
		return MigrationStatus{}, err
	}
	defer pool.Close()
	return InspectPostgresMigrations(ctx, pool)
}

func ensureMigrationTable(ctx context.Context, conn *pgxpool.Conn) error {
	if _, err := conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    checksum TEXT NOT NULL DEFAULT '',
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum TEXT NOT NULL DEFAULT '';
`); err != nil {
		return fmt.Errorf("prepare schema migration table: %w", err)
	}
	return nil
}

func acquireMigrationLock(ctx context.Context, conn *pgxpool.Conn, timeout time.Duration) error {
	lockCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		var acquired bool
		if err := conn.QueryRow(lockCtx, `SELECT pg_try_advisory_lock($1)`, postgresMigrationLockKey).Scan(&acquired); err != nil {
			return fmt.Errorf("acquire migration lock: %w", err)
		}
		if acquired {
			return nil
		}
		select {
		case <-lockCtx.Done():
			return fmt.Errorf("acquire migration lock: %w", lockCtx.Err())
		case <-ticker.C:
		}
	}
}

func releaseMigrationLock(conn *pgxpool.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, postgresMigrationLockKey)
}

func durationMilliseconds(value time.Duration) string {
	return fmt.Sprintf("%dms", value.Milliseconds())
}
