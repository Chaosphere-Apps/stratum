package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/modelgateway"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/security"
	"github.com/system-design-evaluator/backend/internal/store"
)

func TestDesignAIConversationPersistsGroundedMessages(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := repo.CreateWorkspace(t.Context(), "Platform", admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	document := []byte(`{"schemaVersion":"1","id":"design-ai","title":"Notifications","requirementBrief":{},"components":[{"id":"queue-1","name":"Delivery queue","type":"messaging.queue"}],"connectors":[]}`)
	design, err := repo.CreateDesign(t.Context(), workspace.ID, "Notifications", document, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateAIProviderConfig(t.Context(), domain.AIProviderConfig{Enabled: true, Provider: "openai", Model: "model-a"}, "secret"); err != nil {
		t.Fatal(err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	gateway := &fakeModelGateway{response: `{"answer":"Protect the queue consumer with bounded retries.","references":[{"kind":"component","id":"queue-1","name":"wrong name"},{"kind":"component","id":"missing","name":"invented"}],"followUps":["Inspect the DLQ"]}`}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	server.ai = gateway

	create := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/ai/conversations", strings.NewReader(`{"title":"Queue review"}`))
	create.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	createRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRecorder, create)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create conversation status=%d body=%q", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Conversation domain.AIConversation `json:"conversation"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Conversation.AccessMode != "read" {
		t.Fatalf("AI conversations must default to read-only, got %q", created.Conversation.AccessMode)
	}
	if !strings.HasPrefix(created.Conversation.ID, "chat_") {
		t.Fatalf("new conversation IDs should avoid repeating the resource name, got %q", created.Conversation.ID)
	}

	send := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/ai/conversations/"+created.Conversation.ID+"/messages", strings.NewReader(`{"content":"How should retries work?"}`))
	send.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	sendRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(sendRecorder, send)
	if sendRecorder.Code != http.StatusCreated {
		t.Fatalf("send message status=%d body=%q", sendRecorder.Code, sendRecorder.Body.String())
	}
	var sent struct {
		AssistantMessage domain.AIMessage `json:"assistantMessage"`
	}
	if err := json.Unmarshal(sendRecorder.Body.Bytes(), &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.AssistantMessage.References) != 1 || sent.AssistantMessage.References[0].Name != "Delivery queue" {
		t.Fatalf("assistant references were not grounded: %#v", sent.AssistantMessage.References)
	}
	if !strings.Contains(gateway.request.System, "untrusted data") || len(gateway.request.Messages) < 2 {
		t.Fatalf("gateway request lacks safety or history context: %#v", gateway.request)
	}
	listConversations := httptest.NewRequest(http.MethodGet, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/ai/conversations", nil)
	listConversations.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	listConversationsRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(listConversationsRecorder, listConversations)
	if listConversationsRecorder.Code != http.StatusOK || !strings.Contains(listConversationsRecorder.Body.String(), "Queue review") {
		t.Fatalf("list conversations status=%d body=%q", listConversationsRecorder.Code, listConversationsRecorder.Body.String())
	}
	listMessages := httptest.NewRequest(http.MethodGet, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/ai/conversations/"+created.Conversation.ID+"/messages", nil)
	listMessages.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	listMessagesRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(listMessagesRecorder, listMessages)
	if listMessagesRecorder.Code != http.StatusOK || !strings.Contains(listMessagesRecorder.Body.String(), "How should retries work?") || !strings.Contains(listMessagesRecorder.Body.String(), "bounded retries") {
		t.Fatalf("list messages status=%d body=%q", listMessagesRecorder.Code, listMessagesRecorder.Body.String())
	}
	messages, err := repo.ListAIMessages(t.Context(), created.Conversation.ID, 10)
	if err != nil || len(messages) != 2 {
		t.Fatalf("persisted messages=%#v err=%v", messages, err)
	}
}

func TestAIConversationWriteAccessRequiresDesignEditPermission(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, _ := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	viewer, _ := repo.CreateUser(t.Context(), "Viewer", "viewer@example.com", "member", "password123")
	workspace, _ := repo.CreateWorkspace(t.Context(), "Platform", admin.ID)
	design, _ := repo.CreateDesign(t.Context(), workspace.ID, "Secure design", []byte(`{"schemaVersion":"1","id":"secure","components":[],"connectors":[]}`), admin.ID)
	if _, err := repo.GrantDesignAccess(t.Context(), domain.DesignAccess{
		WorkspaceID: workspace.ID, DesignID: design.ID, UserID: viewer.ID, CanRead: true, CanComment: true,
	}); err != nil {
		t.Fatal(err)
	}
	token, _ := repo.CreateSession(t.Context(), viewer.ID, time.Now().Add(time.Hour))
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/ai/conversations", strings.NewReader(`{"title":"Unsafe","accessMode":"read_write"}`))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "edit access") {
		t.Fatalf("write access must be denied server-side: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestAIConversationAppliesValidatedWriteToolResult(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, _ := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	workspace, _ := repo.CreateWorkspace(t.Context(), "Platform", admin.ID)
	document := []byte(`{"schemaVersion":"1","id":"design-ai","title":"Payments","components":[{"id":"api","name":"API"}],"connectors":[]}`)
	design, _ := repo.CreateDesign(t.Context(), workspace.ID, "Payments", document, admin.ID)
	_, _ = repo.UpdateAIProviderConfig(t.Context(), domain.AIProviderConfig{Enabled: true, Provider: "openai", Model: "model-a"}, "secret")
	token, _ := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	gateway := &fakeModelGateway{response: `{"answer":"Added the database.","references":[],"followUps":[],"designUpdate":{"schemaVersion":"1","id":"design-ai","title":"Payments","components":[{"id":"api","name":"API"},{"id":"db","name":"Database"}],"connectors":[{"id":"api-db","fromComponentId":"api","toComponentId":"db"}]},"updateSummary":"Added a database dependency"}`}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	server.ai = gateway

	create := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/ai/conversations", strings.NewReader(`{"accessMode":"read_write"}`))
	create.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	createdRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(createdRecorder, create)
	if createdRecorder.Code != http.StatusCreated {
		t.Fatalf("create write conversation status=%d body=%q", createdRecorder.Code, createdRecorder.Body.String())
	}
	var created struct {
		Conversation domain.AIConversation `json:"conversation"`
	}
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	send := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspace.ID+"/designs/"+design.ID+"/ai/conversations/"+created.Conversation.ID+"/messages", strings.NewReader(`{"content":"Add a database"}`))
	send.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	sentRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(sentRecorder, send)
	if sentRecorder.Code != http.StatusCreated || !strings.Contains(sentRecorder.Body.String(), `"designUpdated":true`) {
		t.Fatalf("write result status=%d body=%q", sentRecorder.Code, sentRecorder.Body.String())
	}
	updated, err := repo.GetDesign(t.Context(), workspace.ID, design.ID)
	if err != nil || !strings.Contains(string(updated.Document), `"id":"db"`) {
		t.Fatalf("validated design update was not stored: document=%s err=%v", updated.Document, err)
	}
}

func TestValidateAIDesignUpdateProtectsIdentityAndGraphIntegrity(t *testing.T) {
	current := json.RawMessage(`{"schemaVersion":"1","id":"design-1","components":[{"id":"api"}],"connectors":[]}`)
	valid := json.RawMessage(`{"schemaVersion":"1","id":"design-1","components":[{"id":"api"},{"id":"db"}],"connectors":[{"id":"api-db","fromComponentId":"api","toComponentId":"db"}]}`)
	if err := validateAIDesignUpdate(current, valid, 1<<20); err != nil {
		t.Fatalf("valid update rejected: %v", err)
	}
	changedIdentity := json.RawMessage(`{"schemaVersion":"1","id":"other","components":[],"connectors":[]}`)
	if err := validateAIDesignUpdate(current, changedIdentity, 1<<20); err == nil {
		t.Fatal("expected identity change to be rejected")
	}
	danglingConnector := json.RawMessage(`{"schemaVersion":"1","id":"design-1","components":[{"id":"api"}],"connectors":[{"id":"bad","fromComponentId":"api","toComponentId":"missing"}]}`)
	if err := validateAIDesignUpdate(current, danglingConnector, 1<<20); err == nil {
		t.Fatal("expected dangling connector to be rejected")
	}
}

func TestPublicAIChatProviderErrorOnlyExposesTrustedMessages(t *testing.T) {
	status, message := publicAIChatProviderError(&modelgateway.ProviderHTTPError{StatusCode: http.StatusServiceUnavailable})
	if status != http.StatusServiceUnavailable || !strings.Contains(message, "temporarily unavailable") {
		t.Fatalf("unexpected transient provider response: status=%d message=%q", status, message)
	}
	status, message = publicAIChatProviderError(errors.New("database password: do not expose"))
	if status != http.StatusBadGateway || strings.Contains(message, "database password") {
		t.Fatalf("raw backend error escaped sanitization: status=%d message=%q", status, message)
	}
}

func TestBoundedAIChatHistoryKeepsNewestContextWithinBudget(t *testing.T) {
	history := make([]domain.AIMessage, 0, 20)
	for index := 0; index < 20; index++ {
		history = append(history, domain.AIMessage{ID: string(rune('a' + index)), Role: "assistant", Content: strings.Repeat("x", 2000)})
	}
	bounded := boundedAIChatHistory(history, 4, 4500)
	if len(bounded) != 3 || bounded[len(bounded)-1].ID != history[len(history)-1].ID {
		t.Fatalf("newest history was not retained: %#v", bounded)
	}
	if len([]rune(bounded[0].Content)) > 600 {
		t.Fatalf("oldest retained message should be truncated to the remaining budget, got %d runes", len([]rune(bounded[0].Content)))
	}
}

func TestAIChatRateLimitIsScopedAndReturnsRetryAfter(t *testing.T) {
	server := &Server{aiRate: security.NewRateLimiter()}
	for attempt := 0; attempt < 30; attempt++ {
		recorder := httptest.NewRecorder()
		if !server.allowAIAttempt(recorder, "user-1", "design-1") {
			t.Fatalf("attempt %d was unexpectedly rejected", attempt+1)
		}
	}

	recorder := httptest.NewRecorder()
	if server.allowAIAttempt(recorder, "user-1", "design-1") {
		t.Fatal("expected the 31st request to be rejected")
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d want=%d", recorder.Code, http.StatusTooManyRequests)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}

	otherDesign := httptest.NewRecorder()
	if !server.allowAIAttempt(otherDesign, "user-1", "design-2") {
		t.Fatal("a different design should use a separate rate-limit bucket")
	}
}
