package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/store"
)

func main() {
	cfg := config.Load()
	defaultDatabaseURL := os.Getenv("MIGRATION_DATABASE_URL")
	if defaultDatabaseURL == "" {
		defaultDatabaseURL = cfg.DatabaseURL
	}
	databaseURL := flag.String("database-url", defaultDatabaseURL, "PostgreSQL connection URL (defaults to MIGRATION_DATABASE_URL, then DATABASE_URL)")
	lockTimeout := flag.Duration("lock-timeout", cfg.MigrationLockTimeout, "maximum time to wait for another migrator")
	statementTimeout := flag.Duration("statement-timeout", cfg.MigrationStatementTimeout, "maximum execution time for one migration")
	flag.Parse()

	if *databaseURL == "" {
		exitf("DATABASE_URL or -database-url is required")
	}
	command := "status"
	if flag.NArg() > 0 {
		command = flag.Arg(0)
	}
	if flag.NArg() > 1 {
		exitf("usage: migrate [flags] [status|check|up]")
	}

	postgresOptions := postgresOptionsFromConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), *lockTimeout+*statementTimeout+30*time.Second)
	defer cancel()

	var (
		status store.MigrationStatus
		err    error
	)
	switch command {
	case "status":
		status, err = store.CheckPostgresMigrations(ctx, *databaseURL, postgresOptions)
	case "check":
		status, err = store.CheckPostgresMigrations(ctx, *databaseURL, postgresOptions)
	case "up":
		status, err = store.RunPostgresMigrations(ctx, *databaseURL, postgresOptions, store.MigrationOptions{
			LockTimeout:      *lockTimeout,
			StatementTimeout: *statementTimeout,
		})
	default:
		exitf("unknown command %q; expected status, check, or up", command)
	}
	if err != nil {
		exitf("migration %s failed: %v", command, err)
	}
	printStatus(status)
	if command == "check" && !status.IsCurrent() {
		exitf("migration check failed: database schema is not current")
	}
}

func postgresOptionsFromConfig(cfg config.Config) store.PostgresOptions {
	return store.PostgresOptions{
		AutoMigrate:       false,
		MaxConns:          int32(cfg.DatabaseMaxConnections),
		MinConns:          0,
		MaxConnLifetime:   cfg.DatabaseMaxConnLifetime,
		MaxConnIdleTime:   cfg.DatabaseMaxConnIdleTime,
		HealthCheckPeriod: cfg.DatabaseHealthCheckPeriod,
	}
}

func printStatus(status store.MigrationStatus) {
	for _, migration := range status.Applied {
		fmt.Printf("applied  %s  %s\n", migration.Version, migration.AppliedAt.UTC().Format(time.RFC3339))
	}
	for _, migration := range status.Pending {
		fmt.Printf("pending  %s\n", migration.Version)
	}
	for _, migration := range status.Unknown {
		fmt.Printf("unknown  %s\n", migration.Version)
	}
	fmt.Printf("summary  %d applied, %d pending, %d unknown\n", len(status.Applied), len(status.Pending), len(status.Unknown))
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
