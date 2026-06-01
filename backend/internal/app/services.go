package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

type RepositoryProvider interface {
	Repository() store.Repository
}

type StorageBackend interface {
	Repository() store.Repository
	Status(ctx context.Context) store.StorageStatus
	TestDatabase(ctx context.Context, databaseURL string) error
	ConfigureDatabase(ctx context.Context, databaseURL string) (store.StorageStatus, error)
	MigrateUsersToDatabase(ctx context.Context) (store.StorageStatus, int, error)
}

type StaticRepositoryProvider struct {
	Repo store.Repository
}

func (p StaticRepositoryProvider) Repository() store.Repository {
	return p.Repo
}

type Services struct {
	Identity   IdentityService
	Workspaces WorkspaceService
	Designs    DesignService
	ACL        ACLService
	Catalog    CatalogService
	Analysis   AnalysisService
	Storage    StorageAdminService
	Docs       DocsService
	Reviews    ReviewService
	Versions   VersionService
}

func NewServices(provider RepositoryProvider, storage StorageBackend) Services {
	return Services{
		Identity:   IdentityService{provider: provider},
		Workspaces: WorkspaceService{provider: provider},
		Designs:    DesignService{provider: provider},
		ACL:        ACLService{provider: provider},
		Catalog:    CatalogService{provider: provider},
		Analysis:   AnalysisService{provider: provider, analyzer: analysis.New()},
		Storage:    StorageAdminService{storage: storage},
		Docs:       DocsService{provider: provider},
		Reviews:    ReviewService{provider: provider},
		Versions:   VersionService{provider: provider},
	}
}

type IdentityService struct {
	provider RepositoryProvider
}

func (s IdentityService) repo() store.Repository { return s.provider.Repository() }

func (s IdentityService) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.repo().ListUsers(ctx)
}

func (s IdentityService) UserBySessionToken(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, errors.New("session is required")
	}
	return s.repo().GetUserBySessionToken(ctx, token)
}

func (s IdentityService) Authenticate(ctx context.Context, email string, password string) (domain.User, error) {
	return s.repo().AuthenticateUser(ctx, email, password)
}

func (s IdentityService) CreateSession(ctx context.Context, userID string, expiresAt time.Time) (string, error) {
	return s.repo().CreateSession(ctx, userID, expiresAt)
}

func (s IdentityService) DeleteSession(ctx context.Context, token string) error {
	return s.repo().DeleteSession(ctx, token)
}

func (s IdentityService) PasswordSetupRequired(ctx context.Context) (bool, error) {
	return s.repo().PasswordSetupRequired(ctx)
}

func (s IdentityService) CreateFirstAdmin(ctx context.Context, displayName string, email string, password string) (domain.User, error) {
	return s.repo().CreateFirstAdmin(ctx, displayName, email, password)
}

func (s IdentityService) SetInitialAdminPassword(ctx context.Context, email string, password string) (domain.User, error) {
	return s.repo().SetInitialAdminPassword(ctx, email, password)
}

func (s IdentityService) CreateUser(ctx context.Context, displayName string, email string, role string, password string) (domain.User, error) {
	return s.repo().CreateUser(ctx, displayName, email, role, password)
}

func (s IdentityService) UpdateUser(ctx context.Context, userID string, displayName string, email string, role string, status string, password string) (domain.User, error) {
	return s.repo().UpdateUser(ctx, userID, displayName, email, role, status, password)
}

func (s IdentityService) DeleteUser(ctx context.Context, userID string) error {
	return s.repo().DeleteUser(ctx, userID)
}

func (s IdentityService) GetSignInConfig(ctx context.Context) (domain.SignInConfig, error) {
	return s.repo().GetSignInConfig(ctx)
}

func (s IdentityService) UpdateSignInConfig(ctx context.Context, config domain.SignInConfig, clientSecret string) (domain.SignInConfig, error) {
	return s.repo().UpdateSignInConfig(ctx, config, clientSecret)
}

func (s IdentityService) GetAIProviderConfig(ctx context.Context) (domain.AIProviderConfig, error) {
	return s.repo().GetAIProviderConfig(ctx)
}

func (s IdentityService) GetAIProviderConfigWithSecret(ctx context.Context) (domain.AIProviderConfig, error) {
	return s.repo().GetAIProviderConfigWithSecret(ctx)
}

