package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/modelgateway"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Server) handleGetAIProviderConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	config, err := s.services.Identity.GetAIProviderConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"aiProvider": config})
}

func (s *Server) handleUpdateAIProviderConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Enabled  bool   `json:"enabled"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		BaseURL  string `json:"baseUrl"`
		APIKey   string `json:"apiKey"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		message := jsonRequestError(err)
		s.log.Warn("ai_provider_update_rejected", "stage", "request_validation", "reason", message)
		writeError(w, http.StatusBadRequest, message)
		return
	}
	verifyKey := strings.TrimSpace(body.APIKey)
	current, currentErr := s.services.Identity.GetAIProviderConfigWithSecret(r.Context())
	providerChanged := currentErr == nil && !strings.EqualFold(strings.TrimSpace(current.Provider), strings.TrimSpace(body.Provider))
	if providerChanged && verifyKey == "" {
		writeError(w, http.StatusBadRequest, "API key is required when changing AI providers")
		return
	}
	if body.Enabled && verifyKey == "" {
		if currentErr == nil {
			verifyKey = current.APIKey
		}
	}
	if body.Enabled || verifyKey != "" {
		if err := verifyAIProvider(r.Context(), body.Provider, body.Model, body.BaseURL, verifyKey, s.cfg.AllowPrivateAIProviderURLs); err != nil {
			s.log.Warn("ai_provider_update_rejected", "stage", "provider_verification", "provider", strings.ToLower(strings.TrimSpace(body.Provider)), "reason", err.Error())
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	config, err := s.services.Identity.UpdateAIProviderConfig(r.Context(), domain.AIProviderConfig{
		Enabled:  body.Enabled,
		Provider: body.Provider,
		Model:    body.Model,
		BaseURL:  body.BaseURL,
	}, body.APIKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"aiProvider": config})
}

func (s *Server) handleGetMCPConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	config, err := s.services.Identity.GetMCPConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mcp": config})
}

func (s *Server) handleUpdateMCPConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Enabled             bool   `json:"enabled"`
		EndpointPath        string `json:"endpointPath"`
		ReadCatalog         bool   `json:"readCatalog"`
		ReadDesigns         bool   `json:"readDesigns"`
		CreateDraftDesign   bool   `json:"createDraftDesign"`
		RunAnalysis         bool   `json:"runAnalysis"`
		FetchImpactReport   bool   `json:"fetchImpactReport"`
		RequireAdminConsent bool   `json:"requireAdminConsent"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	config, err := s.services.Identity.UpdateMCPConfig(r.Context(), domain.MCPConfig{
		Enabled:             body.Enabled,
		EndpointPath:        body.EndpointPath,
		ReadCatalog:         body.ReadCatalog,
		ReadDesigns:         body.ReadDesigns,
		CreateDraftDesign:   body.CreateDraftDesign,
		RunAnalysis:         body.RunAnalysis,
		FetchImpactReport:   body.FetchImpactReport,
		RequireAdminConsent: body.RequireAdminConsent,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mcp": config})
}

func (s *Server) handleGetTelemetryIntegrationConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	config, err := s.services.Identity.GetTelemetryIntegrationConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"telemetry": config})
}

func (s *Server) handleUpdateTelemetryIntegrationConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Enabled             bool                              `json:"enabled"`
		Provider            string                            `json:"provider"`
		DisplayName         string                            `json:"displayName"`
		BaseURL             string                            `json:"baseUrl"`
		AuthMode            string                            `json:"authMode"`
		Secret              string                            `json:"secret"`
		CustomHeaderName    string                            `json:"customHeaderName"`
		QueryWindow         string                            `json:"queryWindow"`
		RequestTotalMetric  string                            `json:"requestTotalMetric"`
		RequestFailedMetric string                            `json:"requestFailedMetric"`
		ServerLatencyMetric string                            `json:"serverLatencyMetric"`
		ClientLatencyMetric string                            `json:"clientLatencyMetric"`
		Filters             domain.TelemetryIntegrationFilter `json:"filters"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	config, err := s.services.Identity.UpdateTelemetryIntegrationConfig(r.Context(), domain.TelemetryIntegrationConfig{
		Enabled:             body.Enabled,
		Provider:            body.Provider,
		DisplayName:         body.DisplayName,
		BaseURL:             body.BaseURL,
		AuthMode:            body.AuthMode,
		CustomHeaderName:    body.CustomHeaderName,
		QueryWindow:         body.QueryWindow,
		RequestTotalMetric:  body.RequestTotalMetric,
		RequestFailedMetric: body.RequestFailedMetric,
		ServerLatencyMetric: body.ServerLatencyMetric,
		ClientLatencyMetric: body.ClientLatencyMetric,
		Filters:             body.Filters,
	}, body.Secret)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"telemetry": config})
}

