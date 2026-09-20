package httpapi

import (
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
	workspace, err := repo.CreateWorkspace(t.Context(), "Platform", "")
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
	workspace, err := repo.CreateWorkspace(t.Context(), "Platform", "")
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
