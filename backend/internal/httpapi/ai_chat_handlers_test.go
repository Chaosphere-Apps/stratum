package httpapi

import (
	"encoding/json"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/security"
	"github.com/system-design-evaluator/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	messages, err := repo.ListAIMessages(t.Context(), created.Conversation.ID, 10)
	if err != nil || len(messages) != 2 {
		t.Fatalf("persisted messages=%#v err=%v", messages, err)
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
