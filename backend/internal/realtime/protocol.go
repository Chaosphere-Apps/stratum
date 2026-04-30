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
}

type WorkspaceSnapshotPayload struct {
	Snapshot domain.WorkspaceSnapshot `json:"snapshot"`
}

type DesignUpdatedPayload struct {
	Design domain.Design `json:"design"`
}

const (
	MessageWorkspaceSnapshot = "workspace.snapshot"
	MessageDesignUpsert      = "design.upsert"
	MessageDesignUpdated     = "design.updated"
	MessagePing              = "ping"
	MessagePong              = "pong"
	MessageError             = "error"
)
