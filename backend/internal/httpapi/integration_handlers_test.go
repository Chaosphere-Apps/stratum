package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
)

func TestAdminIntegrationSettingsRoundTripWithoutExposingSecrets(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		return recorder
	}

	telemetryBody := `{
		"enabled":true,
		"provider":"prometheus",
		"displayName":"Production Prometheus",
		"baseUrl":"https://prometheus.example.com",
		"authMode":"bearer",
		"secret":"telemetry-secret",
		"queryWindow":"10m",
		"requestTotalMetric":"http_requests_total",
		"filters":{"namespaces":["payments"],"minimumRequestsPerSec":0.5}
	}`
	updatedTelemetry := request(http.MethodPatch, "/api/admin/telemetry", telemetryBody)
	if updatedTelemetry.Code != http.StatusOK || !strings.Contains(updatedTelemetry.Body.String(), `"secretSet":true`) {
		t.Fatalf("update telemetry status=%d body=%q", updatedTelemetry.Code, updatedTelemetry.Body.String())
	}
	if strings.Contains(updatedTelemetry.Body.String(), "telemetry-secret") {
		t.Fatalf("telemetry secret leaked in update response: %q", updatedTelemetry.Body.String())
	}
	readTelemetry := request(http.MethodGet, "/api/admin/telemetry", "")
	if readTelemetry.Code != http.StatusOK || strings.Contains(readTelemetry.Body.String(), "telemetry-secret") || !strings.Contains(readTelemetry.Body.String(), "Production Prometheus") {
		t.Fatalf("get telemetry status=%d body=%q", readTelemetry.Code, readTelemetry.Body.String())
	}

	updatedAI := request(http.MethodPatch, "/api/admin/ai-provider", `{"enabled":false,"provider":"openai","model":"gpt-test","baseUrl":"https://api.openai.com"}`)
	if updatedAI.Code != http.StatusOK || !strings.Contains(updatedAI.Body.String(), `"model":"gpt-test"`) {
		t.Fatalf("update disabled AI provider status=%d body=%q", updatedAI.Code, updatedAI.Body.String())
	}
	readAI := request(http.MethodGet, "/api/admin/ai-provider", "")
	if readAI.Code != http.StatusOK || !strings.Contains(readAI.Body.String(), `"enabled":false`) {
		t.Fatalf("get AI provider status=%d body=%q", readAI.Code, readAI.Body.String())
	}
	invalidAI := request(http.MethodPatch, "/api/admin/ai-provider", `{"enabled":false,"provider":"openai","model":"gpt-test","apiKeySet":true}`)
	if invalidAI.Code != http.StatusBadRequest || !strings.Contains(invalidAI.Body.String(), `unsupported field \"apiKeySet\"`) {
		t.Fatalf("invalid AI provider response should explain the field status=%d body=%q", invalidAI.Code, invalidAI.Body.String())
	}
}

func TestVerifyGoogleAIProviderValidatesConfiguredModelAndAPIKeyHeader(t *testing.T) {
	var receivedPath string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		if r.Header.Get("x-goog-api-key") != "google-secret" {
			t.Fatalf("missing Google API key header")
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("Google key must not be sent in Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer provider.Close()

	if err := verifyAIProvider(t.Context(), "google", "gemini-test", provider.URL+"/v1beta", "google-secret", true); err != nil {
		t.Fatalf("verify Google provider: %v", err)
	}
	if receivedPath != "/v1beta/models/gemini-test" {
		t.Fatalf("unexpected verification path %q", receivedPath)
	}
}
