package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/modelgateway"
	"github.com/system-design-evaluator/backend/internal/policy"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
)

const (
	maxAIChatMessageRunes = 8000
	maxAIChatHistoryRunes = 16000
	maxAIChatHistoryItems = 12
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
	user, design, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignComment)
	if !ok {
		return
	}
	var body struct {
		Title      string `json:"title"`
		VersionID  string `json:"versionId"`
		AccessMode string `json:"accessMode"`
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
	accessMode := normalizeAIChatAccessMode(body.AccessMode)
	if accessMode == "read_write" {
		if strings.TrimSpace(body.VersionID) != "" {
			writeError(w, http.StatusBadRequest, "saved-version conversations are read-only")
			return
		}
		if !s.authz.CanAccessDesign(r.Context(), user, design, policy.DesignEdit) {
			writeError(w, http.StatusForbidden, "design edit access is required for AI write tools")
			return
		}
	}
	conversation, err := s.services.AIChat.CreateConversation(r.Context(), domain.AIConversation{
		WorkspaceID: workspaceID,
		DesignID:    designID,
		VersionID:   strings.TrimSpace(body.VersionID),
		Title:       body.Title,
		AccessMode:  accessMode,
		CreatedBy:   user.ID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"conversation": conversation})
}

func (s *Server) handleUpdateAIConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID, designID := r.PathValue("workspaceID"), r.PathValue("designID")
	user, design, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignComment)
	if !ok {
		return
	}
	conversation, err := s.services.AIChat.GetConversation(r.Context(), workspaceID, designID, r.PathValue("conversationID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var body struct {
		AccessMode string `json:"accessMode"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, jsonRequestError(err))
		return
	}
	accessMode := normalizeAIChatAccessMode(body.AccessMode)
	if accessMode == "read_write" {
		if conversation.VersionID != "" {
			writeError(w, http.StatusBadRequest, "saved-version conversations are read-only")
			return
		}
		if !s.authz.CanAccessDesign(r.Context(), user, design, policy.DesignEdit) {
			writeError(w, http.StatusForbidden, "design edit access is required for AI write tools")
			return
		}
	}
	conversation, err = s.services.AIChat.UpdateConversationAccess(r.Context(), workspaceID, designID, conversation.ID, accessMode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": conversation})
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
	if conversation.AccessMode == "read_write" {
		if conversation.VersionID != "" || !s.authz.CanAccessDesign(r.Context(), user, design, policy.DesignEdit) {
			writeError(w, http.StatusForbidden, "AI write tools are not permitted for this conversation")
			return
		}
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Content = strings.TrimSpace(body.Content)
	if body.Content == "" {
		writeError(w, http.StatusBadRequest, "AI message content is required")
		return
	}
	if len([]rune(body.Content)) > maxAIChatMessageRunes {
		writeError(w, http.StatusBadRequest, "AI message content must be 8000 characters or fewer")
		return
	}
	config, err := s.services.Identity.GetAIProviderConfigWithSecret(r.Context())
	if err != nil || !config.Enabled || strings.TrimSpace(config.APIKey) == "" {
		writePublicError(w, http.StatusServiceUnavailable, "AI provider is not configured. Ask an administrator to configure and verify it.")
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
	history, err := s.services.AIChat.ListMessages(r.Context(), conversation.ID, maxAIChatHistoryItems+4)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	history = boundedAIChatHistory(history, maxAIChatHistoryItems, maxAIChatHistoryRunes)
	contextMessage, err := analysis.BuildChatContext(document, report, conversation, conversation.AccessMode, len(history) > 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	messages := []modelgateway.Message{{Role: "user", Content: contextMessage}}
	for _, message := range history {
		messages = append(messages, modelgateway.Message{Role: message.Role, Content: message.Content})
	}
	content, err := s.ai.Complete(r.Context(), config, modelgateway.Request{
		System:       analysis.BuildChatSystemPrompt(conversation.AccessMode),
		Messages:     messages,
		JSONResponse: true,
		MaxTokens:    map[bool]int{true: 6000, false: 1800}[conversation.AccessMode == "read_write"],
	})
	if err != nil {
		s.log.Warn("AI chat completion failed", "provider", config.Provider, "model", config.Model, "error", err)
		status, message := publicAIChatProviderError(err)
		writePublicError(w, status, message)
		return
	}
	envelope, err := analysis.ParseChatEnvelope(content)
	if err != nil {
		s.log.Warn("AI chat response rejected", "provider", config.Provider, "model", config.Model, "reason", err)
		writePublicError(w, http.StatusBadGateway, "The AI provider returned an invalid response. Your message was saved; try again or ask an administrator to verify the configured model.")
		return
	}
	envelope.References = validAIReferences(document, envelope.References)
	var proposedDesign json.RawMessage
	proposalRevision := ""
	if len(envelope.DesignUpdate) > 0 && string(envelope.DesignUpdate) != "null" {
		if conversation.AccessMode != "read_write" {
			writePublicError(w, http.StatusBadGateway, "AI returned a design update without write access. No changes were applied.")
			return
		}
		if err := domain.ValidateAIDesignUpdate(document, envelope.DesignUpdate, s.cfg.MaxRequestBodyBytes); err != nil {
			s.log.Warn("AI design update rejected", "workspace_id", workspaceID, "design_id", designID, "conversation_id", conversation.ID, "reason", err)
			writePublicError(w, http.StatusBadGateway, "AI returned a design update that failed safety validation. No changes were applied.")
			return
		}
		proposedDesign = envelope.DesignUpdate
		proposalRevision = design.DocumentRevision
	}
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
		"proposedDesign":   proposedDesign,
		"baseRevision":     proposalRevision,
		"updateSummary":    envelope.UpdateSummary,
	})
}

func (s *Server) handleApplyAIDesignProposal(w http.ResponseWriter, r *http.Request) {
	workspaceID, designID := r.PathValue("workspaceID"), r.PathValue("designID")
	user, design, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignEdit)
	if !ok {
		return
	}
	conversation, err := s.services.AIChat.GetConversation(r.Context(), workspaceID, designID, r.PathValue("conversationID"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if conversation.AccessMode != "read_write" || conversation.VersionID != "" {
		writeError(w, http.StatusForbidden, "AI write tools are not permitted for this conversation")
		return
	}
	var body struct {
		Document     json.RawMessage `json:"document"`
		BaseRevision string          `json:"baseRevision"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, jsonRequestError(err))
		return
	}
	if strings.TrimSpace(body.BaseRevision) == "" {
		writeError(w, http.StatusBadRequest, "base revision is required")
		return
	}
	if err := domain.ValidateAIDesignUpdate(design.Document, body.Document, s.cfg.MaxRequestBodyBytes); err != nil {
		writeError(w, http.StatusBadRequest, "proposed design failed validation")
		return
	}
	updated, err := s.services.Designs.UpdateDocument(r.Context(), workspaceID, designID, body.Document, design.CanvasSnapshot, body.BaseRevision)
	if err != nil {
		if errors.Is(err, store.ErrDesignConflict) {
			writeError(w, http.StatusConflict, "the design changed since this AI proposal was created; request a new proposal")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.hub != nil {
		if encoded, encodeErr := json.Marshal(realtime.DesignUpdatedPayload{Design: updated}); encodeErr == nil {
			s.hub.Broadcast(r.Context(), workspaceID, realtime.Envelope{Type: realtime.MessageDesignUpdated, Payload: encoded})
		}
	}
	s.log.Info("AI design proposal applied", "user_id", user.ID, "workspace_id", workspaceID, "design_id", designID, "conversation_id", conversation.ID, "document_revision", updated.DocumentRevision)
	writeJSON(w, http.StatusOK, map[string]any{"design": updated})
}

func boundedAIChatHistory(history []domain.AIMessage, maxMessages int, maxRunes int) []domain.AIMessage {
	if maxMessages <= 0 || maxRunes <= 0 || len(history) == 0 {
		return nil
	}
	selected := make([]domain.AIMessage, 0, min(maxMessages, len(history)))
	remaining := maxRunes
	for index := len(history) - 1; index >= 0 && len(selected) < maxMessages && remaining > 0; index-- {
		message := history[index]
		content := []rune(message.Content)
		if len(content) > remaining {
			content = content[:remaining]
			message.Content = string(content) + "\n[earlier content truncated]"
		}
		remaining -= min(len(content), remaining)
		selected = append(selected, message)
	}
	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}
	return selected
}

