package domain

import (
	"encoding/json"
	"time"
)

const GuestWorkspaceID = "guest-workspace"

type User struct {
	ID           string    `json:"id"`
	DisplayName  string    `json:"displayName"`
	Email        string    `json:"email"`
	PasswordSet  bool      `json:"passwordSet"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type SignInConfig struct {
	LocalPasswordEnabled  bool      `json:"localPasswordEnabled"`
	SSOEnabled            bool      `json:"ssoEnabled"`
	Provider              string    `json:"provider"`
	OktaDomain            string    `json:"oktaDomain"`
	Issuer                string    `json:"issuer"`
	ClientID              string    `json:"clientId"`
	ClientSecretSet       bool      `json:"clientSecretSet"`
	RedirectURI           string    `json:"redirectUri"`
	PostLogoutRedirectURI string    `json:"postLogoutRedirectUri"`
	Scopes                string    `json:"scopes"`
	GroupsClaim           string    `json:"groupsClaim"`
	AdminGroup            string    `json:"adminGroup"`
	ReviewerGroup         string    `json:"reviewerGroup"`
	JITProvisioning       bool      `json:"jitProvisioning"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type PasswordResetToken struct {
	Token     string     `json:"-"`
	UserID    string     `json:"userId"`
	ExpiresAt time.Time  `json:"expiresAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
}

type AIProviderConfig struct {
	Enabled    bool      `json:"enabled"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	BaseURL    string    `json:"baseUrl"`
	APIKey     string    `json:"-"`
	APIKeySet  bool      `json:"apiKeySet"`
	VerifiedAt time.Time `json:"verifiedAt,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type MCPConfig struct {
	Enabled             bool      `json:"enabled"`
	EndpointPath        string    `json:"endpointPath"`
	ReadCatalog         bool      `json:"readCatalog"`
	ReadDesigns         bool      `json:"readDesigns"`
	CreateDraftDesign   bool      `json:"createDraftDesign"`
	RunAnalysis         bool      `json:"runAnalysis"`
	FetchImpactReport   bool      `json:"fetchImpactReport"`
	RequireAdminConsent bool      `json:"requireAdminConsent"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type CatalogAsset struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	NormalizedName    string          `json:"normalizedName"`
	Type              string          `json:"type"`
	Owner             string          `json:"owner"`
	Description       string          `json:"description"`
	Criticality       string          `json:"criticality"`
	Tags              []string        `json:"tags"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CreatedBy         string          `json:"createdBy"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
	UsedInDesignCount int             `json:"usedInDesignCount"`
}

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type WorkspaceAccess struct {
	WorkspaceID     string    `json:"workspaceId"`
	UserID          string    `json:"userId"`
	CanRead         bool      `json:"canRead"`
	CanCreateDesign bool      `json:"canCreateDesign"`
	CanManage       bool      `json:"canManage"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type AccessGroup struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	OktaGroupName string    `json:"oktaGroupName"`
	MemberCount   int       `json:"memberCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type AccessGroupMember struct {
	GroupID string    `json:"groupId"`
	UserID  string    `json:"userId"`
	AddedAt time.Time `json:"addedAt"`
}

type WorkspaceGroupAccess struct {
	WorkspaceID     string    `json:"workspaceId"`
	GroupID         string    `json:"groupId"`
	CanRead         bool      `json:"canRead"`
	CanCreateDesign bool      `json:"canCreateDesign"`
	CanManage       bool      `json:"canManage"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
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
	VersionRemarks string          `json:"-"`
	CreatedBy      string          `json:"createdBy"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type DesignAccess struct {
	WorkspaceID string    `json:"workspaceId"`
	DesignID    string    `json:"designId"`
	UserID      string    `json:"userId"`
	CanRead     bool      `json:"canRead"`
	CanEdit     bool      `json:"canEdit"`
	CanComment  bool      `json:"canComment"`
	CanReview   bool      `json:"canReview"`
	CanManage   bool      `json:"canManage"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type DesignGroupAccess struct {
	WorkspaceID string    `json:"workspaceId"`
	DesignID    string    `json:"designId"`
	GroupID     string    `json:"groupId"`
	CanRead     bool      `json:"canRead"`
	CanEdit     bool      `json:"canEdit"`
	CanComment  bool      `json:"canComment"`
	CanReview   bool      `json:"canReview"`
	CanManage   bool      `json:"canManage"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type DesignVersion struct {
	ID             string          `json:"id"`
	DesignID       string          `json:"designId"`
	WorkspaceID    string          `json:"workspaceId"`
	VersionNumber  int             `json:"versionNumber"`
	Status         string          `json:"status"`
	Remarks        string          `json:"remarks"`
	Document       json.RawMessage `json:"document"`
	CanvasSnapshot json.RawMessage `json:"canvasSnapshot,omitempty"`
	CreatedBy      string          `json:"createdBy"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
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

type DesignComment struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	DesignID    string    `json:"designId"`
	AuthorID    string    `json:"authorId"`
	Body        string    `json:"body"`
	ComponentID string    `json:"componentId,omitempty"`
	ConnectorID string    `json:"connectorId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type DesignReviewRequest struct {
	ID            string     `json:"id"`
	WorkspaceID   string     `json:"workspaceId"`
	DesignID      string     `json:"designId"`
	VersionID     string     `json:"versionId"`
	VersionNumber int        `json:"versionNumber"`
	RequestedBy   string     `json:"requestedBy"`
	ReviewerID    string     `json:"reviewerId"`
	Status        string     `json:"status"`
	Message       string     `json:"message"`
	Summary       string     `json:"summary"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

type Notification struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	WorkspaceID string    `json:"workspaceId"`
	DesignID    string    `json:"designId"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Read        bool      `json:"read"`
	CreatedAt   time.Time `json:"createdAt"`
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
