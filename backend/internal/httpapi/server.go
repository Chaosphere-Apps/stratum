package httpapi

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/app"
	"github.com/system-design-evaluator/backend/internal/authn"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/modelgateway"
	"github.com/system-design-evaluator/backend/internal/policy"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/security"
	"github.com/system-design-evaluator/backend/internal/store"
	"log/slog"
	"net/http"
)

type Server struct {
	cfg      config.Config
	hub      *realtime.Hub
	storage  *store.StorageEngine
	services app.Services
	authz    policy.Authorizer
	log      *slog.Logger
	mux      *http.ServeMux
	authRate *security.RateLimiter
	aiRate   *security.RateLimiter
	oidc     oidcProtocol
	ai       modelgateway.Gateway
}

type oidcProtocol interface {
	Begin(context.Context, domain.SignInConfig) (authn.Authorization, error)
	Complete(context.Context, domain.SignInConfig, string, domain.OIDCFlow) (domain.OIDCIdentity, error)
	VerifyConfiguration(context.Context, domain.SignInConfig) error
}

func NewServer(cfg config.Config, hub *realtime.Hub, log *slog.Logger, storageEngine ...*store.StorageEngine) *Server {
	var storage *store.StorageEngine
	if len(storageEngine) > 0 {
		storage = storageEngine[0]
	}
	var provider app.RepositoryProvider
	if storage != nil {
		provider = storage
	} else if hub != nil {
		provider = app.StaticRepositoryProvider{Repo: hub.Repository()}
	} else {
		provider = app.StaticRepositoryProvider{}
	}
	var storageBackend app.StorageBackend
	if storage != nil {
		storageBackend = storage
	}
	server := &Server{
		cfg:      cfg,
		hub:      hub,
		storage:  storage,
		services: app.NewServices(provider, storageBackend),
		authz:    policy.NewAuthorizer(provider),
		log:      log,
		mux:      http.NewServeMux(),
		authRate: security.NewRateLimiter(),
		aiRate:   security.NewRateLimiter(),
		oidc:     authn.NewOIDCClient(),
		ai:       modelgateway.New(cfg.AllowPrivateAIProviderURLs),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.recoverPanic(s.cors(s.csrfGuard(requestLogger(s.log, s.mux)))))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.HandleFunc("GET /ws", s.handleWorkspaceSocket)
	s.mux.HandleFunc("GET /api/storage/status", s.handleStorageStatus)
	s.mux.HandleFunc("GET /api/setup/status", s.handleSetupStatus)
	s.mux.HandleFunc("POST /api/setup/first-admin", s.handleCreateFirstAdmin)
	s.mux.HandleFunc("POST /api/setup/admin-password", s.handleSetInitialAdminPassword)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/auth/config", s.handleAuthConfig)
	s.mux.HandleFunc("GET /api/auth/oidc/start", s.handleOIDCStart)
	s.mux.HandleFunc("GET /api/auth/oidc/callback", s.handleOIDCCallback)
	s.mux.HandleFunc("POST /api/auth/password-reset", s.handleResetPassword)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/profile", s.handleProfile)
	s.mux.HandleFunc("GET /api/users", s.handleListUsers)
	s.mux.HandleFunc("POST /api/admin/users", s.handleCreateUser)
	s.mux.HandleFunc("PATCH /api/admin/users/{userID}", s.handleUpdateUser)
	s.mux.HandleFunc("POST /api/admin/users/{userID}/password-reset-link", s.handleCreatePasswordResetLink)
	s.mux.HandleFunc("DELETE /api/admin/users/{userID}", s.handleDeleteUser)
	s.mux.HandleFunc("GET /api/admin/sign-in", s.handleGetSignInConfig)
	s.mux.HandleFunc("PATCH /api/admin/sign-in", s.handleUpdateSignInConfig)
	s.mux.HandleFunc("POST /api/admin/sign-in/verify", s.handleVerifySignInConfig)
	s.mux.HandleFunc("GET /api/admin/ai-provider", s.handleGetAIProviderConfig)
	s.mux.HandleFunc("PATCH /api/admin/ai-provider", s.handleUpdateAIProviderConfig)
	s.mux.HandleFunc("GET /api/admin/mcp", s.handleGetMCPConfig)
	s.mux.HandleFunc("PATCH /api/admin/mcp", s.handleUpdateMCPConfig)
	s.mux.HandleFunc("GET /api/admin/telemetry", s.handleGetTelemetryIntegrationConfig)
	s.mux.HandleFunc("PATCH /api/admin/telemetry", s.handleUpdateTelemetryIntegrationConfig)
	s.mux.HandleFunc("GET /api/admin/storage", s.handleAdminStorageStatus)
	s.mux.HandleFunc("POST /api/admin/storage/test", s.handleTestDatabaseStorage)
	s.mux.HandleFunc("PATCH /api/admin/storage/database", s.handleConfigureDatabaseStorage)
	s.mux.HandleFunc("POST /api/admin/storage/migrate-users", s.handleMigrateStorageUsers)
	s.mux.HandleFunc("GET /api/admin/access/users/{userID}", s.handleListUserAccess)
	s.mux.HandleFunc("GET /api/admin/access/groups", s.handleListAccessGroups)
	s.mux.HandleFunc("POST /api/admin/access/groups", s.handleCreateAccessGroup)
	s.mux.HandleFunc("PATCH /api/admin/access/groups/{groupID}", s.handleUpdateAccessGroup)
	s.mux.HandleFunc("DELETE /api/admin/access/groups/{groupID}", s.handleDeleteAccessGroup)
	s.mux.HandleFunc("GET /api/admin/access/groups/{groupID}/members", s.handleListAccessGroupMembers)
	s.mux.HandleFunc("PUT /api/admin/access/groups/{groupID}/members", s.handleReplaceAccessGroupMembers)
	s.mux.HandleFunc("GET /api/catalog/assets", s.handleListCatalogAssets)
	s.mux.HandleFunc("POST /api/catalog/assets", s.handleCreateCatalogAsset)
	s.mux.HandleFunc("PATCH /api/admin/catalog/assets/{assetID}", s.handleUpdateCatalogAsset)
	s.mux.HandleFunc("DELETE /api/admin/catalog/assets/{assetID}", s.handleDeleteCatalogAsset)
	s.mux.HandleFunc("GET /api/notifications", s.handleListNotifications)
	s.mux.HandleFunc("POST /api/notifications/{notificationID}/read", s.handleMarkNotificationRead)
	s.mux.HandleFunc("POST /api/ai/provider/verify", s.handleVerifyAIProvider)
	s.mux.HandleFunc("GET /api/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("GET /api/workspace-share-principals", s.handleListWorkspaceSharePrincipals)
	s.mux.HandleFunc("POST /api/workspaces", s.handleCreateWorkspace)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}", s.handleDeleteWorkspace)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/access", s.handleListWorkspaceAccess)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/access", s.handleGrantWorkspaceAccess)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/access/{userID}", s.handleRevokeWorkspaceAccess)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/group-access", s.handleListWorkspaceGroupAccess)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/group-access", s.handleGrantWorkspaceGroupAccess)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/group-access/{groupID}", s.handleRevokeWorkspaceGroupAccess)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs", s.handleListDesigns)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs", s.handleCreateDesign)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}", s.handleGetDesign)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/designs/{designID}", s.handleDeleteDesign)
	s.mux.HandleFunc("PATCH /api/workspaces/{workspaceID}/designs/{designID}", s.handleUpdateDesignMetadata)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/access", s.handleListDesignAccess)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/access", s.handleGrantDesignAccess)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/designs/{designID}/access/{userID}", s.handleRevokeDesignAccess)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/group-access", s.handleListDesignGroupAccess)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/group-access", s.handleGrantDesignGroupAccess)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/designs/{designID}/group-access/{groupID}", s.handleRevokeDesignGroupAccess)
	s.mux.HandleFunc("PUT /api/workspaces/{workspaceID}/designs/{designID}/document", s.handleSaveDesignDocument)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/analysis", s.handleAnalyzeDesign)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations", s.handleListAIConversations)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations", s.handleCreateAIConversation)
	s.mux.HandleFunc("PATCH /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations/{conversationID}", s.handleUpdateAIConversation)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations/{conversationID}/messages", s.handleListAIMessages)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations/{conversationID}/messages", s.handleCreateAIMessage)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/ai/conversations/{conversationID}/apply", s.handleApplyAIDesignProposal)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/versions", s.handleListDesignVersions)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/versions", s.handleCreateDesignVersion)
	s.mux.HandleFunc("PATCH /api/workspaces/{workspaceID}/designs/{designID}/versions/{versionID}", s.handleUpdateDesignVersionStatus)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/designs/{designID}/versions/{versionID}", s.handleDeleteDesignVersion)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/docs", s.handleListDesignDocs)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/docs", s.handleCreateDesignDoc)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}", s.handleGetDesignDoc)
	s.mux.HandleFunc("PATCH /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}", s.handleUpdateDesignDoc)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/designs/{designID}/docs/{docID}", s.handleDeleteDesignDoc)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/comments", s.handleListDesignComments)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/comments", s.handleCreateDesignComment)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/reviews", s.handleListDesignReviews)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/reviews", s.handleCreateDesignReview)
	s.mux.HandleFunc("PATCH /api/workspaces/{workspaceID}/designs/{designID}/reviews/{reviewID}", s.handleUpdateDesignReview)
	if s.cfg.StaticAssetsDir != "" {
		s.mux.HandleFunc("GET /", s.handleStaticAssets)
	}
}
