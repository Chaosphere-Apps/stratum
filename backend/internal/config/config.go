package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                       string
	AllowedOrigins             []string
	DatabaseURL                string
	DatabaseAutoMigrate        bool
	DatabaseMaxConnections     int
	DatabaseMinConnections     int
	DatabaseMaxConnLifetime    time.Duration
	DatabaseMaxConnIdleTime    time.Duration
	DatabaseHealthCheckPeriod  time.Duration
	MigrationLockTimeout       time.Duration
	MigrationStatementTimeout  time.Duration
	StartupTimeout             time.Duration
	ShutdownTimeout            time.Duration
	ReadHeaderTimeout          time.Duration
	ReadTimeout                time.Duration
	WriteTimeout               time.Duration
	IdleTimeout                time.Duration
	MaxRequestBodyBytes        int64
	WebSocketReadLimit         int64
	AllowPrivateAIProviderURLs bool
	StaticAssetsDir            string
	SessionTTL                 time.Duration
	PublicURL                  string
	TrustedProxyCIDRs          []string
	LoginAttemptsPerWindow     int
	PasswordResetAttempts      int
	AuthRateLimitWindow        time.Duration
}

func Load() Config {
	return Config{
		Addr:                       envString("BACKEND_ADDR", ":8081"),
		AllowedOrigins:             envList("ALLOWED_ORIGINS", []string{"http://localhost:*", "http://127.0.0.1:*", "http://[::1]:*"}),
		DatabaseURL:                envString("DATABASE_URL", ""),
		DatabaseAutoMigrate:        envBool("AUTO_MIGRATE", true),
		DatabaseMaxConnections:     envInt("DATABASE_MAX_CONNECTIONS", 20),
		DatabaseMinConnections:     envInt("DATABASE_MIN_CONNECTIONS", 2),
		DatabaseMaxConnLifetime:    envDuration("DATABASE_MAX_CONN_LIFETIME", 30*time.Minute),
		DatabaseMaxConnIdleTime:    envDuration("DATABASE_MAX_CONN_IDLE_TIME", 5*time.Minute),
		DatabaseHealthCheckPeriod:  envDuration("DATABASE_HEALTH_CHECK_PERIOD", 30*time.Second),
		MigrationLockTimeout:       envDuration("MIGRATION_LOCK_TIMEOUT", 30*time.Second),
		MigrationStatementTimeout:  envDuration("MIGRATION_STATEMENT_TIMEOUT", 5*time.Minute),
		StartupTimeout:             envDuration("STARTUP_TIMEOUT", 2*time.Minute),
		ShutdownTimeout:            envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout:          envDuration("READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:                envDuration("READ_TIMEOUT", 15*time.Second),
		WriteTimeout:               envDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:                envDuration("IDLE_TIMEOUT", 60*time.Second),
		MaxRequestBodyBytes:        envInt64("MAX_REQUEST_BODY_BYTES", 4<<20),
		WebSocketReadLimit:         envInt64("WEBSOCKET_READ_LIMIT_BYTES", 2<<20),
		AllowPrivateAIProviderURLs: envBool("ALLOW_PRIVATE_AI_PROVIDER_URLS", false),
		StaticAssetsDir:            envString("STATIC_ASSETS_DIR", ""),
		SessionTTL:                 envDuration("SESSION_TTL", 12*time.Hour),
		PublicURL:                  strings.TrimRight(envString("PUBLIC_URL", ""), "/"),
		TrustedProxyCIDRs:          envList("TRUSTED_PROXY_CIDRS", nil),
		LoginAttemptsPerWindow:     envInt("LOGIN_ATTEMPTS_PER_WINDOW", 10),
		PasswordResetAttempts:      envInt("PASSWORD_RESET_ATTEMPTS_PER_WINDOW", 6),
		AuthRateLimitWindow:        envDuration("AUTH_RATE_LIMIT_WINDOW", 5*time.Minute),
	}
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func Logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

func envString(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
