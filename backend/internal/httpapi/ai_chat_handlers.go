package httpapi

import (
	"encoding/json"
	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/modelgateway"
	"github.com/system-design-evaluator/backend/internal/policy"
	"net/http"
	"strings"
)

func (s *Server) handleListAIConversations(w http.ResponseWriter, r *http.Request) {
	workspaceID, designID := r.PathValue("workspaceID"), r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignRead); !ok {
		return
	}
	items, err := s.services.AIChat.ListConversations(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
}

func (s *Server) handleCreateAIConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID, designID := r.PathValue("workspaceID"), r.PathValue("designID")
	user, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignComment)
	if !ok {
		return
	}
	var body struct {
		Title     string `json:"title"`
		VersionID string `json:"versionId"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.VersionID) != "" {
		if _, err := s.designVersionByID(r.Context(), workspaceID, designID, body.VersionID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	conversation, err := s.services.AIChat.CreateConversation(r.Context(), domain.AIConversation{
		WorkspaceID: workspaceID,
		DesignID:    designID,
		VersionID:   strings.TrimSpace(body.VersionID),
		Title:       body.Title,
		CreatedBy:   user.ID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"conversation": conversation})
}

func (s *Server) handleListAIMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID, designID := r.PathValue("workspaceID"), r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignRead); !ok {
		return
	}
	conversation, err := s.services.AIChat.GetConversation(r.Context(), workspaceID, designID, r.PathValue("conversationID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	messages, err := s.services.AIChat.ListMessages(r.Context(), conversation.ID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": conversation, "messages": messages})
}

func (s *Server) handleCreateAIMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID, designID := r.PathValue("workspaceID"), r.PathValue("designID")
	user, design, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignComment)
	if !ok {
		return
	}
	if !s.allowAIAttempt(w, user.ID, designID) {
		return
	}
	conversation, err := s.services.AIChat.GetConversation(r.Context(), workspaceID, designID, r.PathValue("conversationID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	config, err := s.services.Identity.GetAIProviderConfigWithSecret(r.Context())
	if err != nil || !config.Enabled || strings.TrimSpace(config.APIKey) == "" {
		writeError(w, http.StatusServiceUnavailable, "AI provider is not configured")
		return
	}
	userMessage, err := s.services.AIChat.AddMessage(r.Context(), domain.AIMessage{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        body.Content,
		CreatedBy:      user.ID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	document := design.Document
	if conversation.VersionID != "" {
		version, versionErr := s.designVersionByID(r.Context(), workspaceID, designID, conversation.VersionID)
		if versionErr != nil {
			writeError(w, http.StatusConflict, "the conversation's design version is no longer available")
			return
		}
		document = version.Document
	}
	report, err := s.services.Analysis.AnalyzeDocument(document)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	history, err := s.services.AIChat.ListMessages(r.Context(), conversation.ID, 24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	contextMessage, err := analysis.BuildChatContext(document, report, conversation)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	messages := []modelgateway.Message{{Role: "user", Content: contextMessage}}
	for _, message := range history {
		messages = append(messages, modelgateway.Message{Role: message.Role, Content: message.Content})
	}
	content, err := s.ai.Complete(r.Context(), config, modelgateway.Request{
		System:       analysis.BuildChatSystemPrompt(),
		Messages:     messages,
		JSONResponse: true,
		MaxTokens:    1800,
	})
	if err != nil {
		s.log.Warn("AI chat completion failed", "provider", config.Provider, "model", config.Model, "error", err)
		writeError(w, http.StatusBadGateway, "AI response could not be generated; your message was saved")
		return
	}
	envelope, err := analysis.ParseChatEnvelope(content)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	envelope.References = validAIReferences(document, envelope.References)
	assistantMessage, err := s.services.AIChat.AddMessage(r.Context(), domain.AIMessage{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        envelope.Answer,
		References:     envelope.References,
		Provider:       config.Provider,
		Model:          config.Model,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"userMessage":      userMessage,
		"assistantMessage": assistantMessage,
		"followUps":        envelope.FollowUps,
	})
}

func validAIReferences(document json.RawMessage, references []domain.AIMessageReference) []domain.AIMessageReference {
	var graph struct {
		Components []struct{ ID, Name string } `json:"components"`
		Connectors []struct{ ID string }       `json:"connectors"`
	}
	if json.Unmarshal(document, &graph) != nil {
		return nil
	}
	components := map[string]string{}
	connectors := map[string]bool{}
	for _, component := range graph.Components {
		components[component.ID] = component.Name
	}
	for _, connector := range graph.Connectors {
		connectors[connector.ID] = true
	}
	valid := make([]domain.AIMessageReference, 0, len(references))
	for _, reference := range references {
		if reference.Kind == "component" {
			if name, ok := components[reference.ID]; ok {
				reference.Name = name
				valid = append(valid, reference)
			}
		} else if reference.Kind == "connector" && connectors[reference.ID] {
			valid = append(valid, reference)
		}
	}
	return valid
}