func (s IdentityService) UpdateAIProviderConfig(ctx context.Context, config domain.AIProviderConfig, apiKey string) (domain.AIProviderConfig, error) {
	return s.repo().UpdateAIProviderConfig(ctx, config, apiKey)
}

func (s IdentityService) GetMCPConfig(ctx context.Context) (domain.MCPConfig, error) {
	return s.repo().GetMCPConfig(ctx)
}

func (s IdentityService) UpdateMCPConfig(ctx context.Context, config domain.MCPConfig) (domain.MCPConfig, error) {
	return s.repo().UpdateMCPConfig(ctx, config)
}

func (s IdentityService) ListNotifications(ctx context.Context, userID string) ([]domain.Notification, error) {
	return s.repo().ListNotifications(ctx, userID)
}

func (s IdentityService) MarkNotificationRead(ctx context.Context, userID string, notificationID string) error {
	return s.repo().MarkNotificationRead(ctx, userID, notificationID)
}

type WorkspaceService struct {
	provider RepositoryProvider
}

func (s WorkspaceService) repo() store.Repository { return s.provider.Repository() }

func (s WorkspaceService) List(ctx context.Context) ([]domain.Workspace, error) {
	return s.repo().ListWorkspaces(ctx)
}

func (s WorkspaceService) Get(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	return s.repo().GetWorkspace(ctx, workspaceID)
}

func (s WorkspaceService) GetOrCreateGuest(ctx context.Context) (domain.Workspace, error) {
	return s.repo().GetOrCreateGuestWorkspace(ctx)
}

func (s WorkspaceService) Create(ctx context.Context, name string) (domain.Workspace, error) {
	return s.repo().CreateWorkspace(ctx, name)
}

func (s WorkspaceService) Delete(ctx context.Context, workspaceID string) error {
	return s.repo().DeleteWorkspace(ctx, workspaceID)
}

type DesignService struct {
	provider RepositoryProvider
}

func (s DesignService) repo() store.Repository { return s.provider.Repository() }

func (s DesignService) List(ctx context.Context, workspaceID string) ([]domain.Design, error) {
	return s.repo().ListDesigns(ctx, workspaceID)
}

func (s DesignService) Get(ctx context.Context, workspaceID string, designID string) (domain.Design, error) {
	return s.repo().GetDesign(ctx, workspaceID, designID)
}

func (s DesignService) Create(ctx context.Context, workspaceID string, name string, document []byte, createdBy string) (domain.Design, error) {
	return s.repo().CreateDesign(ctx, workspaceID, name, document, createdBy)
}

func (s DesignService) UpdateMetadata(ctx context.Context, workspaceID string, designID string, name string, access string) (domain.Design, error) {
	return s.repo().UpdateDesignMetadata(ctx, workspaceID, designID, name, access)
}

func (s DesignService) Upsert(ctx context.Context, design domain.Design) (domain.Design, error) {
	return s.repo().UpsertDesign(ctx, design)
}

func (s DesignService) Delete(ctx context.Context, workspaceID string, designID string) error {
	return s.repo().DeleteDesign(ctx, workspaceID, designID)
}

type VersionService struct {
	provider RepositoryProvider
}

func (s VersionService) repo() store.Repository { return s.provider.Repository() }

func (s VersionService) List(ctx context.Context, workspaceID string, designID string) ([]domain.DesignVersion, error) {
	return s.repo().ListDesignVersions(ctx, workspaceID, designID)
}

func (s VersionService) Create(ctx context.Context, workspaceID string, designID string, createdBy string, remarks string) (domain.DesignVersion, error) {
	return s.repo().CreateDesignVersion(ctx, workspaceID, designID, createdBy, remarks)
}

func (s VersionService) UpdateStatus(ctx context.Context, workspaceID string, designID string, versionID string, status string) (domain.DesignVersion, error) {
	return s.repo().UpdateDesignVersionStatus(ctx, workspaceID, designID, versionID, status)
}

func (s VersionService) Delete(ctx context.Context, workspaceID string, designID string, versionID string) error {
	return s.repo().DeleteDesignVersion(ctx, workspaceID, designID, versionID)
}

type DocsService struct {
	provider RepositoryProvider
}

func (s DocsService) repo() store.Repository { return s.provider.Repository() }

