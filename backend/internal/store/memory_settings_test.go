package store

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"testing"
)

func TestMemoryRepositoryMCPConfigPersistsAdminSettings(t *testing.T) {
	repo := NewMemoryRepository()

	initial, err := repo.GetMCPConfig(context.Background())
	if err != nil {
		t.Fatalf("GetMCPConfig returned error: %v", err)
	}
	if initial.Enabled || initial.EndpointPath != "/mcp" || !initial.RequireAdminConsent {
		t.Fatalf("initial mcp config = %#v", initial)
	}

	updated, err := repo.UpdateMCPConfig(context.Background(), domain.MCPConfig{
		Enabled:             true,
		EndpointPath:        "stratum-mcp",
		ReadCatalog:         true,
		ReadDesigns:         true,
		CreateDraftDesign:   false,
		RunAnalysis:         true,
		FetchImpactReport:   false,
		RequireAdminConsent: true,
	})
	if err != nil {
		t.Fatalf("UpdateMCPConfig returned error: %v", err)
	}
	if !updated.Enabled || updated.EndpointPath != "/stratum-mcp" || updated.CreateDraftDesign {
		t.Fatalf("updated mcp config = %#v", updated)
	}
}

func TestMemoryRepositoryTelemetryIntegrationConfigSanitizesSecret(t *testing.T) {
	repo := NewMemoryRepository()

	initial, err := repo.GetTelemetryIntegrationConfig(context.Background())
	if err != nil {
		t.Fatalf("GetTelemetryIntegrationConfig returned error: %v", err)
	}
	if initial.Enabled || initial.Provider != "prometheus" || initial.Secret != "" || initial.SecretSet {
		t.Fatalf("initial telemetry config = %#v", initial)
	}

	updated, err := repo.UpdateTelemetryIntegrationConfig(context.Background(), domain.TelemetryIntegrationConfig{
		Enabled:     true,
		Provider:    "prometheus",
		DisplayName: "Production Prometheus",
		BaseURL:     "https://prometheus.example.com",
		AuthMode:    "bearer",
		QueryWindow: "10m",
		Filters: domain.TelemetryIntegrationFilter{
			Namespaces:             []string{"payments", " payments ", "risk"},
			ExcludeEndpoints:       []string{"/healthz", "/readyz"},
			MinimumRequestsPerSec:  0.5,
			IncludeExternal:        true,
			IncludeDatabaseClients: true,
		},
	}, "secret-token")
	if err != nil {
		t.Fatalf("UpdateTelemetryIntegrationConfig returned error: %v", err)
	}
	if !updated.Enabled || updated.Secret != "" || !updated.SecretSet || len(updated.Filters.Namespaces) != 2 {
		t.Fatalf("sanitized telemetry config = %#v", updated)
	}

	withSecret, err := repo.GetTelemetryIntegrationConfigWithSecret(context.Background())
	if err != nil {
		t.Fatalf("GetTelemetryIntegrationConfigWithSecret returned error: %v", err)
	}
	if withSecret.Secret != "secret-token" {
		t.Fatalf("secret not preserved: %#v", withSecret)
	}

	withoutSecretChange, err := repo.UpdateTelemetryIntegrationConfig(context.Background(), domain.TelemetryIntegrationConfig{
		Enabled:     true,
		Provider:    "prometheus",
		DisplayName: "Production Prometheus",
		BaseURL:     "https://prometheus.example.com",
		AuthMode:    "bearer",
		QueryWindow: "15m",
	}, "")
	if err != nil {
		t.Fatalf("UpdateTelemetryIntegrationConfig without secret returned error: %v", err)
	}
	if !withoutSecretChange.SecretSet || withoutSecretChange.Secret != "" {
		t.Fatalf("sanitized config after blank secret = %#v", withoutSecretChange)
	}
	withSecret, err = repo.GetTelemetryIntegrationConfigWithSecret(context.Background())
	if err != nil {
		t.Fatalf("GetTelemetryIntegrationConfigWithSecret after blank secret returned error: %v", err)
	}
	if withSecret.Secret != "secret-token" || withSecret.QueryWindow != "15m" {
		t.Fatalf("secret should persist while other fields update: %#v", withSecret)
	}
}
