package httpapi

import (
	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsAllowedOrigin(t *testing.T) {
	allowed := []string{"https://stratum.example.com", "http://localhost:*"}

	if !isAllowedOrigin("https://stratum.example.com", allowed) {
		t.Fatal("expected exact production origin to be allowed")
	}
	if !isAllowedOrigin("http://localhost:5173", allowed) {
		t.Fatal("expected wildcard localhost port to be allowed")
	}
	if isAllowedOrigin("https://evil.example.com", allowed) {
		t.Fatal("unexpectedly allowed unrelated origin")
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Workspace","extra":true}`))
	recorder := httptest.NewRecorder()

	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(recorder, request, 1024, &body); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"too large"}`))
	recorder := httptest.NewRecorder()

	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(recorder, request, 4, &body); err == nil {
		t.Fatal("expected oversized request body to be rejected")
	}
}

func TestJSONRequestErrorExplainsSafeValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		body string
		max  int64
		want string
	}{
		{name: "unknown field", body: `{"name":"Workspace","apiKeySet":true}`, max: 1024, want: `request contains unsupported field "apiKeySet"`},
		{name: "malformed", body: `{"name":`, max: 1024, want: "request body contains malformed JSON"},
		{name: "oversized", body: `{"name":"Workspace"}`, max: 4, want: "request body exceeds the allowed size"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			recorder := httptest.NewRecorder()
			var body struct {
				Name string `json:"name"`
			}
			err := decodeJSON(recorder, request, test.max, &body)
			if got := jsonRequestError(err); got != test.want {
				t.Fatalf("jsonRequestError() = %q, want %q (decode error: %v)", got, test.want, err)
			}
		})
	}
}

func TestServerErrorsRemainPrivateUnlessExplicitlyMarkedPublic(t *testing.T) {
	privateRecorder := httptest.NewRecorder()
	writeError(privateRecorder, http.StatusServiceUnavailable, "database password leaked")
	if strings.Contains(privateRecorder.Body.String(), "database password") || !strings.Contains(privateRecorder.Body.String(), "internal server error") {
		t.Fatalf("private server error was not sanitized: %q", privateRecorder.Body.String())
	}

	publicRecorder := httptest.NewRecorder()
	writePublicError(publicRecorder, http.StatusServiceUnavailable, "The AI provider is temporarily unavailable.")
	if !strings.Contains(publicRecorder.Body.String(), "temporarily unavailable") {
		t.Fatalf("trusted public error was hidden: %q", publicRecorder.Body.String())
	}
}

func TestValidateProviderEndpointRejectsPrivateAddress(t *testing.T) {
	if err := validateProviderEndpoint("http://127.0.0.1:11434", false); err == nil {
		t.Fatal("expected private provider URL to be rejected")
	}
	if err := validateProviderEndpoint("http://127.0.0.1:11434", true); err != nil {
		t.Fatalf("expected private provider URL to be allowed when explicitly configured: %v", err)
	}
}

func TestParseAIReviewNormalizesAndRejectsInvalidShape(t *testing.T) {
	config := domain.AIProviderConfig{Provider: "openai", Model: "gpt-4.1"}
	review := parseAIReview(`{
		"executiveReview": "  Looks reasonable.  ",
		"strengths": ["  clear path  ", ""],
		"risks": [
			{"severity":"critical","suite":"unknown","title":"  Missing DLQ  ","detail":"No retry sink","impact":"Lost events","recommendation":"Add DLQ"},
			{"severity":"low","suite":"data","title":"","detail":"ignored"}
		],
		"recommendations": [
			{"severity":"high","suite":"security","title":"Use TLS","detail":"Transport is unclear","impact":"Sensitive data exposure","recommendation":"Set https/grpc"}
		],
		"openQuestions": ["  owner?  ", ""]
	}`, config)
	if review.Status != "completed" || review.Provider != "openai" || review.Model != "gpt-4.1" || review.PromptVersion != analysis.PromptVersion {
		t.Fatalf("review metadata = %#v", review)
	}
	if review.ExecutiveReview != "Looks reasonable." || len(review.Strengths) != 1 || review.Strengths[0] != "clear path" {
		t.Fatalf("review text not cleaned: %#v", review)
	}
	if len(review.Risks) != 1 || review.Risks[0].Severity != "medium" || review.Risks[0].Suite != "integrity" || review.Risks[0].Title != "Missing DLQ" {
		t.Fatalf("risk normalization failed: %#v", review.Risks)
	}
	if len(review.Recommendations) != 1 || review.Recommendations[0].Severity != "high" || review.Recommendations[0].Suite != "security" {
		t.Fatalf("recommendation normalization failed: %#v", review.Recommendations)
	}
	if len(review.OpenQuestions) != 1 || review.OpenQuestions[0] != "owner?" {
		t.Fatalf("open questions not cleaned: %#v", review.OpenQuestions)
	}

	failed := parseAIReview(`not json`, config)
	if failed.Status != "failed" || failed.Error == "" || failed.PromptVersion != analysis.PromptVersion {
		t.Fatalf("invalid review should fail with prompt version, got %#v", failed)
	}
}

func TestStaticAssetsServeSPAAndFiles(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<div>stratum app</div>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.Mkdir(filepath.Join(staticDir, "assets"), 0o700); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "assets", "app.js"), []byte("console.log('stratum')"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := NewServer(config.Config{StaticAssetsDir: staticDir}, nil, config.Logger())

	indexRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(indexRecorder, httptest.NewRequest(http.MethodGet, "/workspaces/ws/designs/d1", nil))
	if indexRecorder.Code != http.StatusOK || !strings.Contains(indexRecorder.Body.String(), "stratum app") {
		t.Fatalf("expected SPA fallback, got status=%d body=%q", indexRecorder.Code, indexRecorder.Body.String())
	}

	assetRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(assetRecorder, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if assetRecorder.Code != http.StatusOK || !strings.Contains(assetRecorder.Body.String(), "console.log") {
		t.Fatalf("expected static asset, got status=%d body=%q", assetRecorder.Code, assetRecorder.Body.String())
	}
	csp := assetRecorder.Header().Get("Content-Security-Policy")
	for _, directive := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"connect-src 'self' ws: wss:",
		"frame-ancestors 'none'",
	} {
		if !strings.Contains(csp, directive) {
			t.Fatalf("CSP missing %q: %q", directive, csp)
		}
	}
}

func TestStaticAssetsDoNotMaskMissingAPI(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<div>stratum app</div>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	server := NewServer(config.Config{StaticAssetsDir: staticDir}, nil, config.Logger())
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "stratum app") {
		t.Fatalf("expected API 404 instead of SPA fallback, got status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
