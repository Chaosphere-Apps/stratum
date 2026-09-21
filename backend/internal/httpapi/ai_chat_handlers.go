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
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	envelope.References = validAIReferences(document, envelope.References)
	var updatedDesign *domain.Design
	if len(envelope.DesignUpdate) > 0 && string(envelope.DesignUpdate) != "null" {
		if conversation.AccessMode != "read_write" {
			writeError(w, http.StatusBadGateway, "AI attempted a design update without write-tool permission")
			return
		}
		if err := validateAIDesignUpdate(document, envelope.DesignUpdate, s.cfg.MaxRequestBodyBytes); err != nil {
			writeError(w, http.StatusBadGateway, "AI proposed an unsafe design update: "+err.Error())
			return
		}
		updated, updateErr := s.services.Designs.UpdateDocument(r.Context(), workspaceID, designID, envelope.DesignUpdate, design.CanvasSnapshot, design.DocumentRevision)
		if updateErr != nil {
			writeError(w, http.StatusConflict, "AI design update was not applied: "+updateErr.Error())
			return
		}
		updatedDesign = &updated
		s.log.Info("AI design tool applied", "user_id", user.ID, "workspace_id", workspaceID, "design_id", designID, "conversation_id", conversation.ID, "document_revision", updated.DocumentRevision)
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
		"designUpdated":    updatedDesign != nil,
		"updatedDesign":    updatedDesign,
		"updateSummary":    envelope.UpdateSummary,
	})
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

func validateAIDesignUpdate(current json.RawMessage, proposed json.RawMessage, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = 4 << 20
	}
	if int64(len(proposed)) > maxBytes || len(proposed) == 0 || !json.Valid(proposed) {
		return errors.New("design document is invalid or exceeds the configured size limit")
	}
	type graphDocument struct {
		SchemaVersion string `json:"schemaVersion"`
		ID            string `json:"id"`
		Components    []struct {
			ID string `json:"id"`
		} `json:"components"`
		Connectors []struct {
			ID   string `json:"id"`
			From string `json:"fromComponentId"`
			To   string `json:"toComponentId"`
		} `json:"connectors"`
	}
	var before, after graphDocument
	if json.Unmarshal(current, &before) != nil || json.Unmarshal(proposed, &after) != nil {
		return errors.New("design document does not match the structured schema")
	}
	if before.ID != after.ID || before.SchemaVersion != after.SchemaVersion || after.ID == "" || after.SchemaVersion == "" {
		return errors.New("schema version and design identity must be preserved")
	}
	if len(after.Components) > 500 || len(after.Connectors) > 1000 {
		return errors.New("design update exceeds component or connector limits")
	}
	componentIDs := map[string]bool{}
	for _, component := range after.Components {
		if strings.TrimSpace(component.ID) == "" || componentIDs[component.ID] {
			return errors.New("component IDs must be present and unique")
		}
		componentIDs[component.ID] = true
	}
	connectorIDs := map[string]bool{}
	for _, connector := range after.Connectors {
		if strings.TrimSpace(connector.ID) == "" || connectorIDs[connector.ID] || !componentIDs[connector.From] || !componentIDs[connector.To] {
			return errors.New("connectors must have unique IDs and reference existing components")
		}
		connectorIDs[connector.ID] = true
	}
	return nil
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
