package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
	"time"
)

func (r *PostgresRepository) GetAIProviderConfig(ctx context.Context) (domain.AIProviderConfig, error) {
	config, err := r.getAIProviderConfig(ctx, false)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	return config, nil
}

func (r *PostgresRepository) GetAIProviderConfigWithSecret(ctx context.Context) (domain.AIProviderConfig, error) {
	return r.getAIProviderConfig(ctx, true)
}

func (r *PostgresRepository) getAIProviderConfig(ctx context.Context, includeSecret bool) (domain.AIProviderConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultAIProviderConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.AIProviderConfig{}, err
	}
	row := tx.QueryRow(ctx, `
SELECT enabled, provider, model, base_url, api_key, api_key <> '', verified_at, updated_at
FROM ai_provider_settings
WHERE id = 'default'
`)
	config, err := scanAIProviderConfig(row)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AIProviderConfig{}, err
	}
	if !includeSecret {
		config.APIKey = ""
	}
	return config, nil
}

func (r *PostgresRepository) UpdateAIProviderConfig(ctx context.Context, config domain.AIProviderConfig, apiKey string) (domain.AIProviderConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultAIProviderConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.AIProviderConfig{}, err
	}
	currentSecret := ""
	var currentVerifiedAt sql.NullTime
	if err := tx.QueryRow(ctx, `SELECT api_key, verified_at FROM ai_provider_settings WHERE id = 'default' FOR UPDATE`).Scan(&currentSecret, &currentVerifiedAt); err != nil {
		return domain.AIProviderConfig{}, err
	}
	verifiedAt := time.Time{}
	if currentVerifiedAt.Valid {
		verifiedAt = currentVerifiedAt.Time
	}
	if strings.TrimSpace(apiKey) != "" {
		currentSecret = strings.TrimSpace(apiKey)
		verifiedAt = r.clock().UTC()
	}
	provider := normalizedAIProvider(config.Provider)
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = defaultAIModel(provider)
	}
	updatedAt := r.clock().UTC()
	row := tx.QueryRow(ctx, `
UPDATE ai_provider_settings
SET enabled = $1,
	provider = $2,
	model = $3,
	base_url = $4,
	api_key = $5,
	verified_at = $6,
	updated_at = $7
WHERE id = 'default'
RETURNING enabled, provider, model, base_url, api_key, api_key <> '', verified_at, updated_at
`, config.Enabled, provider, model, strings.TrimSpace(config.BaseURL), currentSecret, nullableTime(verifiedAt), updatedAt)
	next, err := scanAIProviderConfig(row)
	if err != nil {
		return domain.AIProviderConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AIProviderConfig{}, err
	}
	return sanitizeAIProviderConfig(next), nil
}

func (r *PostgresRepository) GetMCPConfig(ctx context.Context) (domain.MCPConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.MCPConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultMCPConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.MCPConfig{}, err
	}
	row := tx.QueryRow(ctx, `
SELECT enabled, endpoint_path, read_catalog, read_designs, create_draft_design, run_analysis, fetch_impact_report, require_admin_consent, updated_at
FROM mcp_settings
WHERE id = 'default'
`)
	config, err := scanMCPConfig(row)
	if err != nil {
		return domain.MCPConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.MCPConfig{}, err
	}
	return config, nil
}

func (r *PostgresRepository) UpdateMCPConfig(ctx context.Context, config domain.MCPConfig) (domain.MCPConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.MCPConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultMCPConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.MCPConfig{}, err
	}
	row := tx.QueryRow(ctx, `
UPDATE mcp_settings
SET enabled = $1,
	endpoint_path = $2,
	read_catalog = $3,
	read_designs = $4,
	create_draft_design = $5,
	run_analysis = $6,
	fetch_impact_report = $7,
	require_admin_consent = $8,
	updated_at = $9
WHERE id = 'default'
RETURNING enabled, endpoint_path, read_catalog, read_designs, create_draft_design, run_analysis, fetch_impact_report, require_admin_consent, updated_at
`, config.Enabled, normalizedMCPPath(config.EndpointPath), config.ReadCatalog, config.ReadDesigns, config.CreateDraftDesign, config.RunAnalysis, config.FetchImpactReport, config.RequireAdminConsent, r.clock().UTC())
	next, err := scanMCPConfig(row)
	if err != nil {
		return domain.MCPConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.MCPConfig{}, err
	}
	return next, nil
}

func (r *PostgresRepository) GetTelemetryIntegrationConfig(ctx context.Context) (domain.TelemetryIntegrationConfig, error) {
	config, err := r.getTelemetryIntegrationConfig(ctx, false)
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	return config, nil
}

func (r *PostgresRepository) GetTelemetryIntegrationConfigWithSecret(ctx context.Context) (domain.TelemetryIntegrationConfig, error) {
	return r.getTelemetryIntegrationConfig(ctx, true)
}

func (r *PostgresRepository) getTelemetryIntegrationConfig(ctx context.Context, includeSecret bool) (domain.TelemetryIntegrationConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultTelemetryIntegrationConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	row := tx.QueryRow(ctx, `
SELECT enabled, provider, display_name, base_url, auth_mode, secret, secret <> '', custom_header_name,
	query_window, request_total_metric, request_failed_metric, server_latency_metric, client_latency_metric, filters, updated_at
FROM telemetry_integration_settings
WHERE id = 'default'
`)
	config, err := scanTelemetryIntegrationConfig(row)
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	if !includeSecret {
		return sanitizeTelemetryIntegrationConfig(config), nil
	}
	return config, nil
}

func (r *PostgresRepository) UpdateTelemetryIntegrationConfig(ctx context.Context, config domain.TelemetryIntegrationConfig, secret string) (domain.TelemetryIntegrationConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDefaultTelemetryIntegrationConfig(ctx, tx, r.clock().UTC()); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	current, err := getTelemetryIntegrationConfigWithTx(ctx, tx)
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	next, err := normalizedTelemetryIntegrationConfig(current, config, secret, r.clock().UTC())
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	filterBytes, err := json.Marshal(next.Filters)
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	row := tx.QueryRow(ctx, `
UPDATE telemetry_integration_settings
SET enabled = $1,
	provider = $2,
	display_name = $3,
	base_url = $4,
	auth_mode = $5,
	secret = $6,
	custom_header_name = $7,
	query_window = $8,
	request_total_metric = $9,
	request_failed_metric = $10,
	server_latency_metric = $11,
	client_latency_metric = $12,
	filters = $13,
	updated_at = $14
WHERE id = 'default'
RETURNING enabled, provider, display_name, base_url, auth_mode, secret, secret <> '', custom_header_name,
	query_window, request_total_metric, request_failed_metric, server_latency_metric, client_latency_metric, filters, updated_at
`, next.Enabled, next.Provider, next.DisplayName, next.BaseURL, next.AuthMode, next.Secret, next.CustomHeaderName,
		next.QueryWindow, next.RequestTotalMetric, next.RequestFailedMetric, next.ServerLatencyMetric, next.ClientLatencyMetric, string(filterBytes), next.UpdatedAt)
	updated, err := scanTelemetryIntegrationConfig(row)
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	return sanitizeTelemetryIntegrationConfig(updated), nil
}
