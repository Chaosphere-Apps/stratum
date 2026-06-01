package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
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

func TestLoginSetsHttpOnlyCookieAndDoesNotReturnToken(t *testing.T) {
	repo := store.NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"password123"}`))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%q", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"token"`) {
		t.Fatalf("login response leaked token: %q", recorder.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "stratum_session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %#v, want secure HTTP-only browser cookie", sessionCookie)
	}

	profile := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	profile.AddCookie(sessionCookie)
	profileRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(profileRecorder, profile)
	if profileRecorder.Code != http.StatusOK {
		t.Fatalf("profile after login status = %d body=%q", profileRecorder.Code, profileRecorder.Body.String())
	}

	badLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"wrong"}`))
	badRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(badRecorder, badLogin)
	if badRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want %d", badRecorder.Code, http.StatusUnauthorized)
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
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

	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if _, err := repo.GetUserBySessionToken(t.Context(), token); err == nil {
		t.Fatal("session token should be deleted after logout")
	}
	var cleared bool
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "stratum_session" && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("logout did not clear session cookie: %#v", recorder.Result().Cookies())
	}
}

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

func TestWriteRoutesRequireExpectedRoles(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser member returned error: %v", err)
	}
	reviewer, err := repo.CreateUser(t.Context(), "Reviewer", "reviewer@example.com", "reviewer", "password123")
	if err != nil {
		t.Fatalf("CreateUser reviewer returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(t.Context())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(t.Context(), workspace.ID, "Role Test", []byte(`{"id":"design_role","title":"Role Test","components":[]}`), admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	if _, err := repo.GrantDesignAccess(t.Context(), domain.DesignAccess{
		WorkspaceID: design.WorkspaceID,
		DesignID:    design.ID,
		UserID:      reviewer.ID,
		CanRead:     true,
		CanReview:   true,
	}); err != nil {
		t.Fatalf("GrantDesignAccess returned error: %v", err)
	}
	memberToken, err := repo.CreateSession(t.Context(), member.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession member returned error: %v", err)
	}
	reviewerToken, err := repo.CreateSession(t.Context(), reviewer.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession reviewer returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	memberWrite := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs", strings.NewReader(`{"name":"Denied","document":{}}`))
	memberWrite.AddCookie(&http.Cookie{Name: "stratum_session", Value: memberToken})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, memberWrite)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("member create design status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	memberAnalyze := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/analysis", strings.NewReader(`{}`))
	memberAnalyze.AddCookie(&http.Cookie{Name: "stratum_session", Value: memberToken})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, memberAnalyze)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("member analyze design status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	reviewerAnalyze := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/analysis", strings.NewReader(`{}`))
	reviewerAnalyze.AddCookie(&http.Cookie{Name: "stratum_session", Value: reviewerToken})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, reviewerAnalyze)
	if recorder.Code != http.StatusOK {
		t.Fatalf("reviewer analyze design status = %d, want %d body=%q", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestAccessGrantRejectsEmptyPermissions(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(t.Context())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	if _, err := repo.GrantWorkspaceAccess(t.Context(), domain.WorkspaceAccess{
		WorkspaceID: workspace.ID,
		UserID:      admin.ID,
		CanManage:   true,
	}); err != nil {
		t.Fatalf("GrantWorkspaceAccess admin returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/access", strings.NewReader(`{"userId":"`+member.ID+`"}`))
	request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "at least one workspace permission") {
		t.Fatalf("empty workspace grant status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestAccessGroupGrantsAuthorizeWorkspaceAndDesign(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	workspace, err := repo.CreateWorkspace(t.Context(), "Platform")
	if err != nil {
		t.Fatalf("CreateWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(t.Context(), workspace.ID, "Payments", []byte(`{"id":"design_group_acl","title":"Payments","components":[]}`), admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	group, err := repo.CreateAccessGroup(t.Context(), domain.AccessGroup{Name: "Architects", OktaGroupName: "stratum-architects"})
	if err != nil {
		t.Fatalf("CreateAccessGroup returned error: %v", err)
	}
	if _, err := repo.ReplaceAccessGroupMembers(t.Context(), group.ID, []string{member.ID}); err != nil {
		t.Fatalf("ReplaceAccessGroupMembers returned error: %v", err)
	}
	if _, err := repo.GrantWorkspaceGroupAccess(t.Context(), domain.WorkspaceGroupAccess{
		WorkspaceID:     workspace.ID,
		GroupID:         group.ID,
		CanRead:         true,
		CanCreateDesign: true,
	}); err != nil {
		t.Fatalf("GrantWorkspaceGroupAccess returned error: %v", err)
	}
	if _, err := repo.GrantDesignGroupAccess(t.Context(), domain.DesignGroupAccess{
		WorkspaceID: design.WorkspaceID,
		DesignID:    design.ID,
		GroupID:     group.ID,
		CanRead:     true,
		CanReview:   true,
	}); err != nil {
		t.Fatalf("GrantDesignGroupAccess returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), member.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	createDesign := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs", strings.NewReader(`{"name":"Allowed","document":{}}`))
	createDesign.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, createDesign)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("group create design status = %d, want %d body=%q", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	analyze := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/analysis", strings.NewReader(`{}`))
	analyze.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, analyze)
	if recorder.Code != http.StatusOK {
		t.Fatalf("group analyze status = %d, want %d body=%q", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestAdminCanListUserAccessAcrossWorkspacesAndDesigns(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	workspace, err := repo.CreateWorkspace(t.Context(), "Platform")
	if err != nil {
		t.Fatalf("CreateWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(t.Context(), workspace.ID, "Payments", []byte(`{"id":"design_acl","title":"Payments","components":[]}`), admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	if _, err := repo.GrantWorkspaceAccess(t.Context(), domain.WorkspaceAccess{
		WorkspaceID: workspace.ID,
		UserID:      member.ID,
		CanRead:     true,
	}); err != nil {
		t.Fatalf("GrantWorkspaceAccess member returned error: %v", err)
	}
	if _, err := repo.GrantDesignAccess(t.Context(), domain.DesignAccess{
		WorkspaceID: design.WorkspaceID,
		DesignID:    design.ID,
		UserID:      member.ID,
		CanComment:  true,
		CanReview:   true,
	}); err != nil {
		t.Fatalf("GrantDesignAccess member returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodGet, "/api/admin/access/users/"+member.ID, nil)
	request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"scope":"workspace"`) || !strings.Contains(body, `"scope":"design"`) {
		t.Fatalf("user access summary status=%d body=%q", recorder.Code, body)
	}
}
