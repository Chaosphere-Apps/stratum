package domain

import (
	"encoding/json"
	"time"
)

const GuestWorkspaceID = "guest-workspace"
const GuestUserID = "guest-user"

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Design struct {
	ID             string          `json:"id"`
	WorkspaceID    string          `json:"workspaceId"`
	Name           string          `json:"name"`
	Access         string          `json:"access"`
	Title          string          `json:"title"`
	Document       json.RawMessage `json:"document"`
	CanvasSnapshot json.RawMessage `json:"canvasSnapshot,omitempty"`
	VersionNumber  int             `json:"versionNumber"`
	CreatedBy      string          `json:"createdBy"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type DesignVersion struct {
	ID             string          `json:"id"`
	DesignID       string          `json:"designId"`
	WorkspaceID    string          `json:"workspaceId"`
	VersionNumber  int             `json:"versionNumber"`
	Document       json.RawMessage `json:"document"`
	CanvasSnapshot json.RawMessage `json:"canvasSnapshot,omitempty"`
	CreatedBy      string          `json:"createdBy"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type DesignDoc struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	DesignID    string    `json:"designId"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Format      string    `json:"format"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type WorkspaceSnapshot struct {
	Workspace Workspace `json:"workspace"`
	Designs   []Design  `json:"designs"`
}

func NewGuestWorkspace(now time.Time) Workspace {
	return Workspace{
		ID:        GuestWorkspaceID,
		Name:      "Guest Workspace",
		CreatedAt: now,
		UpdatedAt: now,
	}
}
