package app

import (
	"context"
	"testing"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

func TestAIChatServicePersistsDesignScopedConversation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := store.NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(ctx)
	requireNoError(t, err)
	design, err := repo.CreateDesign(ctx, workspace.ID, "Payments", nil, "architect")
	requireNoError(t, err)
	service := AIChatService{provider: StaticRepositoryProvider{Repo: repo}}

	conversation, err := service.CreateConversation(ctx, domain.AIConversation{
		WorkspaceID: workspace.ID,
		DesignID:    design.ID,
		CreatedBy:   "architect",
	})
	requireNoError(t, err)
	conversations, err := service.ListConversations(ctx, workspace.ID, design.ID)
	requireNoError(t, err)
	if len(conversations) != 1 || conversations[0].ID != conversation.ID {
		t.Fatalf("conversations = %#v", conversations)
	}
	conversation, err = service.UpdateConversationAccess(ctx, workspace.ID, design.ID, conversation.ID, "read_write")
	requireNoError(t, err)
	if conversation.AccessMode != "read_write" {
		t.Fatalf("conversation access mode = %q", conversation.AccessMode)
	}
	if conversation.Title != "Architecture discussion" {
		t.Fatalf("default title = %q", conversation.Title)
	}
	message, err := service.AddMessage(ctx, domain.AIMessage{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Where should retries live?",
		CreatedBy:      "architect",
	})
	requireNoError(t, err)
	if message.ID == "" {
		t.Fatal("message id was not generated")
	}
	messages, err := service.ListMessages(ctx, conversation.ID, 10)
	requireNoError(t, err)
	if len(messages) != 1 || messages[0].Content != message.Content {
		t.Fatalf("messages = %#v", messages)
	}
	loaded, err := service.GetConversation(ctx, workspace.ID, design.ID, conversation.ID)
	requireNoError(t, err)
	if loaded.ID != conversation.ID || loaded.UpdatedAt.Before(loaded.CreatedAt) {
		t.Fatalf("conversation timestamps were not updated: %#v", loaded)
	}
}

func TestAIChatServiceRejectsCrossDesignConversationLookup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := store.NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(ctx)
	requireNoError(t, err)
	design, err := repo.CreateDesign(ctx, workspace.ID, "Payments", nil, "architect")
	requireNoError(t, err)
	other, err := repo.CreateDesign(ctx, workspace.ID, "Ledger", nil, "architect")
	requireNoError(t, err)
	service := AIChatService{provider: StaticRepositoryProvider{Repo: repo}}
	conversation, err := service.CreateConversation(ctx, domain.AIConversation{WorkspaceID: workspace.ID, DesignID: design.ID})
	requireNoError(t, err)

	if _, err := service.GetConversation(ctx, workspace.ID, other.ID, conversation.ID); err == nil {
		t.Fatal("cross-design conversation lookup unexpectedly succeeded")
	}
}
