package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
)

func TestDesignDocumentationAndCommentsLifecycle(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	design, err := repo.CreateDesign(t.Context(), domain.GuestWorkspaceID, "Payments", []byte(`{"title":"Payments","components":[]}`), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	base := "/api/workspaces/" + design.WorkspaceID + "/designs/" + design.ID

	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		return recorder
	}

	created := request(http.MethodPost, base+"/docs", `{"title":"Overview","body":"Initial context","format":"markdown"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create doc status=%d body=%q", created.Code, created.Body.String())
	}
	var createdBody struct {
		Doc domain.DesignDoc `json:"doc"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatal(err)
	}
	if createdBody.Doc.ID == "" || createdBody.Doc.Title != "Overview" {
		t.Fatalf("created doc=%#v", createdBody.Doc)
	}

	updated := request(http.MethodPatch, base+"/docs/"+createdBody.Doc.ID, `{"title":"Architecture overview","body":"Updated context"}`)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), "Architecture overview") {
		t.Fatalf("update doc status=%d body=%q", updated.Code, updated.Body.String())
	}
	listed := request(http.MethodGet, base+"/docs", "")
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), "Architecture overview") {
		t.Fatalf("list docs status=%d body=%q", listed.Code, listed.Body.String())
	}

	comment := request(http.MethodPost, base+"/comments", `{"body":"Check failure handling","componentId":"service-1"}`)
	if comment.Code != http.StatusCreated || !strings.Contains(comment.Body.String(), "Check failure handling") {
		t.Fatalf("create comment status=%d body=%q", comment.Code, comment.Body.String())
	}
	comments := request(http.MethodGet, base+"/comments", "")
	if comments.Code != http.StatusOK || !strings.Contains(comments.Body.String(), `"componentId":"service-1"`) {
		t.Fatalf("list comments status=%d body=%q", comments.Code, comments.Body.String())
	}

	deleted := request(http.MethodDelete, base+"/docs/"+createdBody.Doc.ID, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete doc status=%d body=%q", deleted.Code, deleted.Body.String())
	}
	missing := request(http.MethodGet, base+"/docs/"+createdBody.Doc.ID, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("get deleted doc status=%d body=%q", missing.Code, missing.Body.String())
	}
}

