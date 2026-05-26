package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
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

func TestValidateProviderEndpointRejectsPrivateAddress(t *testing.T) {
	if err := validateProviderEndpoint("http://127.0.0.1:11434", false); err == nil {
		t.Fatal("expected private provider URL to be rejected")
	}
	if err := validateProviderEndpoint("http://127.0.0.1:11434", true); err != nil {
		t.Fatalf("expected private provider URL to be allowed when explicitly configured: %v", err)
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

func TestProfileRequiresSessionAfterUsersExist(t *testing.T) {
	repo := store.NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/profile", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("profile without session status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestProfileReturnsUserForValidSession(t *testing.T) {
	repo := store.NewMemoryRepository()
	user, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), user.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), user.Email) {
		t.Fatalf("profile with session returned status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestProductDataRoutesRequireSession(t *testing.T) {
	repo := store.NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	for _, target := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/workspaces"},
		{http.MethodPost, "/api/workspaces"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs"},
		{http.MethodPut, "/api/workspaces/guest-workspace/designs/design_1/document"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs/design_1/docs"},
	} {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(target.method, target.path, strings.NewReader(`{}`)))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", target.method, target.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}
