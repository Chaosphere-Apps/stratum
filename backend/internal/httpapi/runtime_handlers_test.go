package httpapi

import (
	"encoding/json"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSetupStatusIncludesStatelessStorageStatus(t *testing.T) {
	engine, err := store.NewStorageEngine(t.Context(), "")
	if err != nil {
		t.Fatalf("NewStorageEngine returned error: %v", err)
	}
	defer engine.Close()

	server := NewServer(config.Config{}, realtime.NewHub(engine.Repository(), config.Logger()), config.Logger(), engine)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/setup/status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Storage struct {
			Mode      string `json:"mode"`
			Stateless bool   `json:"stateless"`
			Warning   string `json:"warning"`
		} `json:"storage"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Storage.Mode != store.StorageModeStateless || !body.Storage.Stateless || body.Storage.Warning == "" {
		t.Fatalf("storage status = %#v, want stateless warning status", body.Storage)
	}

	for _, endpoint := range []string{"/healthz", "/readyz", "/api/storage/status"} {
		recorder = httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%q", endpoint, recorder.Code, recorder.Body.String())
		}
	}
}

func TestAdminStorageRoutesRequireAdminAndValidateInput(t *testing.T) {
	engine, err := store.NewStorageEngine(t.Context(), "")
	if err != nil {
		t.Fatalf("NewStorageEngine returned error: %v", err)
	}
	defer engine.Close()
	repo := engine.Repository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	adminToken, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession admin returned error: %v", err)
	}
	memberToken, err := repo.CreateSession(t.Context(), member.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession member returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger(), engine)

	memberRequest := httptest.NewRequest(http.MethodGet, "/api/admin/storage", nil)
	memberRequest.AddCookie(&http.Cookie{Name: "stratum_session", Value: memberToken})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, memberRequest)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("member admin storage status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	adminRequest := httptest.NewRequest(http.MethodGet, "/api/admin/storage", nil)
	adminRequest.AddCookie(&http.Cookie{Name: "stratum_session", Value: adminToken})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, adminRequest)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"stateless":true`) {
		t.Fatalf("admin storage status=%d body=%q", recorder.Code, recorder.Body.String())
	}

	blankTest := httptest.NewRequest(http.MethodPost, "/api/admin/storage/test", strings.NewReader(`{"databaseUrl":" "}`))
	blankTest.AddCookie(&http.Cookie{Name: "stratum_session", Value: adminToken})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, blankTest)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "database url is required") {
		t.Fatalf("blank storage test status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestAdminMCPRoutesRequireAdminAndPersistConfig(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	adminToken, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession admin returned error: %v", err)
	}
	memberToken, err := repo.CreateSession(t.Context(), member.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession member returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	memberRequest := httptest.NewRequest(http.MethodGet, "/api/admin/mcp", nil)
	memberRequest.AddCookie(&http.Cookie{Name: "stratum_session", Value: memberToken})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, memberRequest)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("member mcp status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	updateBody := `{
		"enabled": true,
		"endpointPath": "internal-mcp",
		"readCatalog": true,
		"readDesigns": false,
		"createDraftDesign": true,
		"runAnalysis": true,
		"fetchImpactReport": true,
		"requireAdminConsent": false
	}`
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/admin/mcp", strings.NewReader(updateBody))
	updateRequest.AddCookie(&http.Cookie{Name: "stratum_session", Value: adminToken})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, updateRequest)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update mcp status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	var response struct {
		MCP domain.MCPConfig `json:"mcp"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode mcp response: %v", err)
	}
	if !response.MCP.Enabled || response.MCP.EndpointPath != "/internal-mcp" || response.MCP.ReadDesigns || response.MCP.RequireAdminConsent {
		t.Fatalf("updated mcp config = %#v", response.MCP)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/admin/mcp", nil)
	getRequest.AddCookie(&http.Cookie{Name: "stratum_session", Value: adminToken})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, getRequest)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"/internal-mcp"`) {
		t.Fatalf("get mcp status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
