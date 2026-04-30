package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

func TestHubUpsertDesign(t *testing.T) {
	repo := store.NewMemoryRepository()
	hub := NewHub(repo, slog.Default())

	payload := UpsertDesignPayload{
		Design:         json.RawMessage(`{"id":"design-1","title":"Design One","components":[],"connectors":[]}`),
		CanvasSnapshot: json.RawMessage(`{"provider":"react-flow","viewport":{"x":0,"y":0,"zoom":1}}`),
	}

	design, err := hub.UpsertDesign(context.Background(), domain.GuestWorkspaceID, payload)
	if err != nil {
		t.Fatalf("UpsertDesign returned error: %v", err)
	}
	if design.ID != "design-1" {
		t.Fatalf("design id = %q, want design-1", design.ID)
	}
	if design.Name != "Design One" {
		t.Fatalf("design name = %q, want Design One", design.Name)
	}

	snapshot, err := hub.Snapshot(context.Background(), domain.GuestWorkspaceID)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(snapshot.Designs) != 1 {
		t.Fatalf("snapshot design count = %d, want 1", len(snapshot.Designs))
	}
	if len(snapshot.Designs[0].CanvasSnapshot) == 0 {
		t.Fatal("snapshot design canvas snapshot was not retained")
	}
}

func TestHubUpsertDesignDropsInvalidCanvasSnapshot(t *testing.T) {
	repo := store.NewMemoryRepository()
	hub := NewHub(repo, slog.Default())

	payload := UpsertDesignPayload{
		Design:         json.RawMessage(`{"id":"design-1","title":"Design One","components":[],"connectors":[]}`),
		CanvasSnapshot: json.RawMessage(`null`),
	}

	design, err := hub.UpsertDesign(context.Background(), domain.GuestWorkspaceID, payload)
	if err != nil {
		t.Fatalf("UpsertDesign returned error: %v", err)
	}
	if len(design.CanvasSnapshot) != 0 {
		t.Fatalf("invalid canvas snapshot was retained: %s", string(design.CanvasSnapshot))
	}
}
