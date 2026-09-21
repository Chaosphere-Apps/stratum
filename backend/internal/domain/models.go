package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

const (
	GuestWorkspaceID    = "guest-workspace"
	MaxDesignNameLength = 120
)

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
	ClientSecret          string    `json:"-"`
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

type OIDCIdentity struct {
	Provider    string
	Subject     string
	Email       string
	DisplayName string
	Role        string
	Groups      []string
}

type OIDCFlow struct {
	StateHash    string
	Nonce        string
	CodeVerifier string
	ExpiresAt    time.Time
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

type TelemetryIntegrationConfig struct {
	Enabled             bool                       `json:"enabled"`
	Provider            string                     `json:"provider"`
	DisplayName         string                     `json:"displayName"`
	BaseURL             string                     `json:"baseUrl"`
	AuthMode            string                     `json:"authMode"`
	Secret              string                     `json:"-"`
	SecretSet           bool                       `json:"secretSet"`
	CustomHeaderName    string                     `json:"customHeaderName"`
	QueryWindow         string                     `json:"queryWindow"`
	RequestTotalMetric  string                     `json:"requestTotalMetric"`
	RequestFailedMetric string                     `json:"requestFailedMetric"`
	ServerLatencyMetric string                     `json:"serverLatencyMetric"`
	ClientLatencyMetric string                     `json:"clientLatencyMetric"`
	Filters             TelemetryIntegrationFilter `json:"filters"`
	UpdatedAt           time.Time                  `json:"updatedAt"`
}

type TelemetryIntegrationFilter struct {
	Namespaces             []string          `json:"namespaces"`
	Services               []string          `json:"services"`
	ExcludeServices        []string          `json:"excludeServices"`
	ExcludeEndpoints       []string          `json:"excludeEndpoints"`
	RequiredLabels         map[string]string `json:"requiredLabels,omitempty"`
	MinimumRequestsPerSec  float64           `json:"minimumRequestsPerSec"`
	IncludeExternal        bool              `json:"includeExternal"`
	IncludeDatabaseClients bool              `json:"includeDatabaseClients"`
}

type CatalogAsset struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	NormalizedName     string          `json:"normalizedName"`
	Kind               string          `json:"kind"`
	Type               string          `json:"type"`
	Owner              string          `json:"owner"`
	Description        string          `json:"description"`
	Criticality        string          `json:"criticality"`
	Status             string          `json:"status"`
	Aliases            []string        `json:"aliases"`
	ReplacementAssetID string          `json:"replacementAssetId"`
	UpdateMessage      string          `json:"updateMessage"`
	Tags               []string        `json:"tags"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
	CreatedBy          string          `json:"createdBy"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	UsedInDesignCount  int             `json:"usedInDesignCount"`
}

type Workspace struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	OwnerID         string    `json:"ownerId"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	EffectiveAccess string    `json:"effectiveAccess,omitempty"`
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
	ID               string          `json:"id"`
	WorkspaceID      string          `json:"workspaceId"`
	Name             string          `json:"name"`
	Access           string          `json:"access"`
	Title            string          `json:"title"`
	Document         json.RawMessage `json:"document"`
	DocumentRevision string          `json:"documentRevision"`
	CanvasSnapshot   json.RawMessage `json:"canvasSnapshot,omitempty"`
	VersionNumber    int             `json:"versionNumber"`
	VersionRemarks   string          `json:"-"`
	CreatedBy        string          `json:"createdBy"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	EffectiveAccess  string          `json:"effectiveAccess,omitempty"`
}

func DesignRevision(document []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(document))
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

type AIConversation struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	DesignID    string    `json:"designId"`
	VersionID   string    `json:"versionId,omitempty"`
	Title       string    `json:"title"`
	AccessMode  string    `json:"accessMode"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type AIMessageReference struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type AIMessage struct {
	ID             string               `json:"id"`
	ConversationID string               `json:"conversationId"`
	Role           string               `json:"role"`
	Content        string               `json:"content"`
	References     []AIMessageReference `json:"references,omitempty"`
	Provider       string               `json:"provider,omitempty"`
	Model          string               `json:"model,omitempty"`
	CreatedBy      string               `json:"createdBy,omitempty"`
	CreatedAt      time.Time            `json:"createdAt"`
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
