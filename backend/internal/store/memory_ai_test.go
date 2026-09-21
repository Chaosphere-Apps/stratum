package store

import (
	"context"
	"testing"

	"github.com/system-design-evaluator/backend/internal/domain"
)

func TestMemoryRepositoryDeleteDesignRemovesAIChat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(ctx)
	requireMemoryAINoError(t, err)
	design, err := repo.CreateDesign(ctx, workspace.ID, "Payments", nil, "architect")
	requireMemoryAINoError(t, err)
	conversation, err := repo.CreateAIConversation(ctx, domain.AIConversation{
		WorkspaceID: workspace.ID,
		DesignID:    design.ID,
		CreatedBy:   "architect",
	})
	requireMemoryAINoError(t, err)
	if conversation.AccessMode != "read" {
		t.Fatalf("conversation should default to read access, got %q", conversation.AccessMode)
	}
	conversation, err = repo.UpdateAIConversationAccess(ctx, workspace.ID, design.ID, conversation.ID, "read_write")
	requireMemoryAINoError(t, err)
	if conversation.AccessMode != "read_write" {
		t.Fatalf("conversation access mode was not persisted: %#v", conversation)
	}
	_, err = repo.CreateAIMessage(ctx, domain.AIMessage{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Review this design",
		CreatedBy:      "architect",
	})
	requireMemoryAINoError(t, err)

	requireMemoryAINoError(t, repo.DeleteDesign(ctx, workspace.ID, design.ID))
	if _, err := repo.GetAIConversation(ctx, workspace.ID, design.ID, conversation.ID); err == nil {
		t.Fatal("deleted design's AI conversation is still available")
	}
	if _, err := repo.ListAIMessages(ctx, conversation.ID, 10); err == nil {
		t.Fatal("deleted design's AI messages are still available")
	}
}

func requireMemoryAINoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
