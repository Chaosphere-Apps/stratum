package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/lifecycle"
)

var ErrDesignConflict = errors.New("design was updated by another editor")

type WorkspaceRepository interface {
	GetOrCreateGuestWorkspace(ctx context.Context) (domain.Workspace, error)
	ListWorkspaces(ctx context.Context) ([]domain.Workspace, error)
	ListWorkspacesPage(ctx context.Context, options PageOptions) ([]domain.Workspace, PageInfo, error)
	GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error)
	CreateWorkspace(ctx context.Context, name string, ownerID string) (domain.Workspace, error)
	ListAccessibleWorkspacesPage(ctx context.Context, scope AccessScope, options PageOptions) ([]domain.Workspace, PageInfo, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
}

type DesignRepository interface {
	ListDesigns(ctx context.Context, workspaceID string) ([]domain.Design, error)
	ListDesignsPage(ctx context.Context, workspaceID string, options PageOptions) ([]domain.Design, PageInfo, error)
	ListAccessibleDesignsPage(ctx context.Context, workspaceID string, scope AccessScope, options PageOptions) ([]domain.Design, PageInfo, error)
	GetDesign(ctx context.Context, workspaceID string, designID string) (domain.Design, error)
	CreateDesign(ctx context.Context, workspaceID string, name string, document []byte, createdBy string) (domain.Design, error)
	UpdateDesignMetadata(ctx context.Context, workspaceID string, designID string, name string, access string) (domain.Design, error)
	UpsertDesign(ctx context.Context, design domain.Design) (domain.Design, error)
	UpdateDesignDocument(ctx context.Context, workspaceID string, designID string, document []byte, canvasSnapshot []byte, expectedRevision string) (domain.Design, error)
	DeleteDesign(ctx context.Context, workspaceID string, designID string) error
	ListDesignVersions(ctx context.Context, workspaceID string, designID string) ([]domain.DesignVersion, error)
	ListDesignVersionsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignVersion, PageInfo, error)
	CreateDesignVersion(ctx context.Context, workspaceID string, designID string, createdBy string, remarks string) (domain.DesignVersion, error)
	UpdateDraftDesignVersion(ctx context.Context, workspaceID string, designID string, versionID string, document []byte, canvasSnapshot []byte, remarks string) (domain.DesignVersion, error)
	UpdateDesignVersionStatus(ctx context.Context, workspaceID string, designID string, versionID string, status string) (domain.DesignVersion, error)
	DeleteDesignVersion(ctx context.Context, workspaceID string, designID string, versionID string) error
}

// AccessScope describes the authenticated principal used by storage-level
// visibility queries. Filtering in the query is required so pagination cannot
// hide authorized records behind inaccessible rows.
type AccessScope struct {
	UserID   string
	IsAdmin  bool
	GroupIDs []string
}

type DesignDocRepository interface {
	ListDesignDocs(ctx context.Context, workspaceID string, designID string) ([]domain.DesignDoc, error)
	GetDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) (domain.DesignDoc, error)
	CreateDesignDoc(ctx context.Context, workspaceID string, designID string, title string, body string, format string) (domain.DesignDoc, error)
	UpdateDesignDoc(ctx context.Context, workspaceID string, designID string, docID string, title string, body string, format string) (domain.DesignDoc, error)
	DeleteDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) error
}

type AIConversationRepository interface {
	ListAIConversations(ctx context.Context, workspaceID string, designID string) ([]domain.AIConversation, error)
	GetAIConversation(ctx context.Context, workspaceID string, designID string, conversationID string) (domain.AIConversation, error)
	CreateAIConversation(ctx context.Context, conversation domain.AIConversation) (domain.AIConversation, error)
	ListAIMessages(ctx context.Context, conversationID string, limit int) ([]domain.AIMessage, error)
	CreateAIMessage(ctx context.Context, message domain.AIMessage) (domain.AIMessage, error)
}

