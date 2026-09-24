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

func TestAccessGroupLifecycleThroughHTTP(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetOrCreateGuestWorkspace(t.Context()); err != nil {
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

	created := request(http.MethodPost, "/api/admin/access/groups", `{"name":"Platform architects","description":"Architecture owners","oktaGroupName":"platform-architects"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create group status=%d body=%q", created.Code, created.Body.String())
	}
	var createdBody struct {
		Group domain.AccessGroup `json:"group"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatal(err)
	}
	groupID := createdBody.Group.ID
	if groupID == "" {
		t.Fatalf("created group=%#v", createdBody.Group)
	}

	updated := request(http.MethodPatch, "/api/admin/access/groups/"+groupID, `{"name":"Platform architecture","description":"Updated","oktaGroupName":"platform-architecture"}`)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), "Platform architecture") {
		t.Fatalf("update group status=%d body=%q", updated.Code, updated.Body.String())
	}
	members := request(http.MethodPut, "/api/admin/access/groups/"+groupID+"/members", `{"userIds":["`+member.ID+`"]}`)
	if members.Code != http.StatusOK || !strings.Contains(members.Body.String(), member.ID) {
		t.Fatalf("replace group members status=%d body=%q", members.Code, members.Body.String())
	}
	listedMembers := request(http.MethodGet, "/api/admin/access/groups/"+groupID+"/members", "")
	if listedMembers.Code != http.StatusOK || !strings.Contains(listedMembers.Body.String(), member.ID) {
		t.Fatalf("list group members status=%d body=%q", listedMembers.Code, listedMembers.Body.String())
	}

	grant := request(http.MethodPost, "/api/workspaces/"+domain.GuestWorkspaceID+"/group-access", `{"groupId":"`+groupID+`","canRead":true,"canCreateDesign":true}`)
	if grant.Code != http.StatusOK {
		t.Fatalf("grant workspace group access status=%d body=%q", grant.Code, grant.Body.String())
	}
	revoke := request(http.MethodDelete, "/api/workspaces/"+domain.GuestWorkspaceID+"/group-access/"+groupID, "")
	if revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke workspace group access status=%d body=%q", revoke.Code, revoke.Body.String())
	}
	deleted := request(http.MethodDelete, "/api/admin/access/groups/"+groupID, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete group status=%d body=%q", deleted.Code, deleted.Body.String())
	}
	groups := request(http.MethodGet, "/api/admin/access/groups", "")
	if groups.Code != http.StatusOK || strings.Contains(groups.Body.String(), groupID) {
		t.Fatalf("deleted group remained status=%d body=%q", groups.Code, groups.Body.String())
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

	createDesign := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs", strings.NewReader(`{"name":"Allowed"}`))
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

func TestRevokingGroupDesignAccessImmediatelyRemovesDesignFromReadsAndListing(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "password123")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := repo.CreateWorkspace(t.Context(), "Platform", "")
	if err != nil {
		t.Fatal(err)
	}
	design, err := repo.CreateDesign(t.Context(), workspace.ID, "Payments", []byte(`{"title":"Payments","components":[]}`), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	group, err := repo.CreateAccessGroup(t.Context(), domain.AccessGroup{Name: "Architects"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceAccessGroupMembers(t.Context(), group.ID, []string{member.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantWorkspaceGroupAccess(t.Context(), domain.WorkspaceGroupAccess{WorkspaceID: workspace.ID, GroupID: group.ID, CanRead: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantDesignGroupAccess(t.Context(), domain.DesignGroupAccess{WorkspaceID: workspace.ID, DesignID: design.ID, GroupID: group.ID, CanRead: true}); err != nil {
		t.Fatal(err)
	}
	memberToken, err := repo.CreateSession(t.Context(), member.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	base := "/api/workspaces/" + workspace.ID + "/designs"
	request := func(method, path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		return recorder
	}
	if got := request(http.MethodGet, base+"/"+design.ID, memberToken); got.Code != http.StatusOK {
		t.Fatalf("granted design read status=%d body=%q", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, base, memberToken); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), design.ID) {
		t.Fatalf("granted design list status=%d body=%q", got.Code, got.Body.String())
	}
	if got := request(http.MethodDelete, base+"/"+design.ID+"/group-access/"+group.ID, adminToken); got.Code != http.StatusNoContent {
		t.Fatalf("revoke design group access status=%d body=%q", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, base+"/"+design.ID, memberToken); got.Code != http.StatusForbidden {
		t.Fatalf("revoked design read status=%d body=%q", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, base, memberToken); got.Code != http.StatusOK || strings.Contains(got.Body.String(), design.ID) {
		t.Fatalf("revoked design remained visible in listing: status=%d body=%q", got.Code, got.Body.String())
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
