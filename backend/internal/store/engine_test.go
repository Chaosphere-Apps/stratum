package store

import "testing"

func TestStorageEngineStatelessStatusAndRepository(t *testing.T) {
	engine, err := NewStorageEngine(t.Context(), "")
	if err != nil {
		t.Fatalf("NewStorageEngine returned error: %v", err)
	}
	defer engine.Close()

	status := engine.Status(t.Context())
	if status.Mode != StorageModeStateless || !status.Stateless || status.Warning == "" {
		t.Fatalf("initial status = %#v, want stateless warning status", status)
	}
	if status.DatabaseConfigured || status.DatabaseConnected || status.DatabasePending {
		t.Fatalf("empty database URL should not configure database: %#v", status)
	}

	if _, err := engine.Repository().CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	status = engine.Status(t.Context())
	if status.CacheUserCount != 1 || status.CanMigrateUsers {
		t.Fatalf("status after cache user = %#v, want one cache user and no migration without pending db", status)
	}
}

func TestStorageEngineRejectsBlankDatabaseConfiguration(t *testing.T) {
	engine, err := NewStorageEngine(t.Context(), "")
	if err != nil {
		t.Fatalf("NewStorageEngine returned error: %v", err)
	}
	defer engine.Close()

	if err := engine.TestDatabase(t.Context(), "  "); err == nil {
		t.Fatal("expected blank test database URL to be rejected")
	}
	if _, err := engine.ConfigureDatabase(t.Context(), ""); err == nil {
		t.Fatal("expected blank configure database URL to be rejected")
	}
	status, migrated, err := engine.MigrateUsersToDatabase(t.Context())
	if err == nil || migrated != 0 {
		t.Fatalf("MigrateUsersToDatabase migrated=%d err=%v, want pending database error", migrated, err)
	}
	if status.Mode != StorageModeStateless {
		t.Fatalf("status mode = %q, want stateless after failed migration", status.Mode)
	}
}

func TestRedactDatabaseURL(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"", ""},
		{"postgres://user:secret@db.example.com:5432/stratum?sslmode=require", "postgres://user:****@db.example.com:5432/stratum?sslmode=require"},
		{"postgres://user@db.example.com/stratum", "postgres://user@db.example.com/stratum"},
		{"not-a-url", "not-a-url"},
	} {
		if got := redactDatabaseURL(test.input); got != test.want {
			t.Fatalf("redactDatabaseURL(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestDesignVersionStatusHelpers(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"", "draft"},
		{" DRAFT ", "draft"},
		{"pending_review", "pending_review"},
		{"REVIEWED", "reviewed"},
		{"live", "live"},
		{"unknown", "draft"},
	} {
		if got := NormalizeDesignVersionStatus(test.input); got != test.want {
			t.Fatalf("NormalizeDesignVersionStatus(%q) = %q, want %q", test.input, got, test.want)
		}
	}

	for _, status := range []string{"draft", "pending_review", "reviewed", "live", " LIVE "} {
		if !IsDesignVersionStatus(status) {
			t.Fatalf("expected %q to be a valid version status", status)
		}
	}
	if IsDesignVersionStatus("ready") {
		t.Fatal("unexpectedly accepted invalid version status")
	}

	allowed := [][2]string{
		{"draft", "pending_review"},
		{"pending_review", "reviewed"},
		{"reviewed", "live"},
		{"live", "reviewed"},
		{"live", "draft"},
		{"reviewed", "reviewed"},
	}
	for _, edge := range allowed {
		if err := ValidateDesignVersionStatusTransition(edge[0], edge[1]); err != nil {
			t.Fatalf("transition %s -> %s returned error: %v", edge[0], edge[1], err)
		}
	}

	for _, edge := range [][2]string{
		{"draft", "live"},
		{"pending_review", "live"},
		{"live", "pending_review"},
		{"reviewed", "archived"},
	} {
		if err := ValidateDesignVersionStatusTransition(edge[0], edge[1]); err == nil {
			t.Fatalf("transition %s -> %s unexpectedly allowed", edge[0], edge[1])
		}
	}
}
