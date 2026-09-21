package store

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"testing"
)

func TestMemoryRepositoryGuestWorkspace(t *testing.T) {
	repo := NewMemoryRepository()

	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	if workspace.ID != domain.GuestWorkspaceID {
		t.Fatalf("workspace id = %q, want %q", workspace.ID, domain.GuestWorkspaceID)
	}

	second, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("second GetOrCreateGuestWorkspace returned error: %v", err)
	}
	if !workspace.CreatedAt.Equal(second.CreatedAt) {
		t.Fatalf("guest workspace should be stable across calls")
	}
}
