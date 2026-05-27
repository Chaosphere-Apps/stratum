package store

import (
	"context"
	"strings"
	"time"

	"github.com/system-design-evaluator/backend/internal/domain"
)

type WorkspaceRepository interface {
	GetOrCreateGuestWorkspace(ctx context.Context) (domain.Workspace, error)
	ListWorkspaces(ctx context.Context) ([]domain.Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error)
	CreateWorkspace(ctx context.Context, name string) (domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
}

type DesignRepository interface {
	ListDesigns(ctx context.Context, workspaceID string) ([]domain.Design, error)
	GetDesign(ctx context.Context, workspaceID string, designID string) (domain.Design, error)
	CreateDesign(ctx context.Context, workspaceID string, name string, document []byte, createdBy string) (domain.Design, error)
	UpdateDesignMetadata(ctx context.Context, workspaceID string, designID string, name string, access string) (domain.Design, error)
	UpsertDesign(ctx context.Context, design domain.Design) (domain.Design, error)
	DeleteDesign(ctx context.Context, workspaceID string, designID string) error
	ListDesignVersions(ctx context.Context, workspaceID string, designID string) ([]domain.DesignVersion, error)
}

type DesignDocRepository interface {
	ListDesignDocs(ctx context.Context, workspaceID string, designID string) ([]domain.DesignDoc, error)
	GetDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) (domain.DesignDoc, error)
	CreateDesignDoc(ctx context.Context, workspaceID string, designID string, title string, body string, format string) (domain.DesignDoc, error)
	UpdateDesignDoc(ctx context.Context, workspaceID string, designID string, docID string, title string, body string, format string) (domain.DesignDoc, error)
	DeleteDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) error
}

type CollaborationRepository interface {
	ListUsers(ctx context.Context) ([]domain.User, error)
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
	DeleteUser(ctx context.Context, userID string) error
	GetSignInConfig(ctx context.Context) (domain.SignInConfig, error)
	UpdateSignInConfig(ctx context.Context, config domain.SignInConfig, clientSecret string) (domain.SignInConfig, error)
	GetAIProviderConfig(ctx context.Context) (domain.AIProviderConfig, error)
	GetAIProviderConfigWithSecret(ctx context.Context) (domain.AIProviderConfig, error)
	UpdateAIProviderConfig(ctx context.Context, config domain.AIProviderConfig, apiKey string) (domain.AIProviderConfig, error)
	GetMCPConfig(ctx context.Context) (domain.MCPConfig, error)
	UpdateMCPConfig(ctx context.Context, config domain.MCPConfig) (domain.MCPConfig, error)
	ListDesignComments(ctx context.Context, workspaceID string, designID string) ([]domain.DesignComment, error)
	CreateDesignComment(ctx context.Context, workspaceID string, designID string, authorID string, body string, componentID string, connectorID string) (domain.DesignComment, error)
	ListDesignReviewRequests(ctx context.Context, workspaceID string, designID string) ([]domain.DesignReviewRequest, error)
	CreateDesignReviewRequest(ctx context.Context, workspaceID string, designID string, requestedBy string, reviewerID string, message string) (domain.DesignReviewRequest, error)
	UpdateDesignReviewRequest(ctx context.Context, workspaceID string, designID string, reviewID string, status string, summary string) (domain.DesignReviewRequest, error)
	ListNotifications(ctx context.Context, userID string) ([]domain.Notification, error)
	MarkNotificationRead(ctx context.Context, userID string, notificationID string) error
}

type AccessRepository interface {
	ListWorkspaceAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceAccess, error)
	GrantWorkspaceAccess(ctx context.Context, access domain.WorkspaceAccess) (domain.WorkspaceAccess, error)
	RevokeWorkspaceAccess(ctx context.Context, workspaceID string, userID string) error
	ListDesignAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignAccess, error)
	GrantDesignAccess(ctx context.Context, access domain.DesignAccess) (domain.DesignAccess, error)
	RevokeDesignAccess(ctx context.Context, workspaceID string, designID string, userID string) error
}

type CatalogRepository interface {
	ListCatalogAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error)
	CreateCatalogAsset(ctx context.Context, asset domain.CatalogAsset) (domain.CatalogAsset, error)
	UpdateCatalogAsset(ctx context.Context, assetID string, asset domain.CatalogAsset) (domain.CatalogAsset, error)
	DeleteCatalogAsset(ctx context.Context, assetID string) error
}

type Repository interface {
	WorkspaceRepository
	DesignRepository
	DesignDocRepository
	CollaborationRepository
	AccessRepository
	CatalogRepository
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