func (s DocsService) List(ctx context.Context, workspaceID string, designID string) ([]domain.DesignDoc, error) {
	return s.repo().ListDesignDocs(ctx, workspaceID, designID)
}

func (s DocsService) Get(ctx context.Context, workspaceID string, designID string, docID string) (domain.DesignDoc, error) {
	return s.repo().GetDesignDoc(ctx, workspaceID, designID, docID)
}

func (s DocsService) Create(ctx context.Context, workspaceID string, designID string, title string, body string, format string) (domain.DesignDoc, error) {
	return s.repo().CreateDesignDoc(ctx, workspaceID, designID, title, body, format)
}

func (s DocsService) Update(ctx context.Context, workspaceID string, designID string, docID string, title string, body string, format string) (domain.DesignDoc, error) {
	return s.repo().UpdateDesignDoc(ctx, workspaceID, designID, docID, title, body, format)
}

func (s DocsService) Delete(ctx context.Context, workspaceID string, designID string, docID string) error {
	return s.repo().DeleteDesignDoc(ctx, workspaceID, designID, docID)
}

type ReviewService struct {
	provider RepositoryProvider
}

func (s ReviewService) repo() store.Repository { return s.provider.Repository() }

func (s ReviewService) ListComments(ctx context.Context, workspaceID string, designID string) ([]domain.DesignComment, error) {
	return s.repo().ListDesignComments(ctx, workspaceID, designID)
}

func (s ReviewService) CreateComment(ctx context.Context, workspaceID string, designID string, authorID string, body string, componentID string, connectorID string) (domain.DesignComment, error) {
	return s.repo().CreateDesignComment(ctx, workspaceID, designID, authorID, body, componentID, connectorID)
}

func (s ReviewService) ListReviews(ctx context.Context, workspaceID string, designID string) ([]domain.DesignReviewRequest, error) {
	return s.repo().ListDesignReviewRequests(ctx, workspaceID, designID)
}

func (s ReviewService) CreateReviews(ctx context.Context, workspaceID string, designID string, versionID string, requestedBy string, reviewerIDs []string, message string) ([]domain.DesignReviewRequest, error) {
	return s.repo().CreateDesignReviewRequests(ctx, workspaceID, designID, versionID, requestedBy, reviewerIDs, message)
}

func (s ReviewService) UpdateReview(ctx context.Context, workspaceID string, designID string, reviewID string, status string, summary string) (domain.DesignReviewRequest, error) {
	return s.repo().UpdateDesignReviewRequest(ctx, workspaceID, designID, reviewID, status, summary)
}

type ACLService struct {
	provider RepositoryProvider
}

func (s ACLService) repo() store.Repository { return s.provider.Repository() }

func (s ACLService) ListWorkspaceAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceAccess, error) {
	return s.repo().ListWorkspaceAccess(ctx, workspaceID)
}

func (s ACLService) GrantWorkspaceAccess(ctx context.Context, access domain.WorkspaceAccess) (domain.WorkspaceAccess, error) {
	return s.repo().GrantWorkspaceAccess(ctx, access)
}

func (s ACLService) RevokeWorkspaceAccess(ctx context.Context, workspaceID string, userID string) error {
	return s.repo().RevokeWorkspaceAccess(ctx, workspaceID, userID)
}

func (s ACLService) ListWorkspaceGroupAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceGroupAccess, error) {
	return s.repo().ListWorkspaceGroupAccess(ctx, workspaceID)
}

func (s ACLService) GrantWorkspaceGroupAccess(ctx context.Context, access domain.WorkspaceGroupAccess) (domain.WorkspaceGroupAccess, error) {
	return s.repo().GrantWorkspaceGroupAccess(ctx, access)
}

func (s ACLService) RevokeWorkspaceGroupAccess(ctx context.Context, workspaceID string, groupID string) error {
	return s.repo().RevokeWorkspaceGroupAccess(ctx, workspaceID, groupID)
}

func (s ACLService) ListDesignAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignAccess, error) {
	return s.repo().ListDesignAccess(ctx, workspaceID, designID)
}

func (s ACLService) GrantDesignAccess(ctx context.Context, access domain.DesignAccess) (domain.DesignAccess, error) {
	return s.repo().GrantDesignAccess(ctx, access)
}

