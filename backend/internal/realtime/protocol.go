package realtime

import (
	"encoding/json"

	"github.com/system-design-evaluator/backend/internal/domain"
)

type Envelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     *ProtocolError  `json:"error,omitempty"`
}

type ProtocolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type UpsertDesignPayload struct {
	Design         json.RawMessage `json:"design"`
	CanvasSnapshot json.RawMessage `json:"canvasSnapshot,omitempty"`
	BaseRevision   string          `json:"baseRevision"`
}

type WorkspaceSnapshotPayload struct {
	Snapshot domain.WorkspaceSnapshot `json:"snapshot"`
}

type DesignUpdatedPayload struct {
	Design domain.Design `json:"design"`
}

type DesignConflictPayload struct {
	Design domain.Design `json:"design"`
}

type PresenceMember struct {
	UserID       string `json:"userId"`
	DisplayName  string `json:"displayName"`
	Role         string `json:"role"`
	DesignID     string `json:"designId,omitempty"`
	SessionCount int    `json:"sessionCount"`
}

type PresenceUpdatedPayload struct {
	Members []PresenceMember `json:"members"`
}

const (
	MessageWorkspaceSnapshot = "workspace.snapshot"
	MessageDesignUpsert      = "design.upsert"
	MessageDesignUpdated     = "design.updated"
	MessagePresenceUpdated   = "presence.updated"
	MessagePing              = "ping"
	MessagePong              = "pong"
	MessageError             = "error"
)
