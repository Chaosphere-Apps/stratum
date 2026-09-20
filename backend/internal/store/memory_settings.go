package store

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
)

func (r *MemoryRepository) GetAIProviderConfig(ctx context.Context) (domain.AIProviderConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIProviderConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return sanitizeAIProviderConfig(r.aiConfig), nil
}

func (r *MemoryRepository) GetAIProviderConfigWithSecret(ctx context.Context) (domain.AIProviderConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIProviderConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.aiConfig, nil
}

func (r *MemoryRepository) UpdateAIProviderConfig(ctx context.Context, config domain.AIProviderConfig, apiKey string) (domain.AIProviderConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIProviderConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.aiConfig
	current.Enabled = config.Enabled
	current.Provider = normalizedAIProvider(config.Provider)
	current.Model = strings.TrimSpace(config.Model)
	current.BaseURL = strings.TrimSpace(config.BaseURL)
	if strings.TrimSpace(apiKey) != "" {
		current.APIKey = strings.TrimSpace(apiKey)
		current.APIKeySet = true
		current.VerifiedAt = r.clock().UTC()
	}
	if current.Model == "" {
		current.Model = defaultAIModel(current.Provider)
	}
	current.UpdatedAt = r.clock().UTC()
	r.aiConfig = current
	return sanitizeAIProviderConfig(current), nil
}

func (r *MemoryRepository) GetMCPConfig(ctx context.Context) (domain.MCPConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.MCPConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mcpConfig, nil
}

func (r *MemoryRepository) UpdateMCPConfig(ctx context.Context, config domain.MCPConfig) (domain.MCPConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.MCPConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.mcpConfig
	current.Enabled = config.Enabled
	current.EndpointPath = normalizedMCPPath(config.EndpointPath)
	current.ReadCatalog = config.ReadCatalog
	current.ReadDesigns = config.ReadDesigns
	current.CreateDraftDesign = config.CreateDraftDesign
	current.RunAnalysis = config.RunAnalysis
	current.FetchImpactReport = config.FetchImpactReport
	current.RequireAdminConsent = config.RequireAdminConsent
	current.UpdatedAt = r.clock().UTC()
	r.mcpConfig = current
	return current, nil
}

func (r *MemoryRepository) GetTelemetryIntegrationConfig(ctx context.Context) (domain.TelemetryIntegrationConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return sanitizeTelemetryIntegrationConfig(r.telemetryConfig), nil
}

func (r *MemoryRepository) GetTelemetryIntegrationConfigWithSecret(ctx context.Context) (domain.TelemetryIntegrationConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.telemetryConfig, nil
}

func (r *MemoryRepository) UpdateTelemetryIntegrationConfig(ctx context.Context, config domain.TelemetryIntegrationConfig, secret string) (domain.TelemetryIntegrationConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.telemetryConfig
	next, err := normalizedTelemetryIntegrationConfig(current, config, secret, r.clock().UTC())
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	r.telemetryConfig = next
	return sanitizeTelemetryIntegrationConfig(next), nil
}