func (s ACLService) RevokeDesignAccess(ctx context.Context, workspaceID string, designID string, userID string) error {
	return s.repo().RevokeDesignAccess(ctx, workspaceID, designID, userID)
}

func (s ACLService) ListDesignGroupAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignGroupAccess, error) {
	return s.repo().ListDesignGroupAccess(ctx, workspaceID, designID)
}

func (s ACLService) GrantDesignGroupAccess(ctx context.Context, access domain.DesignGroupAccess) (domain.DesignGroupAccess, error) {
	return s.repo().GrantDesignGroupAccess(ctx, access)
}

func (s ACLService) RevokeDesignGroupAccess(ctx context.Context, workspaceID string, designID string, groupID string) error {
	return s.repo().RevokeDesignGroupAccess(ctx, workspaceID, designID, groupID)
}

func (s ACLService) ListGroups(ctx context.Context) ([]domain.AccessGroup, error) {
	return s.repo().ListAccessGroups(ctx)
}

func (s ACLService) CreateGroup(ctx context.Context, group domain.AccessGroup) (domain.AccessGroup, error) {
	return s.repo().CreateAccessGroup(ctx, group)
}

func (s ACLService) UpdateGroup(ctx context.Context, groupID string, group domain.AccessGroup) (domain.AccessGroup, error) {
	return s.repo().UpdateAccessGroup(ctx, groupID, group)
}

func (s ACLService) DeleteGroup(ctx context.Context, groupID string) error {
	return s.repo().DeleteAccessGroup(ctx, groupID)
}

func (s ACLService) ListGroupMembers(ctx context.Context, groupID string) ([]domain.AccessGroupMember, error) {
	return s.repo().ListAccessGroupMembers(ctx, groupID)
}

func (s ACLService) ReplaceGroupMembers(ctx context.Context, groupID string, userIDs []string) ([]domain.AccessGroupMember, error) {
	return s.repo().ReplaceAccessGroupMembers(ctx, groupID, userIDs)
}

func (s ACLService) ListUserGroupIDs(ctx context.Context, userID string) ([]string, error) {
	return s.repo().ListUserAccessGroupIDs(ctx, userID)
}

type CatalogService struct {
	provider RepositoryProvider
}

func (s CatalogService) repo() store.Repository { return s.provider.Repository() }

func (s CatalogService) ListAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error) {
	return s.repo().ListCatalogAssets(ctx, query)
}

func (s CatalogService) CreateAsset(ctx context.Context, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	return s.repo().CreateCatalogAsset(ctx, asset)
}

func (s CatalogService) UpdateAsset(ctx context.Context, assetID string, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	return s.repo().UpdateCatalogAsset(ctx, assetID, asset)
}

func (s CatalogService) DeleteAsset(ctx context.Context, assetID string) error {
	return s.repo().DeleteCatalogAsset(ctx, assetID)
}

type AnalysisService struct {
	provider RepositoryProvider
	analyzer analysis.Analyzer
}

func (s AnalysisService) AnalyzeDocument(raw json.RawMessage) (analysis.Report, error) {
	return s.analyzer.Analyze(raw)
}

type StorageAdminService struct {
	storage StorageBackend
}

func (s StorageAdminService) Configurable() bool {
	return s.storage != nil
}

func (s StorageAdminService) Status(ctx context.Context) store.StorageStatus {
	if s.storage == nil {
		return store.StorageStatus{Mode: store.StorageModeDatabase, DatabaseConnected: true}
	}
	return s.storage.Status(ctx)
}

func (s StorageAdminService) TestDatabase(ctx context.Context, databaseURL string) error {
	if s.storage == nil {
		return errors.New("storage engine is not configurable")
	}
	return s.storage.TestDatabase(ctx, databaseURL)
}

func (s StorageAdminService) ConfigureDatabase(ctx context.Context, databaseURL string) (store.StorageStatus, error) {
	if s.storage == nil {
		return store.StorageStatus{}, errors.New("storage engine is not configurable")
	}
	return s.storage.ConfigureDatabase(ctx, databaseURL)
}

func (s StorageAdminService) MigrateUsersToDatabase(ctx context.Context) (store.StorageStatus, int, error) {
	if s.storage == nil {
		return store.StorageStatus{}, 0, errors.New("storage engine is not configurable")
	}
	return s.storage.MigrateUsersToDatabase(ctx)
}