func publicAIChatProviderError(err error) (int, string) {
	message := "The AI provider could not complete the request. Your message was saved; retry after checking the provider and model configuration."
	status := http.StatusBadGateway
	var providerError *modelgateway.ProviderHTTPError
	if !errors.As(err, &providerError) {
		if err != nil && err.Error() == "provider could not be reached" {
			return http.StatusServiceUnavailable, "The AI provider could not be reached. Your message was saved; try again shortly."
		}
		return status, message
	}
	switch providerError.StatusCode {
	case http.StatusTooManyRequests:
		return http.StatusServiceUnavailable, "The AI provider rate limit was reached. Your message was saved; try again shortly."
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return http.StatusServiceUnavailable, "The AI provider is temporarily unavailable. Your message was saved; try again shortly."
	case http.StatusUnauthorized, http.StatusForbidden:
		return status, "The AI provider rejected its configured credentials. Your message was saved; ask an administrator to verify the AI configuration."
	case http.StatusBadRequest, http.StatusNotFound:
		return status, "The AI provider rejected the configured model or request. Your message was saved; ask an administrator to verify the provider model."
	default:
		return status, message
	}
}

func normalizeAIChatAccessMode(accessMode string) string {
	if strings.EqualFold(strings.TrimSpace(accessMode), "read_write") {
		return "read_write"
	}
	return "read"
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
