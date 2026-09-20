package store

import (
	"testing"
	"time"
)

func TestEmbeddedPostgresMigrationsAreOrderedAndChecksummed(t *testing.T) {
	t.Parallel()
	files, err := loadPostgresMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no embedded PostgreSQL migrations")
	}
	seen := make(map[string]struct{}, len(files))
	for index, file := range files {
		if !migrationFilenamePattern.MatchString(file.name) {
			t.Fatalf("invalid migration filename %q", file.name)
		}
		if len(file.checksum) != 64 {
			t.Fatalf("migration %s checksum length = %d", file.name, len(file.checksum))
		}
		if file.sql == "" {
			t.Fatalf("migration %s is empty", file.name)
		}
		if _, duplicate := seen[file.name]; duplicate {
			t.Fatalf("duplicate migration %s", file.name)
		}
		seen[file.name] = struct{}{}
		if index > 0 && files[index-1].name >= file.name {
			t.Fatalf("migrations are not strictly ordered: %s then %s", files[index-1].name, file.name)
		}
	}
}

func TestMigrationAndPostgresOptionsUseProductionDefaults(t *testing.T) {
	t.Parallel()
	migration := normalizeMigrationOptions(MigrationOptions{})
	if migration.LockTimeout != 30*time.Second || migration.StatementTimeout != 5*time.Minute {
		t.Fatalf("migration defaults = %#v", migration)
	}
	postgres := normalizePostgresOptions(PostgresOptions{MinConns: 50, MaxConns: 10})
	if postgres.MaxConns != 10 || postgres.MinConns != 10 {
		t.Fatalf("normalized pool sizes = min %d max %d", postgres.MinConns, postgres.MaxConns)
	}
	if postgres.MaxConnLifetime <= 0 || postgres.MaxConnIdleTime <= 0 || postgres.HealthCheckPeriod <= 0 {
		t.Fatalf("pool duration defaults = %#v", postgres)
	}
}

func TestMigrationStatusIsCurrent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status MigrationStatus
		want   bool
	}{
		{name: "empty current state", want: true},
		{name: "applied only", status: MigrationStatus{Applied: []MigrationRecord{{Version: "001.sql"}}}, want: true},
		{name: "pending", status: MigrationStatus{Pending: []MigrationRecord{{Version: "002.sql"}}}},
		{name: "unknown", status: MigrationStatus{Unknown: []MigrationRecord{{Version: "999.sql"}}}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.status.IsCurrent(); got != test.want {
				t.Fatalf("IsCurrent() = %t, want %t", got, test.want)
			}
		})
	}
}
