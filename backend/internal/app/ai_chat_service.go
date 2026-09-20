package app

import (
	"context"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

// AIChatService owns persisted, design-scoped AI conversations. Model
// invocation remains an infrastructure concern orchestrated above this service.
type AIChatService struct {
	provider RepositoryProvider
}

func (s AIChatService) repo() store.Repository { return s.provider.Repository() }

func (s AIChatService) ListConversations(ctx context.Context, workspaceID string, designID string) ([]domain.AIConversation, error) {
	return s.repo().ListAIConversations(ctx, workspaceID, designID)
}

func (s AIChatService) GetConversation(ctx context.Context, workspaceID string, designID string, conversationID string) (domain.AIConversation, error) {
	return s.repo().GetAIConversation(ctx, workspaceID, designID, conversationID)
}

func (s AIChatService) CreateConversation(ctx context.Context, conversation domain.AIConversation) (domain.AIConversation, error) {
	return s.repo().CreateAIConversation(ctx, conversation)
}

func (s AIChatService) ListMessages(ctx context.Context, conversationID string, limit int) ([]domain.AIMessage, error) {
	return s.repo().ListAIMessages(ctx, conversationID, limit)
}

func (s AIChatService) AddMessage(ctx context.Context, message domain.AIMessage) (domain.AIMessage, error) {
	return s.repo().CreateAIMessage(ctx, message)
}
