package httpapi

import (
	"fmt"
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

func TestSaveDesignDocumentRejectsStaleRevision(t *testing.T) {
	ctx := t.Context()
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(ctx, "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	design, err := repo.CreateDesign(ctx, domain.GuestWorkspaceID, "Payments", []byte(`{"title":"Payments","components":[]}`), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	token, err := repo.CreateSession(ctx, admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	target := fmt.Sprintf("/api/workspaces/%s/designs/%s/document", domain.GuestWorkspaceID, design.ID)

	save := func(title string, revision string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"document":{"id":%q,"title":%q,"components":[]},"baseRevision":%q}`, design.ID, title, revision)
		request := httptest.NewRequest(http.MethodPut, target, strings.NewReader(body))
		request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		return recorder
	}

	first := save("First editor", design.DocumentRevision)
	if first.Code != http.StatusOK {
		t.Fatalf("first save status=%d body=%q", first.Code, first.Body.String())
	}
	stale := save("Stale editor", design.DocumentRevision)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"documentRevision"`) {
		t.Fatalf("stale save status=%d body=%q", stale.Code, stale.Body.String())
	}
	stored, err := repo.GetDesign(ctx, domain.GuestWorkspaceID, design.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stored.Document), "First editor") || strings.Contains(string(stored.Document), "Stale editor") {
		t.Fatalf("stale save overwrote current document: %s", stored.Document)
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