func TestDesignDocumentationMutationsRequireEditAccess(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := repo.CreateUser(t.Context(), "Reader", "reader@example.com", "member", "password123")
	if err != nil {
		t.Fatal(err)
	}
	design, err := repo.CreateDesign(t.Context(), domain.GuestWorkspaceID, "Payments", []byte(`{"title":"Payments","components":[]}`), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantDesignAccess(t.Context(), domain.DesignAccess{
		WorkspaceID: design.WorkspaceID,
		DesignID:    design.ID,
		UserID:      reader.ID,
		CanRead:     true,
	}); err != nil {
		t.Fatal(err)
	}
	token, err := repo.CreateSession(t.Context(), reader.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+design.WorkspaceID+"/designs/"+design.ID+"/docs", strings.NewReader(`{"title":"Denied"}`))
	req.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("read-only doc creation status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestVersionReviewLifecycleThroughHTTP(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := repo.CreateUser(t.Context(), "Reviewer", "reviewer@example.com", "reviewer", "password123")
	if err != nil {
		t.Fatal(err)
	}
	unassigned, err := repo.CreateUser(t.Context(), "Other reviewer", "other-reviewer@example.com", "reviewer", "password123")
	if err != nil {
		t.Fatal(err)
	}
	design, err := repo.CreateDesign(t.Context(), domain.GuestWorkspaceID, "Payments", []byte(`{"title":"Payments","components":[]}`), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantDesignAccess(t.Context(), domain.DesignAccess{
		WorkspaceID: design.WorkspaceID,
		DesignID:    design.ID,
		UserID:      reviewer.ID,
		CanRead:     true,
		CanReview:   true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantDesignAccess(t.Context(), domain.DesignAccess{
		WorkspaceID: design.WorkspaceID,
		DesignID:    design.ID,
		UserID:      unassigned.ID,
		CanRead:     true,
		CanReview:   true,
	}); err != nil {
		t.Fatal(err)
	}
	adminToken, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	reviewerToken, err := repo.CreateSession(t.Context(), reviewer.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	unassignedToken, err := repo.CreateSession(t.Context(), unassigned.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	base := "/api/workspaces/" + design.WorkspaceID + "/designs/" + design.ID
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		return recorder
	}

	createdVersion := request(http.MethodPost, base+"/versions", `{"remarks":"Ready for review"}`, adminToken)
	if createdVersion.Code != http.StatusCreated {
		t.Fatalf("create version status=%d body=%q", createdVersion.Code, createdVersion.Body.String())
	}
	var versionBody struct {
		Version domain.DesignVersion `json:"version"`
	}
	if err := json.Unmarshal(createdVersion.Body.Bytes(), &versionBody); err != nil {
		t.Fatal(err)
	}
	if versionBody.Version.ID == "" || versionBody.Version.Status != "draft" {
		t.Fatalf("created version=%#v", versionBody.Version)
	}

	createdReview := request(http.MethodPost, base+"/reviews", `{"versionId":"`+versionBody.Version.ID+`","reviewerIds":["`+reviewer.ID+`"],"message":"Please review"}`, adminToken)
	if createdReview.Code != http.StatusCreated {
		t.Fatalf("create review status=%d body=%q", createdReview.Code, createdReview.Body.String())
	}
	var reviewBody struct {
		Reviews []domain.DesignReviewRequest `json:"reviews"`
	}
	if err := json.Unmarshal(createdReview.Body.Bytes(), &reviewBody); err != nil {
		t.Fatal(err)
	}
	if len(reviewBody.Reviews) != 1 || reviewBody.Reviews[0].Status != "requested" {
		t.Fatalf("created reviews=%#v", reviewBody.Reviews)
	}
	notificationResponse := request(http.MethodGet, "/api/notifications", "", reviewerToken)
	if notificationResponse.Code != http.StatusOK {
		t.Fatalf("list reviewer notifications status=%d body=%q", notificationResponse.Code, notificationResponse.Body.String())
	}
	var notificationBody struct {
		Notifications []domain.Notification `json:"notifications"`
	}
	if err := json.Unmarshal(notificationResponse.Body.Bytes(), &notificationBody); err != nil {
		t.Fatal(err)
	}
	if len(notificationBody.Notifications) != 1 || notificationBody.Notifications[0].UserID != reviewer.ID {
		t.Fatalf("review notifications=%#v", notificationBody.Notifications)
	}
	notificationID := notificationBody.Notifications[0].ID
	wrongOwner := request(http.MethodPost, "/api/notifications/"+notificationID+"/read", "", adminToken)
	if wrongOwner.Code != http.StatusNotFound {
		t.Fatalf("cross-user notification update status=%d body=%q", wrongOwner.Code, wrongOwner.Body.String())
	}
	markedRead := request(http.MethodPost, "/api/notifications/"+notificationID+"/read", "", reviewerToken)
	if markedRead.Code != http.StatusNoContent {
		t.Fatalf("mark notification read status=%d body=%q", markedRead.Code, markedRead.Body.String())
	}
	notificationsAfterRead := request(http.MethodGet, "/api/notifications", "", reviewerToken)
	if !strings.Contains(notificationsAfterRead.Body.String(), `"read":true`) {
		t.Fatalf("notification not marked read body=%q", notificationsAfterRead.Body.String())
	}
	unauthorizedReview := request(http.MethodPatch, base+"/reviews/"+reviewBody.Reviews[0].ID, `{"status":"approved","summary":"Not assigned"}`, unassignedToken)
	if unauthorizedReview.Code != http.StatusForbidden {
		t.Fatalf("unassigned reviewer update status=%d body=%q", unauthorizedReview.Code, unauthorizedReview.Body.String())
	}
	stillRequested := request(http.MethodGet, base+"/reviews", "", reviewerToken)
	if stillRequested.Code != http.StatusOK || !strings.Contains(stillRequested.Body.String(), `"status":"requested"`) || strings.Contains(stillRequested.Body.String(), "Not assigned") {
		t.Fatalf("unassigned reviewer changed review status=%d body=%q", stillRequested.Code, stillRequested.Body.String())
	}

	approved := request(http.MethodPatch, base+"/reviews/"+reviewBody.Reviews[0].ID, `{"status":"approved","summary":"Looks good"}`, reviewerToken)
	if approved.Code != http.StatusOK || !strings.Contains(approved.Body.String(), `"status":"approved"`) {
		t.Fatalf("approve review status=%d body=%q", approved.Code, approved.Body.String())
	}
	versions := request(http.MethodGet, base+"/versions", "", reviewerToken)
	if versions.Code != http.StatusOK || !strings.Contains(versions.Body.String(), `"status":"reviewed"`) {
		t.Fatalf("list reviewed versions status=%d body=%q", versions.Code, versions.Body.String())
	}

	promoted := request(http.MethodPatch, base+"/versions/"+versionBody.Version.ID, `{"status":"live"}`, adminToken)
	if promoted.Code != http.StatusOK || !strings.Contains(promoted.Body.String(), `"status":"live"`) {
		t.Fatalf("promote version status=%d body=%q", promoted.Code, promoted.Body.String())
	}
	deleteLive := request(http.MethodDelete, base+"/versions/"+versionBody.Version.ID, "", adminToken)
	if deleteLive.Code != http.StatusNoContent {
		t.Fatalf("delete live version status=%d body=%q", deleteLive.Code, deleteLive.Body.String())
	}
	afterDelete := request(http.MethodGet, base+"/versions", "", adminToken)
	if afterDelete.Code != http.StatusOK || strings.Contains(afterDelete.Body.String(), versionBody.Version.ID) {
		t.Fatalf("deleted version remained status=%d body=%q", afterDelete.Code, afterDelete.Body.String())
	}
}
