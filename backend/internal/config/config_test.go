package config

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadUsesDefaultsAndParsesEnvironment(t *testing.T) {
	t.Setenv("BACKEND_ADDR", ":9090")
	t.Setenv("ALLOWED_ORIGINS", " https://app.example.com, ,http://localhost:5173 ")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("AUTO_MIGRATE", "false")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "32")
	t.Setenv("DATABASE_MIN_CONNECTIONS", "4")
	t.Setenv("MIGRATION_STATEMENT_TIMEOUT", "7m")
	t.Setenv("STARTUP_TIMEOUT", "90s")
	t.Setenv("SHUTDOWN_TIMEOUT", "25s")
	t.Setenv("READ_HEADER_TIMEOUT", "bad-duration")
	t.Setenv("MAX_REQUEST_BODY_BYTES", "8192")
	t.Setenv("WEBSOCKET_READ_LIMIT_BYTES", "bad-int")
	t.Setenv("ALLOW_PRIVATE_AI_PROVIDER_URLS", "true")
	t.Setenv("STATIC_ASSETS_DIR", "/srv/stratum")
	t.Setenv("SESSION_TTL", "30m")

	cfg := Load()
	if cfg.Addr != ":9090" || cfg.DatabaseURL != "postgres://example" || cfg.StaticAssetsDir != "/srv/stratum" {
		t.Fatalf("string env config not applied: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, []string{"https://app.example.com", "http://localhost:5173"}) {
		t.Fatalf("allowed origins = %#v", cfg.AllowedOrigins)
	}
	if cfg.ShutdownTimeout != 25*time.Second || cfg.SessionTTL != 30*time.Minute {
		t.Fatalf("duration env config not applied: %#v", cfg)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("invalid duration should fall back, got %s", cfg.ReadHeaderTimeout)
	}
	if cfg.MaxRequestBodyBytes != 8192 || cfg.WebSocketReadLimit != 2<<20 {
		t.Fatalf("int env config not applied/fallback not used: %#v", cfg)
	}
	if !cfg.AllowPrivateAIProviderURLs {
		t.Fatal("expected boolean env to parse true")
	}
	if cfg.DatabaseAutoMigrate || cfg.DatabaseMaxConnections != 32 || cfg.DatabaseMinConnections != 4 {
		t.Fatalf("database config not applied: %#v", cfg)
	}
	if cfg.MigrationStatementTimeout != 7*time.Minute || cfg.StartupTimeout != 90*time.Second {
		t.Fatalf("migration/startup timeouts not applied: %#v", cfg)
	}
}

func TestLoadFallsBackForBlankAndInvalidValues(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", ", ,")
	t.Setenv("ALLOW_PRIVATE_AI_PROVIDER_URLS", "not-bool")

	cfg := Load()
	if !reflect.DeepEqual(cfg.AllowedOrigins, []string{"http://localhost:*", "http://127.0.0.1:*", "http://[::1]:*"}) {
		t.Fatalf("blank origin list should fall back, got %#v", cfg.AllowedOrigins)
	}
	if cfg.AllowPrivateAIProviderURLs {
		t.Fatal("invalid boolean should fall back to false")
	}
	if !cfg.DatabaseAutoMigrate || cfg.DatabaseMaxConnections != 20 || cfg.DatabaseMinConnections != 2 {
		t.Fatalf("database defaults not applied: %#v", cfg)
	}
}
