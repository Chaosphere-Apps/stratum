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
	ShutdownTimeout            time.Duration
	ReadHeaderTimeout          time.Duration
	ReadTimeout                time.Duration
	WriteTimeout               time.Duration
	IdleTimeout                time.Duration
	MaxRequestBodyBytes        int64
	WebSocketReadLimit         int64
	AllowPrivateAIProviderURLs bool
}

func Load() Config {
	return Config{
		Addr:                       envString("BACKEND_ADDR", ":8081"),
		AllowedOrigins:             envList("ALLOWED_ORIGINS", []string{"http://localhost:*", "http://127.0.0.1:*", "http://[::1]:*"}),
		DatabaseURL:                envString("DATABASE_URL", ""),
		ShutdownTimeout:            envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadHeaderTimeout:          envDuration("READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:                envDuration("READ_TIMEOUT", 15*time.Second),
		WriteTimeout:               envDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:                envDuration("IDLE_TIMEOUT", 60*time.Second),
		MaxRequestBodyBytes:        envInt64("MAX_REQUEST_BODY_BYTES", 4<<20),
		WebSocketReadLimit:         envInt64("WEBSOCKET_READ_LIMIT_BYTES", 2<<20),
		AllowPrivateAIProviderURLs: envBool("ALLOW_PRIVATE_AI_PROVIDER_URLS", false),
	}
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
