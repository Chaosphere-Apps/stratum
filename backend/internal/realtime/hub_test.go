package realtime

import (
	"context"
	"encoding/json"
	"errors"
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

func TestHubUpdateDesignUsesOptimisticConcurrency(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemoryRepository()
	design, err := repo.CreateDesign(ctx, domain.GuestWorkspaceID, "Design", []byte(`{"id":"design-1","title":"Design"}`), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repo, slog.Default())
	firstDocument := json.RawMessage(`{"id":"` + design.ID + `","title":"First editor"}`)
	staleDocument := json.RawMessage(`{"id":"` + design.ID + `","title":"Stale editor"}`)

	if _, err := hub.UpdateDesign(ctx, domain.GuestWorkspaceID, UpsertDesignPayload{
		Design:       firstDocument,
		BaseRevision: design.DocumentRevision,
	}); err != nil {
		t.Fatalf("first update returned error: %v", err)
	}
	_, err = hub.UpdateDesign(ctx, domain.GuestWorkspaceID, UpsertDesignPayload{
		Design:       staleDocument,
		BaseRevision: design.DocumentRevision,
	})
	if !errors.Is(err, store.ErrDesignConflict) {
		t.Fatalf("stale update error = %v, want ErrDesignConflict", err)
	}
}

func TestHubPresenceAggregatesTabsPerUserAndDesign(t *testing.T) {
	hub := NewHub(store.NewMemoryRepository(), slog.Default())
	firstTab := NewClient(nil, hub, slog.Default(), "workspace-1", "user-1", "architect", false).WithPresence("Ada", "design-1")
	secondTab := NewClient(nil, hub, slog.Default(), "workspace-1", "user-1", "architect", false).WithPresence("Ada", "design-1")
	reviewer := NewClient(nil, hub, slog.Default(), "workspace-1", "user-2", "reviewer", false).WithPresence("Grace", "design-2")

	hub.Subscribe("workspace-1", firstTab)
	hub.Subscribe("workspace-1", secondTab)
	hub.Subscribe("workspace-1", reviewer)
	members := hub.presenceMembers("workspace-1")

	if len(members) != 2 {
		t.Fatalf("presence members = %#v, want two principals", members)
	}
	if members[0].DisplayName != "Ada" || members[0].SessionCount != 2 || members[0].DesignID != "design-1" {
		t.Fatalf("aggregated Ada presence = %#v", members[0])
	}
	if members[1].DisplayName != "Grace" || members[1].SessionCount != 1 || members[1].DesignID != "design-2" {
		t.Fatalf("Grace presence = %#v", members[1])
	}
}

func TestClientCanEditDesignBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("admin bypasses payload validation", func(t *testing.T) {
		repo := store.NewMemoryRepository()
		hub := NewHub(repo, slog.Default())
		client := NewClient(nil, hub, slog.Default(), domain.GuestWorkspaceID, "admin", "admin", true)
		if !client.canEditDesign(ctx, UpsertDesignPayload{Design: json.RawMessage(`{`)}) {
			t.Fatal("admin should be allowed before payload inspection")
		}
	})

	t.Run("invalid payload is denied for non-admin", func(t *testing.T) {
		repo := store.NewMemoryRepository()
		hub := NewHub(repo, slog.Default())
		client := NewClient(nil, hub, slog.Default(), domain.GuestWorkspaceID, "user-1", "member", false)
		if client.canEditDesign(ctx, UpsertDesignPayload{Design: json.RawMessage(`{`)}) {
			t.Fatal("invalid payload should be denied")
		}
		if client.canEditDesign(ctx, UpsertDesignPayload{Design: json.RawMessage(`{"title":"missing id"}`)}) {
			t.Fatal("payload without design id should be denied")
		}
	})

	t.Run("new design is denied on websocket even with workspace create access", func(t *testing.T) {
		repo := store.NewMemoryRepository()
		if _, err := repo.CreateFirstAdmin(ctx, "Admin", "admin@example.com", "password123"); err != nil {
			t.Fatalf("CreateFirstAdmin returned error: %v", err)
		}
		user, err := repo.CreateUser(ctx, "User", "user@example.com", "member", "password123")
		if err != nil {
			t.Fatalf("CreateUser returned error: %v", err)
		}
		if _, err := repo.GetOrCreateGuestWorkspace(ctx); err != nil {
			t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
		}
		hub := NewHub(repo, slog.Default())
		client := NewClient(nil, hub, slog.Default(), domain.GuestWorkspaceID, user.ID, user.Role, false)
		payload := UpsertDesignPayload{Design: json.RawMessage(`{"id":"new-design","title":"New"}`)}
		if client.canEditDesign(ctx, payload) {
			t.Fatal("new design should be denied without workspace grant")
		}
		if _, err := repo.GrantWorkspaceAccess(ctx, domain.WorkspaceAccess{
			WorkspaceID:     domain.GuestWorkspaceID,
			UserID:          user.ID,
			CanCreateDesign: true,
		}); err != nil {
			t.Fatalf("GrantWorkspaceAccess returned error: %v", err)
		}
		if client.canEditDesign(ctx, payload) {
			t.Fatal("new design should use the authorized REST create endpoint before websocket updates")
		}
	})

	t.Run("created-by user can edit existing design", func(t *testing.T) {
		repo := store.NewMemoryRepository()
		user, err := repo.CreateFirstAdmin(ctx, "User", "user@example.com", "password123")
		if err != nil {
			t.Fatalf("CreateFirstAdmin returned error: %v", err)
		}
		design, err := repo.CreateDesign(ctx, domain.GuestWorkspaceID, "Owned", []byte(`{"id":"owned","title":"Owned"}`), user.ID)
		if err != nil {
			t.Fatalf("CreateDesign returned error: %v", err)
		}
		hub := NewHub(repo, slog.Default())
		client := NewClient(nil, hub, slog.Default(), domain.GuestWorkspaceID, user.ID, user.Role, false)
		if !client.canEditDesign(ctx, UpsertDesignPayload{Design: json.RawMessage(`{"id":"` + design.ID + `","title":"Owned"}`)}) {
			t.Fatal("creator should be allowed to edit existing design")
		}
	})

	t.Run("design user and group grants can edit existing design", func(t *testing.T) {
		repo := store.NewMemoryRepository()
		if _, err := repo.CreateFirstAdmin(ctx, "Owner", "owner@example.com", "password123"); err != nil {
			t.Fatalf("CreateFirstAdmin returned error: %v", err)
		}
		editor, err := repo.CreateUser(ctx, "Editor", "editor@example.com", "member", "password123")
		if err != nil {
			t.Fatalf("CreateUser editor returned error: %v", err)
		}
		groupEditor, err := repo.CreateUser(ctx, "Group Editor", "group-editor@example.com", "member", "password123")
		if err != nil {
			t.Fatalf("CreateUser group editor returned error: %v", err)
		}
		design, err := repo.CreateDesign(ctx, domain.GuestWorkspaceID, "Shared", []byte(`{"id":"shared","title":"Shared"}`), "owner")
		if err != nil {
			t.Fatalf("CreateDesign returned error: %v", err)
		}
		payload := UpsertDesignPayload{Design: json.RawMessage(`{"id":"` + design.ID + `","title":"Shared"}`)}
		hub := NewHub(repo, slog.Default())

		userClient := NewClient(nil, hub, slog.Default(), domain.GuestWorkspaceID, editor.ID, editor.Role, false)
		if userClient.canEditDesign(ctx, payload) {
			t.Fatal("design should be denied before design access grant")
		}
		if _, err := repo.GrantDesignAccess(ctx, domain.DesignAccess{
			WorkspaceID: domain.GuestWorkspaceID,
			DesignID:    design.ID,
			UserID:      editor.ID,
			CanEdit:     true,
		}); err != nil {
			t.Fatalf("GrantDesignAccess returned error: %v", err)
		}
		if !userClient.canEditDesign(ctx, payload) {
			t.Fatal("design edit grant should allow editing")
		}

		group, err := repo.CreateAccessGroup(ctx, domain.AccessGroup{Name: "Architects"})
		if err != nil {
			t.Fatalf("CreateAccessGroup returned error: %v", err)
		}
		if _, err := repo.ReplaceAccessGroupMembers(ctx, group.ID, []string{groupEditor.ID}); err != nil {
			t.Fatalf("ReplaceAccessGroupMembers returned error: %v", err)
		}
		if _, err := repo.GrantDesignGroupAccess(ctx, domain.DesignGroupAccess{
			WorkspaceID: domain.GuestWorkspaceID,
			DesignID:    design.ID,
			GroupID:     group.ID,
			CanManage:   true,
		}); err != nil {
			t.Fatalf("GrantDesignGroupAccess returned error: %v", err)
		}
		groupClient := NewClient(nil, hub, slog.Default(), domain.GuestWorkspaceID, groupEditor.ID, groupEditor.Role, false)
		if !groupClient.canEditDesign(ctx, payload) {
			t.Fatal("design group manage grant should allow editing")
		}
	})

	t.Run("workspace manage grants can edit existing design", func(t *testing.T) {
		repo := store.NewMemoryRepository()
		if _, err := repo.CreateFirstAdmin(ctx, "Owner", "owner@example.com", "password123"); err != nil {
			t.Fatalf("CreateFirstAdmin returned error: %v", err)
		}
		manager, err := repo.CreateUser(ctx, "Manager", "manager@example.com", "architect", "password123")
		if err != nil {
			t.Fatalf("CreateUser manager returned error: %v", err)
		}
		design, err := repo.CreateDesign(ctx, domain.GuestWorkspaceID, "Managed", []byte(`{"id":"managed","title":"Managed"}`), "owner")
		if err != nil {
			t.Fatalf("CreateDesign returned error: %v", err)
		}
		payload := UpsertDesignPayload{Design: json.RawMessage(`{"id":"` + design.ID + `","title":"Managed"}`)}
		if _, err := repo.GrantWorkspaceAccess(ctx, domain.WorkspaceAccess{
			WorkspaceID: domain.GuestWorkspaceID,
			UserID:      manager.ID,
			CanManage:   true,
		}); err != nil {
			t.Fatalf("GrantWorkspaceAccess returned error: %v", err)
		}
		hub := NewHub(repo, slog.Default())
		client := NewClient(nil, hub, slog.Default(), domain.GuestWorkspaceID, manager.ID, manager.Role, false)
		if !client.canEditDesign(ctx, payload) {
			t.Fatal("workspace manager should be allowed to edit existing design")
		}
	})
}

func TestHubSetRepositoryChangesSnapshotSource(t *testing.T) {
	ctx := context.Background()
	first := store.NewMemoryRepository()
	second := store.NewMemoryRepository()
	hub := NewHub(first, slog.Default())

	if _, err := first.CreateDesign(ctx, domain.GuestWorkspaceID, "First", []byte(`{"id":"first","title":"First"}`), "user"); err != nil {
		t.Fatalf("CreateDesign first returned error: %v", err)
	}
	if _, err := second.CreateDesign(ctx, domain.GuestWorkspaceID, "Second", []byte(`{"id":"second","title":"Second"}`), "user"); err != nil {
		t.Fatalf("CreateDesign second returned error: %v", err)
	}

	hub.SetRepository(second)
	snapshot, err := hub.Snapshot(ctx, domain.GuestWorkspaceID)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(snapshot.Designs) != 1 || snapshot.Designs[0].Name != "Second" {
		t.Fatalf("snapshot = %#v, want repository replacement to read second repo", snapshot.Designs)
	}
}