func (s *Server) handleListCatalogAssets(w http.ResponseWriter, r *http.Request) {
	if _, err := s.currentUser(r); err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return
	}
	assets, page, err := s.services.Catalog.ListAssetsPage(r.Context(), pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets, "page": page})
}

func (s *Server) handleCreateCatalogAsset(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireRole(w, r, "architect")
	if !ok {
		return
	}
	var body struct {
		Name               string          `json:"name"`
		Kind               string          `json:"kind"`
		Type               string          `json:"type"`
		Owner              string          `json:"owner"`
		Description        string          `json:"description"`
		Criticality        string          `json:"criticality"`
		Status             string          `json:"status"`
		Aliases            []string        `json:"aliases"`
		ReplacementAssetID string          `json:"replacementAssetId"`
		UpdateMessage      string          `json:"updateMessage"`
		Tags               []string        `json:"tags"`
		Metadata           json.RawMessage `json:"metadata"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asset, err := s.services.Catalog.CreateAsset(r.Context(), domain.CatalogAsset{
		Name:               body.Name,
		Kind:               body.Kind,
		Type:               body.Type,
		Owner:              body.Owner,
		Description:        body.Description,
		Criticality:        body.Criticality,
		Status:             body.Status,
		Aliases:            body.Aliases,
		ReplacementAssetID: body.ReplacementAssetID,
		UpdateMessage:      body.UpdateMessage,
		Tags:               body.Tags,
		Metadata:           body.Metadata,
		CreatedBy:          user.ID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"asset": asset})
}

func (s *Server) handleUpdateCatalogAsset(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Name               string          `json:"name"`
		Kind               string          `json:"kind"`
		Type               string          `json:"type"`
		Owner              string          `json:"owner"`
		Description        string          `json:"description"`
		Criticality        string          `json:"criticality"`
		Status             string          `json:"status"`
		Aliases            []string        `json:"aliases"`
		ReplacementAssetID string          `json:"replacementAssetId"`
		UpdateMessage      string          `json:"updateMessage"`
		Tags               []string        `json:"tags"`
		Metadata           json.RawMessage `json:"metadata"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asset, err := s.services.Catalog.UpdateAsset(r.Context(), r.PathValue("assetID"), domain.CatalogAsset{
		Name:               body.Name,
		Kind:               body.Kind,
		Type:               body.Type,
		Owner:              body.Owner,
		Description:        body.Description,
		Criticality:        body.Criticality,
		Status:             body.Status,
		Aliases:            body.Aliases,
		ReplacementAssetID: body.ReplacementAssetID,
		UpdateMessage:      body.UpdateMessage,
		Tags:               body.Tags,
		Metadata:           body.Metadata,
	})
	if err != nil {
		writeError(w, statusForCatalogError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"asset": asset})
}

func (s *Server) handleDeleteCatalogAsset(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.services.Catalog.DeleteAsset(r.Context(), r.PathValue("assetID")); err != nil {
		writeError(w, statusForCatalogError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user is required")
		return
	}
	notifications, err := s.services.Identity.ListNotifications(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": notifications})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user is required")
		return
	}
	if err := s.services.Identity.MarkNotificationRead(r.Context(), user.ID, r.PathValue("notificationID")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleVerifyAIProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		BaseURL  string `json:"baseUrl"`
		APIKey   string `json:"apiKey"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := verifyAIProvider(r.Context(), body.Provider, body.Model, body.BaseURL, body.APIKey, s.cfg.AllowPrivateAIProviderURLs); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"message":    "Provider connection verified",
		"provider":   strings.TrimSpace(body.Provider),
		"model":      strings.TrimSpace(body.Model),
		"verifiedAt": time.Now().UTC(),
	})
}

func verifyAIProvider(ctx context.Context, provider string, model string, baseURL string, apiKey string, allowPrivateURLs bool) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	apiKey = strings.TrimSpace(apiKey)
	if provider == "" {
		return errors.New("provider is required")
	}
	if apiKey == "" {
		return errors.New("API key is required")
	}

	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint == "" {
		switch provider {
		case "openai":
			endpoint = "https://api.openai.com/v1"
		case "anthropic":
			endpoint = "https://api.anthropic.com/v1"
		case "openrouter":
			endpoint = "https://openrouter.ai/api/v1"
		case "google":
			endpoint = "https://generativelanguage.googleapis.com/v1beta"
		default:
			return errors.New("base URL is required for custom providers")
		}
	}
	if err := validateProviderEndpoint(endpoint, allowPrivateURLs); err != nil {
		return err
	}

	modelsPath := strings.TrimRight(endpoint, "/") + "/models"
	if provider == "google" {
		model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
		if model == "" {
			return errors.New("model is required")
		}
		modelsPath += "/" + url.PathEscape(model)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsPath, nil)
	if err != nil {
		return errors.New("provider URL is invalid")
	}
	if provider == "anthropic" {
		request.Header.Set("x-api-key", apiKey)
		request.Header.Set("anthropic-version", "2023-06-01")
	} else if provider == "google" {
		request.Header.Set("x-goog-api-key", apiKey)
	} else {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	request.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 8 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("provider could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if provider == "google" && response.StatusCode == http.StatusNotFound {
			return errors.New("Google AI Studio could not find the configured model; choose a model available to this API key")
		}
		return errors.New("provider rejected the credentials or configured model")
	}
	return nil
}

func runAIAnalysis(ctx context.Context, gateway modelgateway.Gateway, config domain.AIProviderConfig, document json.RawMessage, report analysis.Report, options analysis.ReviewOptions) (analysis.AIReview, error) {
	userPrompt, err := analysis.BuildReviewUserPrompt(document, report, options)
	if err != nil {
		return analysis.AIReview{}, err
	}
	content, err := gateway.Complete(ctx, config, modelgateway.Request{
		System: analysis.BuildReviewSystemPrompt(options),
		Messages: []modelgateway.Message{
			{Role: "user", Content: userPrompt},
		},
		JSONResponse: true,
		MaxTokens:    2600,
	})
	if err != nil {
		return analysis.AIReview{}, err
	}
	return parseAIReview(content, config), nil
}

func runOpenAICompatibleAnalysis(ctx context.Context, endpoint string, config domain.AIProviderConfig, userPrompt string) (analysis.AIReview, error) {
	payload := map[string]any{
		"model":       strings.TrimSpace(config.Model),
		"temperature": 0.1,
		"messages": []map[string]string{
			{"role": "system", "content": analysis.BuildSystemPrompt()},
			{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return analysis.AIReview{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return analysis.AIReview{}, errors.New("provider URL is invalid")
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(config.APIKey))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 24 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return analysis.AIReview{}, errors.New("provider could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return analysis.AIReview{}, errors.New("provider rejected the analysis request")
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&parsed); err != nil {
		return analysis.AIReview{}, errors.New("provider returned invalid JSON")
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return analysis.AIReview{}, errors.New("provider returned an empty review")
	}
	return parseAIReview(parsed.Choices[0].Message.Content, config), nil
}

func runAnthropicAnalysis(ctx context.Context, endpoint string, config domain.AIProviderConfig, userPrompt string) (analysis.AIReview, error) {
	payload := map[string]any{
		"model":       strings.TrimSpace(config.Model),
		"max_tokens":  2200,
		"temperature": 0.1,
		"system":      analysis.BuildSystemPrompt(),
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return analysis.AIReview{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/messages", bytes.NewReader(body))
	if err != nil {
		return analysis.AIReview{}, errors.New("provider URL is invalid")
	}
	request.Header.Set("x-api-key", strings.TrimSpace(config.APIKey))
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 24 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return analysis.AIReview{}, errors.New("provider could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return analysis.AIReview{}, errors.New("provider rejected the analysis request")
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&parsed); err != nil {
		return analysis.AIReview{}, errors.New("provider returned invalid JSON")
	}
	for _, content := range parsed.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			return parseAIReview(content.Text, config), nil
		}
	}
	return analysis.AIReview{}, errors.New("provider returned an empty review")
}

func parseAIReview(content string, config domain.AIProviderConfig) analysis.AIReview {
	var envelope analysis.AIReviewEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &envelope); err != nil {
		return analysis.AIReview{
			Status:        "failed",
			Provider:      config.Provider,
			Model:         config.Model,
			PromptVersion: analysis.PromptVersion,
			Error:         "AI response did not match the expected JSON shape",
		}
	}
	return analysis.AIReview{
		Status:          "completed",
		Provider:        config.Provider,
		Model:           config.Model,
		PromptVersion:   analysis.PromptVersion,
		ExecutiveReview: strings.TrimSpace(envelope.ExecutiveReview),
		Strengths:       cleanReviewStrings(envelope.Strengths),
		Risks:           cleanReviewPoints(envelope.Risks),
		Recommendations: cleanReviewPoints(envelope.Recommendations),
		OpenQuestions:   cleanReviewStrings(envelope.OpenQuestions),
	}
}

func cleanReviewStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func cleanReviewPoints(points []analysis.AIReviewPoint) []analysis.AIReviewPoint {
	cleaned := make([]analysis.AIReviewPoint, 0, len(points))
	for _, point := range points {
		point.Severity = normalizeAISeverity(point.Severity)
		point.Suite = normalizeAISuite(point.Suite)
		point.Title = strings.TrimSpace(point.Title)
		point.Detail = strings.TrimSpace(point.Detail)
		point.Impact = strings.TrimSpace(point.Impact)
		point.Recommendation = strings.TrimSpace(point.Recommendation)
		point.ComponentID = strings.TrimSpace(point.ComponentID)
		point.ConnectorID = strings.TrimSpace(point.ConnectorID)
		if point.Title == "" || point.Detail == "" {
			continue
		}
		cleaned = append(cleaned, point)
	}
	return cleaned
}

func normalizeAISeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(severity))
	default:
		return "medium"
	}
}

func normalizeAISuite(suite string) string {
	switch strings.ToLower(strings.TrimSpace(suite)) {
	case "requirements", "topology", "traffic", "consistency", "availability", "security", "data", "operability", "cost", "ai", "integrity":
		return strings.ToLower(strings.TrimSpace(suite))
	default:
		return "integrity"
	}
}

func providerBaseURL(provider string, baseURL string) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint != "" {
		return endpoint, nil
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "https://api.openai.com/v1", nil
	case "anthropic":
		return "https://api.anthropic.com/v1", nil
	case "openrouter":
		return "https://openrouter.ai/api/v1", nil
	case "google":
		return "https://generativelanguage.googleapis.com/v1beta", nil
	default:
		return "", errors.New("base URL is required for custom providers")
	}
}
