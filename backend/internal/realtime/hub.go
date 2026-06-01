package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

type Hub struct {
	repo        store.Repository
	log         *slog.Logger
	mu          sync.RWMutex
	subscribers map[string]map[*Client]struct{}
}

func NewHub(repo store.Repository, log *slog.Logger) *Hub {
	return &Hub{
		repo:        repo,
		log:         log,
		subscribers: make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Repository() store.Repository {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.repo
}

func (h *Hub) SetRepository(repo store.Repository) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.repo = repo
}

func (h *Hub) Snapshot(ctx context.Context, workspaceID string) (domain.WorkspaceSnapshot, error) {
	repo := h.Repository()
	workspace, err := repo.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return domain.WorkspaceSnapshot{}, err
	}
	designs, err := repo.ListDesigns(ctx, workspaceID)
	if err != nil {
		return domain.WorkspaceSnapshot{}, err
	}
	return domain.WorkspaceSnapshot{
		Workspace: workspace,
		Designs:   designs,
	}, nil
}

func (h *Hub) Subscribe(workspaceID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subscribers[workspaceID] == nil {
		h.subscribers[workspaceID] = make(map[*Client]struct{})
	}
	h.subscribers[workspaceID][client] = struct{}{}
}

func (h *Hub) Unsubscribe(workspaceID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients := h.subscribers[workspaceID]
	if clients == nil {
		return
	}
	delete(clients, client)
	if len(clients) == 0 {
		delete(h.subscribers, workspaceID)
	}
}

func (h *Hub) Broadcast(ctx context.Context, workspaceID string, envelope Envelope) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.subscribers[workspaceID]))
	for client := range h.subscribers[workspaceID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case <-ctx.Done():
			return
		default:
			client.Send(envelope)
		}
	}
}

func (h *Hub) UpsertDesign(ctx context.Context, workspaceID string, payload UpsertDesignPayload) (domain.Design, error) {
	var document struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(payload.Design, &document); err != nil {
		return domain.Design{}, err
	}

	now := time.Now().UTC()
	design := domain.Design{
		ID:          document.ID,
		WorkspaceID: workspaceID,
		Title:       document.Title,
		Document:    payload.Design,
		CreatedBy:   "",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if domain.ValidCanvasSnapshot(payload.CanvasSnapshot) {
		design.CanvasSnapshot = payload.CanvasSnapshot
	}

	return h.Repository().UpsertDesign(ctx, design)
}
