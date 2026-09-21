package store

import (
	"github.com/system-design-evaluator/backend/internal/domain"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu              sync.RWMutex
	workspaces      map[string]domain.Workspace
	designs         map[string]domain.Design
	versions        map[string][]domain.DesignVersion
	docs            map[string]domain.DesignDoc
	aiConversations map[string]domain.AIConversation
	aiMessages      map[string]domain.AIMessage
	users           map[string]domain.User
	oidcFlows       map[string]domain.OIDCFlow
	userIdentities  map[string]string
	failedLogins    map[string]int
	lockedUntil     map[string]time.Time
	sessions        map[string]memorySession
	passwordReset   map[string]domain.PasswordResetToken
	accessGroups    map[string]domain.AccessGroup
	groupMembers    map[string]domain.AccessGroupMember
	workspaceACL    map[string]domain.WorkspaceAccess
	workspaceGACL   map[string]domain.WorkspaceGroupAccess
	designACL       map[string]domain.DesignAccess
	designGACL      map[string]domain.DesignGroupAccess
	signInConfig    domain.SignInConfig
	aiConfig        domain.AIProviderConfig
	mcpConfig       domain.MCPConfig
	telemetryConfig domain.TelemetryIntegrationConfig
	catalogAssets   map[string]domain.CatalogAsset
	comments        map[string]domain.DesignComment
	reviews         map[string]domain.DesignReviewRequest
	notifications   map[string]domain.Notification
	clock           func() time.Time
}

type memorySession struct {
	UserID     string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		workspaces:      make(map[string]domain.Workspace),
		designs:         make(map[string]domain.Design),
		versions:        make(map[string][]domain.DesignVersion),
		docs:            make(map[string]domain.DesignDoc),
		aiConversations: make(map[string]domain.AIConversation),
		aiMessages:      make(map[string]domain.AIMessage),
		users:           make(map[string]domain.User),
		oidcFlows:       make(map[string]domain.OIDCFlow),
		userIdentities:  make(map[string]string),
		failedLogins:    make(map[string]int),
		lockedUntil:     make(map[string]time.Time),
		sessions:        make(map[string]memorySession),
		passwordReset:   make(map[string]domain.PasswordResetToken),
		accessGroups:    make(map[string]domain.AccessGroup),
		groupMembers:    make(map[string]domain.AccessGroupMember),
		workspaceACL:    make(map[string]domain.WorkspaceAccess),
		workspaceGACL:   make(map[string]domain.WorkspaceGroupAccess),
		designACL:       make(map[string]domain.DesignAccess),
		designGACL:      make(map[string]domain.DesignGroupAccess),
		signInConfig:    defaultSignInConfig(time.Now().UTC()),
		aiConfig:        defaultAIProviderConfig(time.Now().UTC()),
		mcpConfig:       defaultMCPConfig(time.Now().UTC()),
		telemetryConfig: defaultTelemetryIntegrationConfig(time.Now().UTC()),
		catalogAssets:   make(map[string]domain.CatalogAsset),
		comments:        make(map[string]domain.DesignComment),
		reviews:         make(map[string]domain.DesignReviewRequest),
		notifications:   make(map[string]domain.Notification),
		clock:           time.Now,
	}
}
