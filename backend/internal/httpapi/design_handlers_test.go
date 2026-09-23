package httpapi

import (
	"encoding/json"
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
		{http.MethodGet, "/api/profile"},
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/api/admin/users"},
		{http.MethodGet, "/api/admin/sign-in"},
		{http.MethodGet, "/api/admin/ai-provider"},
		{http.MethodGet, "/api/admin/mcp"},
		{http.MethodGet, "/api/admin/telemetry"},
		{http.MethodGet, "/api/admin/storage"},
		{http.MethodGet, "/api/admin/access/groups"},
		{http.MethodGet, "/api/catalog/assets"},
		{http.MethodGet, "/api/notifications"},
		{http.MethodPost, "/api/ai/provider/verify"},
		{http.MethodGet, "/api/workspace-share-principals"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs"},
		{http.MethodGet, "/api/workspaces/guest-workspace/access"},
		{http.MethodGet, "/api/workspaces/guest-workspace/group-access"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs/design_1"},
		{http.MethodPut, "/api/workspaces/guest-workspace/designs/design_1/document"},
		{http.MethodPost, "/api/workspaces/guest-workspace/designs/design_1/analysis"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs/design_1/versions"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs/design_1/docs"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs/design_1/comments"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs/design_1/reviews"},
		{http.MethodGet, "/api/workspaces/guest-workspace/designs/design_1/ai/conversations"},
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

func TestDesignMetadataAndDeletionLifecycle(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	design, err := repo.CreateDesign(t.Context(), domain.GuestWorkspaceID, "Original", []byte(`{"title":"Original","components":[]}`), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	path := "/api/workspaces/" + design.WorkspaceID + "/designs/" + design.ID
	request := func(method, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		return recorder
	}

	updated := request(http.MethodPatch, `{"name":"Payments v2","access":"private"}`)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"name":"Payments v2"`) || !strings.Contains(updated.Body.String(), `"access":"private"`) {
		t.Fatalf("update metadata status=%d body=%q", updated.Code, updated.Body.String())
	}
	fetched := request(http.MethodGet, "")
	if fetched.Code != http.StatusOK || !strings.Contains(fetched.Body.String(), `"name":"Payments v2"`) {
		t.Fatalf("get updated design status=%d body=%q", fetched.Code, fetched.Body.String())
	}
	deleted := request(http.MethodDelete, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete design status=%d body=%q", deleted.Code, deleted.Body.String())
	}
	missing := request(http.MethodGet, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("get deleted design status=%d body=%q", missing.Code, missing.Body.String())
	}
}

func TestDesignNameLengthBoundary(t *testing.T) {
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

	boundaryName := strings.Repeat("界", domain.MaxDesignNameLength)
	created := request(http.MethodPost, "/api/workspaces/"+domain.GuestWorkspaceID+"/designs", fmt.Sprintf(`{"name":%q,"document":{}}`, boundaryName))
	if created.Code != http.StatusCreated {
		t.Fatalf("boundary name create status=%d body=%q", created.Code, created.Body.String())
	}
	overLimitName := strings.Repeat("a", domain.MaxDesignNameLength+1)
	rejected := request(http.MethodPost, "/api/workspaces/"+domain.GuestWorkspaceID+"/designs", fmt.Sprintf(`{"name":%q,"document":{}}`, overLimitName))
	if rejected.Code != http.StatusBadRequest || !strings.Contains(rejected.Body.String(), "120 characters or fewer") {
		t.Fatalf("over-limit name create status=%d body=%q", rejected.Code, rejected.Body.String())
	}

	design, err := repo.CreateDesign(t.Context(), domain.GuestWorkspaceID, "Existing", []byte(`{"title":"Existing","components":[]}`), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	metadataRejected := request(http.MethodPatch, "/api/workspaces/"+design.WorkspaceID+"/designs/"+design.ID, fmt.Sprintf(`{"name":%q}`, overLimitName))
	if metadataRejected.Code != http.StatusBadRequest || !strings.Contains(metadataRejected.Body.String(), "120 characters or fewer") {
		t.Fatalf("over-limit metadata status=%d body=%q", metadataRejected.Code, metadataRejected.Body.String())
	}
}

func TestCreateDesignCoversUserSuccessAndValidationFailures(t *testing.T) {
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
	path := "/api/workspaces/" + domain.GuestWorkspaceID + "/designs"
	request := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		return recorder
	}

	created := request(`{"name":"  Checkout platform  ","document":{"schemaVersion":"sde-ui/v0.1","id":"client-design","title":"Checkout platform","components":[],"connectors":[],"journeys":[]}}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("valid create status=%d body=%q", created.Code, created.Body.String())
	}
	var response struct {
		Design domain.Design `json:"design"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Design.Name != "Checkout platform" || response.Design.Access != "private" || response.Design.DocumentRevision == "" {
		t.Fatalf("created design did not preserve normalized user intent: %#v", response.Design)
	}
	grants, err := repo.ListDesignAccess(t.Context(), domain.GuestWorkspaceID, response.Design.ID)
	if err != nil || len(grants) != 1 || grants[0].UserID != admin.ID || !grants[0].CanManage || !grants[0].CanEdit {
		t.Fatalf("creator did not receive full design access: grants=%#v err=%v", grants, err)
	}

	for _, scenario := range []struct {
		name string
		body string
		want string
	}{
		{name: "blank required name", body: `{"name":"   ","document":{}}`, want: "design name is required"},
		{name: "non-object document", body: `{"name":"Bad document","document":["not","a","design"]}`, want: "JSON object"},
		{name: "unknown request field", body: `{"name":"Unexpected","document":{},"ownerRole":"admin"}`, want: "invalid request body"},
		{name: "malformed JSON", body: `{"name":"Broken"`, want: "invalid request body"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			rejected := request(scenario.body)
			if rejected.Code != http.StatusBadRequest || !strings.Contains(rejected.Body.String(), scenario.want) {
				t.Fatalf("status=%d body=%q, want 400 containing %q", rejected.Code, rejected.Body.String(), scenario.want)
			}
		})
	}

	designs, err := repo.ListDesigns(t.Context(), domain.GuestWorkspaceID)
	if err != nil || len(designs) != 1 {
		t.Fatalf("rejected creates must not persist designs: count=%d err=%v", len(designs), err)
	}
}