type CollaborationRepository interface {
	ListUsers(ctx context.Context) ([]domain.User, error)
	ListUsersPage(ctx context.Context, options PageOptions) ([]domain.User, PageInfo, error)
	GetUser(ctx context.Context, userID string) (domain.User, error)
	AuthenticateUser(ctx context.Context, email string, password string) (domain.User, error)
	CreateSession(ctx context.Context, userID string, expiresAt time.Time) (string, error)
	GetUserBySessionToken(ctx context.Context, token string) (domain.User, error)
	DeleteSession(ctx context.Context, token string) error
	PasswordSetupRequired(ctx context.Context) (bool, error)
	SetInitialAdminPassword(ctx context.Context, email string, password string) (domain.User, error)
	CreateFirstAdmin(ctx context.Context, displayName string, email string, password string) (domain.User, error)
	CreateUser(ctx context.Context, displayName string, email string, role string, password string) (domain.User, error)
	UpdateUser(ctx context.Context, userID string, displayName string, email string, role string, status string, password string) (domain.User, error)
	CreatePasswordResetToken(ctx context.Context, userID string, expiresAt time.Time) (string, domain.PasswordResetToken, error)
	ResetPasswordWithToken(ctx context.Context, token string, password string) (domain.User, error)
	DeleteUser(ctx context.Context, userID string) error
	GetSignInConfig(ctx context.Context) (domain.SignInConfig, error)
	GetSignInConfigWithSecret(ctx context.Context) (domain.SignInConfig, error)
	UpdateSignInConfig(ctx context.Context, config domain.SignInConfig, clientSecret string) (domain.SignInConfig, error)
	CreateOIDCFlow(ctx context.Context, flow domain.OIDCFlow) error
	ConsumeOIDCFlow(ctx context.Context, state string) (domain.OIDCFlow, error)
	ResolveOIDCUser(ctx context.Context, identity domain.OIDCIdentity, jitProvisioning bool) (domain.User, error)
	SyncUserAccessGroups(ctx context.Context, userID string, externalGroupNames []string) error
	GetAIProviderConfig(ctx context.Context) (domain.AIProviderConfig, error)
	GetAIProviderConfigWithSecret(ctx context.Context) (domain.AIProviderConfig, error)
	UpdateAIProviderConfig(ctx context.Context, config domain.AIProviderConfig, apiKey string) (domain.AIProviderConfig, error)
	GetMCPConfig(ctx context.Context) (domain.MCPConfig, error)
	UpdateMCPConfig(ctx context.Context, config domain.MCPConfig) (domain.MCPConfig, error)
	GetTelemetryIntegrationConfig(ctx context.Context) (domain.TelemetryIntegrationConfig, error)
	GetTelemetryIntegrationConfigWithSecret(ctx context.Context) (domain.TelemetryIntegrationConfig, error)
	UpdateTelemetryIntegrationConfig(ctx context.Context, config domain.TelemetryIntegrationConfig, secret string) (domain.TelemetryIntegrationConfig, error)
	ListDesignComments(ctx context.Context, workspaceID string, designID string) ([]domain.DesignComment, error)
	ListDesignCommentsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignComment, PageInfo, error)
	CreateDesignComment(ctx context.Context, workspaceID string, designID string, authorID string, body string, componentID string, connectorID string) (domain.DesignComment, error)
	ListDesignReviewRequests(ctx context.Context, workspaceID string, designID string) ([]domain.DesignReviewRequest, error)
	ListDesignReviewRequestsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignReviewRequest, PageInfo, error)
	CreateDesignReviewRequests(ctx context.Context, workspaceID string, designID string, versionID string, requestedBy string, reviewerIDs []string, message string) ([]domain.DesignReviewRequest, error)
	UpdateDesignReviewRequest(ctx context.Context, workspaceID string, designID string, reviewID string, status string, summary string) (domain.DesignReviewRequest, error)
	ListNotifications(ctx context.Context, userID string) ([]domain.Notification, error)
	MarkNotificationRead(ctx context.Context, userID string, notificationID string) error
}

type AccessRepository interface {
	ListAccessGroups(ctx context.Context) ([]domain.AccessGroup, error)
	CreateAccessGroup(ctx context.Context, group domain.AccessGroup) (domain.AccessGroup, error)
	UpdateAccessGroup(ctx context.Context, groupID string, group domain.AccessGroup) (domain.AccessGroup, error)
	DeleteAccessGroup(ctx context.Context, groupID string) error
	ListAccessGroupMembers(ctx context.Context, groupID string) ([]domain.AccessGroupMember, error)
	ReplaceAccessGroupMembers(ctx context.Context, groupID string, userIDs []string) ([]domain.AccessGroupMember, error)
	ListUserAccessGroupIDs(ctx context.Context, userID string) ([]string, error)
	ListWorkspaceAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceAccess, error)
	GrantWorkspaceAccess(ctx context.Context, access domain.WorkspaceAccess) (domain.WorkspaceAccess, error)
	RevokeWorkspaceAccess(ctx context.Context, workspaceID string, userID string) error
	ListWorkspaceGroupAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceGroupAccess, error)
	GrantWorkspaceGroupAccess(ctx context.Context, access domain.WorkspaceGroupAccess) (domain.WorkspaceGroupAccess, error)
	RevokeWorkspaceGroupAccess(ctx context.Context, workspaceID string, groupID string) error
	ListDesignAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignAccess, error)
	GrantDesignAccess(ctx context.Context, access domain.DesignAccess) (domain.DesignAccess, error)
	RevokeDesignAccess(ctx context.Context, workspaceID string, designID string, userID string) error
	ListDesignGroupAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignGroupAccess, error)
	GrantDesignGroupAccess(ctx context.Context, access domain.DesignGroupAccess) (domain.DesignGroupAccess, error)
	RevokeDesignGroupAccess(ctx context.Context, workspaceID string, designID string, groupID string) error
}

type CatalogRepository interface {
	ListCatalogAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error)
	ListCatalogAssetsPage(ctx context.Context, options PageOptions) ([]domain.CatalogAsset, PageInfo, error)
	CreateCatalogAsset(ctx context.Context, asset domain.CatalogAsset) (domain.CatalogAsset, error)
	UpdateCatalogAsset(ctx context.Context, assetID string, asset domain.CatalogAsset) (domain.CatalogAsset, error)
	DeleteCatalogAsset(ctx context.Context, assetID string) error
}

type Repository interface {
	WorkspaceRepository
	DesignRepository
	DesignDocRepository
	AIConversationRepository
	CollaborationRepository
	AccessRepository
	CatalogRepository
}

func NormalizeDesignVersionStatus(status string) string {
	return lifecycle.NormalizeVersionStatus(status)
}

func IsDesignVersionStatus(status string) bool {
	return lifecycle.IsVersionStatus(status)
}

func ValidateDesignVersionStatusTransition(from string, to string) error {
	return lifecycle.ValidateVersionStatusTransition(from, to)
}

func NewRepository(ctx context.Context, databaseURL string) (Repository, func(), error) {
	if strings.TrimSpace(databaseURL) == "" {
		return NewMemoryRepository(), func() {}, nil
	}

	repo, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		return nil, func() {}, err
	}
	return repo, repo.Close, nil
}
