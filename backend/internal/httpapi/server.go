package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	urlpath "path"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/app"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/policy"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
)

type Server struct {
	cfg      config.Config
	hub      *realtime.Hub
	storage  *store.StorageEngine
	services app.Services
	authz    policy.Authorizer
	log      *slog.Logger
	mux      *http.ServeMux
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
	s.mux.HandleFunc("GET /api/admin/ai-provider", s.handleGetAIProviderConfig)
	s.mux.HandleFunc("PATCH /api/admin/ai-provider", s.handleUpdateAIProviderConfig)
	s.mux.HandleFunc("GET /api/admin/mcp", s.handleGetMCPConfig)
	s.mux.HandleFunc("PATCH /api/admin/mcp", s.handleUpdateMCPConfig)
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

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	status := s.storageStatus(r.Context())
	httpStatus := http.StatusOK
	if status.Mode == store.StorageModeDatabase && !status.DatabaseConnected {
		httpStatus = http.StatusServiceUnavailable
	}
	writeJSON(w, httpStatus, map[string]any{
		"status":  map[bool]string{true: "ready", false: "not_ready"}[httpStatus == http.StatusOK],
		"storage": status,
	})
}

func (s *Server) storageStatus(ctx context.Context) store.StorageStatus {
	return s.services.Storage.Status(ctx)
}

func (s *Server) handleStorageStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"storage": s.storageStatus(r.Context())})
}

func (s *Server) handleAdminStorageStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"storage": s.storageStatus(r.Context())})
}

func (s *Server) handleTestDatabaseStorage(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		DatabaseURL string `json:"databaseUrl"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.services.Storage.TestDatabase(r.Context(), body.DatabaseURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "storage": s.storageStatus(r.Context())})
}

func (s *Server) handleConfigureDatabaseStorage(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		DatabaseURL string `json:"databaseUrl"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	status, err := s.services.Storage.ConfigureDatabase(r.Context(), body.DatabaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.hub.SetRepository(s.storage.Repository())
	writeJSON(w, http.StatusOK, map[string]any{"storage": status})
}

func (s *Server) handleMigrateStorageUsers(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return
	}
	if !policy.RoleAllows(user.Role, "admin") {
		writeError(w, http.StatusForbidden, "admin role is required")
		return
	}
	status, migrated, err := s.services.Storage.MigrateUsersToDatabase(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.hub.SetRepository(s.storage.Repository())
	token, err := s.services.Identity.CreateSession(r.Context(), user.ID, s.sessionExpiresAt())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"storage": status, "migratedUsers": migrated})
}

func (s *Server) handleStaticAssets(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" || r.URL.Path == "/healthz" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	cleanPath := strings.TrimPrefix(urlpath.Clean("/"+r.URL.Path), "/")
	if cleanPath == "" {
		cleanPath = "index.html"
	}

	staticFile := filepath.Join(s.cfg.StaticAssetsDir, cleanPath)
	if info, err := os.Stat(staticFile); err == nil && !info.IsDir() {
		http.ServeFile(w, r, staticFile)
		return
	}

	http.ServeFile(w, r, filepath.Join(s.cfg.StaticAssetsDir, "index.html"))
}

func (s *Server) handleWorkspaceSocket(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		http.Error(w, "session is required", http.StatusUnauthorized)
		return
	}
	workspaceID := strings.TrimSpace(r.URL.Query().Get("workspaceId"))
	if workspaceID == "" {
		workspaceID = domain.GuestWorkspaceID
	}
	if _, err := s.services.Workspaces.Get(r.Context(), workspaceID); err != nil {
		http.Error(w, "workspace not found", http.StatusNotFound)
		return
	}
	if !s.canAccessWorkspace(r.Context(), user, workspaceID, policy.WorkspaceRead) {
		http.Error(w, "workspace read access is required", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: s.cfg.AllowedOrigins,
	})
	if err != nil {
		s.log.Warn("websocket accept failed", "error", err)
		return
	}
	conn.SetReadLimit(s.cfg.WebSocketReadLimit)

	ctx := r.Context()
	snapshot, err := s.hub.Snapshot(ctx, workspaceID)
	if err != nil {
		s.log.Error("failed to create workspace snapshot", "error", err)
		_ = conn.Close(websocket.StatusInternalError, "snapshot failed")
		return
	}

	client := realtime.NewClient(conn, s.hub, s.log, workspaceID, user.ID, user.Role, policy.RoleAllows(user.Role, "admin"))
	payload, err := json.Marshal(realtime.WorkspaceSnapshotPayload{Snapshot: snapshot})
	if err != nil {
		s.log.Error("failed to encode workspace snapshot", "error", err)
		_ = conn.Close(websocket.StatusInternalError, "snapshot encode failed")
		return
	}
	client.Send(realtime.Envelope{
		Type:    realtime.MessageWorkspaceSnapshot,
		Payload: payload,
	})
	client.Run(ctx)
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	users, err := s.services.Identity.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	passwordSetupRequired, err := s.services.Identity.PasswordSetupRequired(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"requiresSetup":         len(users) == 0,
		"requiresPasswordSetup": passwordSetupRequired,
		"storage":               s.storageStatus(r.Context()),
	})
}

func (s *Server) handleCreateFirstAdmin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := s.services.Identity.CreateFirstAdmin(r.Context(), body.DisplayName, body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, err := s.services.Identity.CreateSession(r.Context(), user.ID, s.sessionExpiresAt())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (s *Server) handleSetInitialAdminPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := s.services.Identity.SetInitialAdminPassword(r.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, err := s.services.Identity.CreateSession(r.Context(), user.ID, s.sessionExpiresAt())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := s.services.Identity.Authenticate(r.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	token, err := s.services.Identity.CreateSession(r.Context(), user.ID, s.sessionExpiresAt())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := s.services.Identity.ResetPasswordWithToken(r.Context(), body.Token, body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r)
	if token != "" {
		_ = s.services.Identity.DeleteSession(r.Context(), token)
	}
	s.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		users, listErr := s.services.Identity.ListUsers(r.Context())
		if listErr != nil {
			writeError(w, http.StatusInternalServerError, listErr.Error())
			return
		}
		if len(users) > 0 {
			writeError(w, http.StatusUnauthorized, "session is required")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"setupRequired": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          user.ID,
		"displayName": user.DisplayName,
		"email":       user.Email,
		"role":        user.Role,
		"status":      user.Status,
		"lastSeenAt":  user.LastSeenAt,
		"createdAt":   user.CreatedAt,
		"updatedAt":   user.UpdatedAt,
		"passwordSet": user.PasswordSet,
		"storage":     s.storageStatus(r.Context()),
	})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if _, err := s.currentUser(r); err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return
	}
	users, page, err := s.services.Identity.ListUsersPage(r.Context(), pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users, "page": page})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Password    string `json:"password"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := s.services.Identity.CreateUser(r.Context(), body.DisplayName, body.Email, body.Role, body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Status      string `json:"status"`
		Password    string `json:"password"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := s.services.Identity.UpdateUser(r.Context(), r.PathValue("userID"), body.DisplayName, body.Email, body.Role, body.Status, body.Password)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleCreatePasswordResetLink(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rawToken, reset, err := s.services.Identity.CreatePasswordResetToken(r.Context(), r.PathValue("userID"), time.Now().UTC().Add(time.Hour))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"resetLink": s.passwordResetLink(r, rawToken),
		"expiresAt": reset.ExpiresAt,
	})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.services.Identity.DeleteUser(r.Context(), r.PathValue("userID")); err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetSignInConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	config, err := s.services.Identity.GetSignInConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"signIn": config})
}

func (s *Server) handleUpdateSignInConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		LocalPasswordEnabled  bool   `json:"localPasswordEnabled"`
		SSOEnabled            bool   `json:"ssoEnabled"`
		Provider              string `json:"provider"`
		OktaDomain            string `json:"oktaDomain"`
		Issuer                string `json:"issuer"`
		ClientID              string `json:"clientId"`
		ClientSecret          string `json:"clientSecret"`
		RedirectURI           string `json:"redirectUri"`
		PostLogoutRedirectURI string `json:"postLogoutRedirectUri"`
		Scopes                string `json:"scopes"`
		GroupsClaim           string `json:"groupsClaim"`
		AdminGroup            string `json:"adminGroup"`
		ReviewerGroup         string `json:"reviewerGroup"`
		JITProvisioning       bool   `json:"jitProvisioning"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	config, err := s.services.Identity.UpdateSignInConfig(r.Context(), domain.SignInConfig{
		LocalPasswordEnabled:  body.LocalPasswordEnabled,
		SSOEnabled:            body.SSOEnabled,
		Provider:              body.Provider,
		OktaDomain:            body.OktaDomain,
		Issuer:                body.Issuer,
		ClientID:              body.ClientID,
		RedirectURI:           body.RedirectURI,
		PostLogoutRedirectURI: body.PostLogoutRedirectURI,
		Scopes:                body.Scopes,
		GroupsClaim:           body.GroupsClaim,
		AdminGroup:            body.AdminGroup,
		ReviewerGroup:         body.ReviewerGroup,
		JITProvisioning:       body.JITProvisioning,
	}, body.ClientSecret)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"signIn": config})
}

func (s *Server) handleGetAIProviderConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	config, err := s.services.Identity.GetAIProviderConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"aiProvider": config})
}

func (s *Server) handleUpdateAIProviderConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Enabled  bool   `json:"enabled"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		BaseURL  string `json:"baseUrl"`
		APIKey   string `json:"apiKey"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	verifyKey := strings.TrimSpace(body.APIKey)
	if body.Enabled && verifyKey == "" {
		current, err := s.services.Identity.GetAIProviderConfigWithSecret(r.Context())
		if err == nil {
			verifyKey = current.APIKey
		}
	}
	if body.Enabled || verifyKey != "" {
		if err := verifyAIProvider(r.Context(), body.Provider, body.BaseURL, verifyKey, s.cfg.AllowPrivateAIProviderURLs); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	config, err := s.services.Identity.UpdateAIProviderConfig(r.Context(), domain.AIProviderConfig{
		Enabled:  body.Enabled,
		Provider: body.Provider,
		Model:    body.Model,
		BaseURL:  body.BaseURL,
	}, body.APIKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"aiProvider": config})
}

func (s *Server) handleGetMCPConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	config, err := s.services.Identity.GetMCPConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mcp": config})
}

func (s *Server) handleUpdateMCPConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Enabled             bool   `json:"enabled"`
		EndpointPath        string `json:"endpointPath"`
		ReadCatalog         bool   `json:"readCatalog"`
		ReadDesigns         bool   `json:"readDesigns"`
		CreateDraftDesign   bool   `json:"createDraftDesign"`
		RunAnalysis         bool   `json:"runAnalysis"`
		FetchImpactReport   bool   `json:"fetchImpactReport"`
		RequireAdminConsent bool   `json:"requireAdminConsent"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	config, err := s.services.Identity.UpdateMCPConfig(r.Context(), domain.MCPConfig{
		Enabled:             body.Enabled,
		EndpointPath:        body.EndpointPath,
		ReadCatalog:         body.ReadCatalog,
		ReadDesigns:         body.ReadDesigns,
		CreateDraftDesign:   body.CreateDraftDesign,
		RunAnalysis:         body.RunAnalysis,
		FetchImpactReport:   body.FetchImpactReport,
		RequireAdminConsent: body.RequireAdminConsent,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mcp": config})
}

func (s *Server) handleListCatalogAssets(w http.ResponseWriter, r *http.Request) {
	if _, err := s.currentUser(r); err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return
	}
	assets, page, err := s.services.Catalog.ListAssetsPage(r.Context(), pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets, "page": page})
}

func (s *Server) handleCreateCatalogAsset(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireRole(w, r, "architect")
	if !ok {
		return
	}
	var body struct {
		Name        string          `json:"name"`
		Type        string          `json:"type"`
		Owner       string          `json:"owner"`
		Description string          `json:"description"`
		Criticality string          `json:"criticality"`
		Tags        []string        `json:"tags"`
		Metadata    json.RawMessage `json:"metadata"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asset, err := s.services.Catalog.CreateAsset(r.Context(), domain.CatalogAsset{
		Name:        body.Name,
		Type:        body.Type,
		Owner:       body.Owner,
		Description: body.Description,
		Criticality: body.Criticality,
		Tags:        body.Tags,
		Metadata:    body.Metadata,
		CreatedBy:   user.ID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"asset": asset})
}

func (s *Server) handleUpdateCatalogAsset(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Name        string          `json:"name"`
		Type        string          `json:"type"`
		Owner       string          `json:"owner"`
		Description string          `json:"description"`
		Criticality string          `json:"criticality"`
		Tags        []string        `json:"tags"`
		Metadata    json.RawMessage `json:"metadata"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	asset, err := s.services.Catalog.UpdateAsset(r.Context(), r.PathValue("assetID"), domain.CatalogAsset{
		Name:        body.Name,
		Type:        body.Type,
		Owner:       body.Owner,
		Description: body.Description,
		Criticality: body.Criticality,
		Tags:        body.Tags,
		Metadata:    body.Metadata,
	})
	if err != nil {
		writeError(w, statusForCatalogError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"asset": asset})
}

func (s *Server) handleDeleteCatalogAsset(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.services.Catalog.DeleteAsset(r.Context(), r.PathValue("assetID")); err != nil {
		writeError(w, statusForCatalogError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"notifications": []domain.Notification{}})
		return
	}
	notifications, err := s.services.Identity.ListNotifications(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": notifications})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user is required")
		return
	}
	if err := s.services.Identity.MarkNotificationRead(r.Context(), user.ID, r.PathValue("notificationID")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleVerifyAIProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		BaseURL  string `json:"baseUrl"`
		APIKey   string `json:"apiKey"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := verifyAIProvider(r.Context(), body.Provider, body.BaseURL, body.APIKey, s.cfg.AllowPrivateAIProviderURLs); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"message":    "Provider connection verified",
		"provider":   strings.TrimSpace(body.Provider),
		"model":      strings.TrimSpace(body.Model),
		"verifiedAt": time.Now().UTC(),
	})
}

func verifyAIProvider(ctx context.Context, provider string, baseURL string, apiKey string, allowPrivateURLs bool) error {
	provider = strings.TrimSpace(provider)
	apiKey = strings.TrimSpace(apiKey)
	if provider == "" {
		return errors.New("provider is required")
	}
	if apiKey == "" {
		return errors.New("API key is required")
	}

	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint == "" {
		switch provider {
		case "openai":
			endpoint = "https://api.openai.com/v1"
		case "anthropic":
			endpoint = "https://api.anthropic.com/v1"
		case "openrouter":
			endpoint = "https://openrouter.ai/api/v1"
		default:
			return errors.New("base URL is required for custom providers")
		}
	}
	if err := validateProviderEndpoint(endpoint, allowPrivateURLs); err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/models", nil)
	if err != nil {
		return errors.New("provider URL is invalid")
	}
	if provider == "anthropic" {
		request.Header.Set("x-api-key", apiKey)
		request.Header.Set("anthropic-version", "2023-06-01")
	} else {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	request.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 8 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("provider could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("provider rejected the credentials")
	}
	return nil
}

func runAIAnalysis(ctx context.Context, config domain.AIProviderConfig, document json.RawMessage, report analysis.Report, allowPrivateURLs bool) (analysis.AIReview, error) {
	endpoint, err := providerBaseURL(config.Provider, config.BaseURL)
	if err != nil {
		return analysis.AIReview{}, err
	}
	if err := validateProviderEndpoint(endpoint, allowPrivateURLs); err != nil {
		return analysis.AIReview{}, err
	}
	userPrompt, err := analysis.BuildUserPrompt(document, report)
	if err != nil {
		return analysis.AIReview{}, err
	}
	if config.Provider == "anthropic" {
		return runAnthropicAnalysis(ctx, endpoint, config, userPrompt)
	}
	return runOpenAICompatibleAnalysis(ctx, endpoint, config, userPrompt)
}

func runOpenAICompatibleAnalysis(ctx context.Context, endpoint string, config domain.AIProviderConfig, userPrompt string) (analysis.AIReview, error) {
	payload := map[string]any{
		"model":       strings.TrimSpace(config.Model),
		"temperature": 0.1,
		"messages": []map[string]string{
			{"role": "system", "content": analysis.BuildSystemPrompt()},
			{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return analysis.AIReview{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return analysis.AIReview{}, errors.New("provider URL is invalid")
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(config.APIKey))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 24 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return analysis.AIReview{}, errors.New("provider could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return analysis.AIReview{}, errors.New("provider rejected the analysis request")
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&parsed); err != nil {
		return analysis.AIReview{}, errors.New("provider returned invalid JSON")
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return analysis.AIReview{}, errors.New("provider returned an empty review")
	}
	return parseAIReview(parsed.Choices[0].Message.Content, config), nil
}

func runAnthropicAnalysis(ctx context.Context, endpoint string, config domain.AIProviderConfig, userPrompt string) (analysis.AIReview, error) {
	payload := map[string]any{
		"model":       strings.TrimSpace(config.Model),
		"max_tokens":  2200,
		"temperature": 0.1,
		"system":      analysis.BuildSystemPrompt(),
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return analysis.AIReview{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/messages", bytes.NewReader(body))
	if err != nil {
		return analysis.AIReview{}, errors.New("provider URL is invalid")
	}
	request.Header.Set("x-api-key", strings.TrimSpace(config.APIKey))
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 24 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return analysis.AIReview{}, errors.New("provider could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return analysis.AIReview{}, errors.New("provider rejected the analysis request")
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&parsed); err != nil {
		return analysis.AIReview{}, errors.New("provider returned invalid JSON")
	}
	for _, content := range parsed.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			return parseAIReview(content.Text, config), nil
		}
	}
	return analysis.AIReview{}, errors.New("provider returned an empty review")
}

func parseAIReview(content string, config domain.AIProviderConfig) analysis.AIReview {
	var envelope analysis.AIReviewEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &envelope); err != nil {
		return analysis.AIReview{
			Status:        "failed",
			Provider:      config.Provider,
			Model:         config.Model,
			PromptVersion: analysis.PromptVersion,
			Error:         "AI response did not match the expected JSON shape",
		}
	}
	return analysis.AIReview{
		Status:          "completed",
		Provider:        config.Provider,
		Model:           config.Model,
		PromptVersion:   analysis.PromptVersion,
		ExecutiveReview: strings.TrimSpace(envelope.ExecutiveReview),
		Strengths:       cleanReviewStrings(envelope.Strengths),
		Risks:           cleanReviewPoints(envelope.Risks),
		Recommendations: cleanReviewPoints(envelope.Recommendations),
		OpenQuestions:   cleanReviewStrings(envelope.OpenQuestions),
	}
}

func cleanReviewStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func cleanReviewPoints(points []analysis.AIReviewPoint) []analysis.AIReviewPoint {
	cleaned := make([]analysis.AIReviewPoint, 0, len(points))
	for _, point := range points {
		point.Severity = normalizeAISeverity(point.Severity)
		point.Suite = normalizeAISuite(point.Suite)
		point.Title = strings.TrimSpace(point.Title)
		point.Detail = strings.TrimSpace(point.Detail)
		point.Impact = strings.TrimSpace(point.Impact)
		point.Recommendation = strings.TrimSpace(point.Recommendation)
		point.ComponentID = strings.TrimSpace(point.ComponentID)
		point.ConnectorID = strings.TrimSpace(point.ConnectorID)
		if point.Title == "" || point.Detail == "" {
			continue
		}
		cleaned = append(cleaned, point)
	}
	return cleaned
}

func normalizeAISeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(severity))
	default:
		return "medium"
	}
}

func normalizeAISuite(suite string) string {
	switch strings.ToLower(strings.TrimSpace(suite)) {
	case "requirements", "topology", "traffic", "consistency", "availability", "security", "data", "operability", "cost", "ai", "integrity":
		return strings.ToLower(strings.TrimSpace(suite))
	default:
		return "integrity"
	}
}

func providerBaseURL(provider string, baseURL string) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint != "" {
		return endpoint, nil
	}
	switch strings.TrimSpace(provider) {
	case "openai":
		return "https://api.openai.com/v1", nil
	case "anthropic":
		return "https://api.anthropic.com/v1", nil
	case "openrouter":
		return "https://openrouter.ai/api/v1", nil
	default:
		return "", errors.New("base URL is required for custom providers")
	}
}

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return
	}
	workspaces, page, err := s.services.Workspaces.ListPage(r.Context(), pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	filtered := make([]domain.Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		if s.canAccessWorkspace(r.Context(), user, workspace.ID, policy.WorkspaceRead) {
			filtered = append(filtered, workspace)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": filtered, "page": page})
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireRole(w, r, "architect")
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	workspace, err := s.services.Workspaces.Create(r.Context(), body.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, _ = s.services.ACL.GrantWorkspaceAccess(r.Context(), domain.WorkspaceAccess{
		WorkspaceID:     workspace.ID,
		UserID:          user.ID,
		CanRead:         true,
		CanCreateDesign: true,
		CanManage:       true,
	})
	writeJSON(w, http.StatusCreated, map[string]any{"workspace": workspace})
}

func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceManage); !ok {
		return
	}
	if err := s.services.Workspaces.Delete(r.Context(), workspaceID); err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceManage); !ok {
		return
	}
	access, err := s.services.ACL.ListWorkspaceAccess(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleGrantWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceManage); !ok {
		return
	}
	var body struct {
		UserID          string `json:"userId"`
		CanRead         bool   `json:"canRead"`
		CanCreateDesign bool   `json:"canCreateDesign"`
		CanManage       bool   `json:"canManage"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !body.CanRead && !body.CanCreateDesign && !body.CanManage {
		writeError(w, http.StatusBadRequest, "at least one workspace permission is required")
		return
	}
	access, err := s.services.ACL.GrantWorkspaceAccess(r.Context(), domain.WorkspaceAccess{
		WorkspaceID:     workspaceID,
		UserID:          body.UserID,
		CanRead:         body.CanRead,
		CanCreateDesign: body.CanCreateDesign,
		CanManage:       body.CanManage,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleRevokeWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceManage); !ok {
		return
	}
	if err := s.services.ACL.RevokeWorkspaceAccess(r.Context(), workspaceID, r.PathValue("userID")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAccessGroups(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	groups, err := s.services.ACL.ListGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (s *Server) handleCreateAccessGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		OktaGroupName string `json:"oktaGroupName"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	group, err := s.services.ACL.CreateGroup(r.Context(), domain.AccessGroup{
		Name:          body.Name,
		Description:   body.Description,
		OktaGroupName: body.OktaGroupName,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"group": group})
}

func (s *Server) handleUpdateAccessGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		OktaGroupName string `json:"oktaGroupName"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	group, err := s.services.ACL.UpdateGroup(r.Context(), r.PathValue("groupID"), domain.AccessGroup{
		Name:          body.Name,
		Description:   body.Description,
		OktaGroupName: body.OktaGroupName,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"group": group})
}

func (s *Server) handleDeleteAccessGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.services.ACL.DeleteGroup(r.Context(), r.PathValue("groupID")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAccessGroupMembers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	members, err := s.services.ACL.ListGroupMembers(r.Context(), r.PathValue("groupID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) handleReplaceAccessGroupMembers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var body struct {
		UserIDs []string `json:"userIds"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	members, err := s.services.ACL.ReplaceGroupMembers(r.Context(), r.PathValue("groupID"), body.UserIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) handleListWorkspaceGroupAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceManage); !ok {
		return
	}
	access, err := s.services.ACL.ListWorkspaceGroupAccess(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleGrantWorkspaceGroupAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceManage); !ok {
		return
	}
	var body struct {
		GroupID         string `json:"groupId"`
		CanRead         bool   `json:"canRead"`
		CanCreateDesign bool   `json:"canCreateDesign"`
		CanManage       bool   `json:"canManage"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !body.CanRead && !body.CanCreateDesign && !body.CanManage {
		writeError(w, http.StatusBadRequest, "at least one workspace permission is required")
		return
	}
	access, err := s.services.ACL.GrantWorkspaceGroupAccess(r.Context(), domain.WorkspaceGroupAccess{
		WorkspaceID:     workspaceID,
		GroupID:         body.GroupID,
		CanRead:         body.CanRead,
		CanCreateDesign: body.CanCreateDesign,
		CanManage:       body.CanManage,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleRevokeWorkspaceGroupAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceManage); !ok {
		return
	}
	if err := s.services.ACL.RevokeWorkspaceGroupAccess(r.Context(), workspaceID, r.PathValue("groupID")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListUserAccess(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	userID := strings.TrimSpace(r.PathValue("userID"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}
	workspaces, err := s.services.Workspaces.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type accessSummary struct {
		Scope         string    `json:"scope"`
		UserID        string    `json:"userId"`
		GroupID       string    `json:"groupId,omitempty"`
		GroupName     string    `json:"groupName,omitempty"`
		Source        string    `json:"source"`
		WorkspaceID   string    `json:"workspaceId"`
		WorkspaceName string    `json:"workspaceName"`
		DesignID      string    `json:"designId,omitempty"`
		DesignName    string    `json:"designName,omitempty"`
		Permissions   []string  `json:"permissions"`
		UpdatedAt     time.Time `json:"updatedAt"`
	}

	summaries := []accessSummary{}
	groupIDs, err := s.services.ACL.ListUserGroupIDs(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	userGroupSet := accessStringSet(groupIDs)
	groups, err := s.services.ACL.ListGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	groupNames := map[string]string{}
	for _, group := range groups {
		groupNames[group.ID] = group.Name
	}
	for _, workspace := range workspaces {
		workspaceAccess, err := s.services.ACL.ListWorkspaceAccess(r.Context(), workspace.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, access := range workspaceAccess {
			if access.UserID != userID {
				continue
			}
			permissions := workspacePermissionLabels(access)
			if len(permissions) == 0 {
				continue
			}
			summaries = append(summaries, accessSummary{
				Scope:         "workspace",
				UserID:        access.UserID,
				Source:        "user",
				WorkspaceID:   access.WorkspaceID,
				WorkspaceName: workspace.Name,
				Permissions:   permissions,
				UpdatedAt:     access.UpdatedAt,
			})
		}
		workspaceGroupAccess, err := s.services.ACL.ListWorkspaceGroupAccess(r.Context(), workspace.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, access := range workspaceGroupAccess {
			if _, ok := userGroupSet[access.GroupID]; !ok {
				continue
			}
			permissions := workspaceGroupPermissionLabels(access)
			if len(permissions) == 0 {
				continue
			}
			summaries = append(summaries, accessSummary{
				Scope:         "workspace",
				UserID:        userID,
				GroupID:       access.GroupID,
				GroupName:     groupNames[access.GroupID],
				Source:        "group",
				WorkspaceID:   access.WorkspaceID,
				WorkspaceName: workspace.Name,
				Permissions:   permissions,
				UpdatedAt:     access.UpdatedAt,
			})
		}

		designs, err := s.services.Designs.List(r.Context(), workspace.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, design := range designs {
			designAccess, err := s.services.ACL.ListDesignAccess(r.Context(), workspace.ID, design.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, access := range designAccess {
				if access.UserID != userID {
					continue
				}
				permissions := designPermissionLabels(access)
				if len(permissions) == 0 {
					continue
				}
				summaries = append(summaries, accessSummary{
					Scope:         "design",
					UserID:        access.UserID,
					Source:        "user",
					WorkspaceID:   access.WorkspaceID,
					WorkspaceName: workspace.Name,
					DesignID:      access.DesignID,
					DesignName:    design.Name,
					Permissions:   permissions,
					UpdatedAt:     access.UpdatedAt,
				})
			}
			designGroupAccess, err := s.services.ACL.ListDesignGroupAccess(r.Context(), workspace.ID, design.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, access := range designGroupAccess {
				if _, ok := userGroupSet[access.GroupID]; !ok {
					continue
				}
				permissions := designGroupPermissionLabels(access)
				if len(permissions) == 0 {
					continue
				}
				summaries = append(summaries, accessSummary{
					Scope:         "design",
					UserID:        userID,
					GroupID:       access.GroupID,
					GroupName:     groupNames[access.GroupID],
					Source:        "group",
					WorkspaceID:   access.WorkspaceID,
					WorkspaceName: workspace.Name,
					DesignID:      access.DesignID,
					DesignName:    design.Name,
					Permissions:   permissions,
					UpdatedAt:     access.UpdatedAt,
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": summaries})
}

func (s *Server) handleListDesigns(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	user, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceRead)
	if !ok {
		return
	}
	designs, page, err := s.services.Designs.ListPage(r.Context(), workspaceID, pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filtered := make([]domain.Design, 0, len(designs))
	for _, design := range designs {
		if s.canAccessDesign(r.Context(), user, design, policy.DesignRead) {
			filtered = append(filtered, design)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"designs": filtered, "page": page})
}

func (s *Server) handleCreateDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	user, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceCreateDesign)
	if !ok {
		return
	}
	var body struct {
		Name     string          `json:"name"`
		Document json.RawMessage `json:"document"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	design, err := s.services.Designs.Create(r.Context(), workspaceID, body.Name, body.Document, user.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, _ = s.services.ACL.GrantDesignAccess(r.Context(), domain.DesignAccess{
		WorkspaceID: design.WorkspaceID,
		DesignID:    design.ID,
		UserID:      user.ID,
		CanRead:     true,
		CanEdit:     true,
		CanComment:  true,
		CanReview:   true,
		CanManage:   true,
	})
	writeJSON(w, http.StatusCreated, map[string]any{"design": design})
}

func (s *Server) handleGetDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	_, design, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignRead)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"design": design})
}

func (s *Server) handleDeleteDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	if err := s.services.Designs.Delete(r.Context(), workspaceID, designID); err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateDesignMetadata(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	var body struct {
		Name   string `json:"name"`
		Access string `json:"access"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	design, err := s.services.Designs.UpdateMetadata(r.Context(), workspaceID, designID, body.Name, body.Access)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"design": design})
}

func (s *Server) handleListDesignAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	access, err := s.services.ACL.ListDesignAccess(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleGrantDesignAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	var body struct {
		UserID     string `json:"userId"`
		CanRead    bool   `json:"canRead"`
		CanEdit    bool   `json:"canEdit"`
		CanComment bool   `json:"canComment"`
		CanReview  bool   `json:"canReview"`
		CanManage  bool   `json:"canManage"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !body.CanRead && !body.CanEdit && !body.CanComment && !body.CanReview && !body.CanManage {
		writeError(w, http.StatusBadRequest, "at least one design permission is required")
		return
	}
	access, err := s.services.ACL.GrantDesignAccess(r.Context(), domain.DesignAccess{
		WorkspaceID: workspaceID,
		DesignID:    designID,
		UserID:      body.UserID,
		CanRead:     body.CanRead,
		CanEdit:     body.CanEdit,
		CanComment:  body.CanComment,
		CanReview:   body.CanReview,
		CanManage:   body.CanManage,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleRevokeDesignAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	if err := s.services.ACL.RevokeDesignAccess(r.Context(), workspaceID, designID, r.PathValue("userID")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDesignGroupAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	access, err := s.services.ACL.ListDesignGroupAccess(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleGrantDesignGroupAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	var body struct {
		GroupID    string `json:"groupId"`
		CanRead    bool   `json:"canRead"`
		CanEdit    bool   `json:"canEdit"`
		CanComment bool   `json:"canComment"`
		CanReview  bool   `json:"canReview"`
		CanManage  bool   `json:"canManage"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !body.CanRead && !body.CanEdit && !body.CanComment && !body.CanReview && !body.CanManage {
		writeError(w, http.StatusBadRequest, "at least one design permission is required")
		return
	}
	access, err := s.services.ACL.GrantDesignGroupAccess(r.Context(), domain.DesignGroupAccess{
		WorkspaceID: workspaceID,
		DesignID:    designID,
		GroupID:     body.GroupID,
		CanRead:     body.CanRead,
		CanEdit:     body.CanEdit,
		CanComment:  body.CanComment,
		CanReview:   body.CanReview,
		CanManage:   body.CanManage,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleRevokeDesignGroupAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	if err := s.services.ACL.RevokeDesignGroupAccess(r.Context(), workspaceID, designID, r.PathValue("groupID")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSaveDesignDocument(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	_, existing, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignEdit)
	if !ok {
		return
	}
	var body struct {
		Document       json.RawMessage `json:"document"`
		CanvasSnapshot json.RawMessage `json:"canvasSnapshot"`
		VersionRemarks string          `json:"versionRemarks"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.Document) == 0 || !json.Valid(body.Document) {
		writeError(w, http.StatusBadRequest, "design document must be valid JSON")
		return
	}
	design := domain.Design{
		ID:             existing.ID,
		WorkspaceID:    existing.WorkspaceID,
		Name:           existing.Name,
		Access:         existing.Access,
		Title:          existing.Title,
		Document:       body.Document,
		CanvasSnapshot: existing.CanvasSnapshot,
		VersionRemarks: strings.TrimSpace(body.VersionRemarks),
		CreatedBy:      existing.CreatedBy,
		CreatedAt:      existing.CreatedAt,
	}
	if domain.ValidCanvasSnapshot(body.CanvasSnapshot) {
		design.CanvasSnapshot = body.CanvasSnapshot
	}
	updated, err := s.services.Designs.Upsert(r.Context(), design)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"design": updated})
}

func (s *Server) handleListDesignVersions(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignRead); !ok {
		return
	}
	versions, page, err := s.services.Versions.ListPage(r.Context(), workspaceID, designID, pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions, "page": page})
}

func (s *Server) handleCreateDesignVersion(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	user, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignEdit)
	if !ok {
		return
	}
	var body struct {
		Remarks string `json:"remarks"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	version, err := s.services.Versions.Create(r.Context(), workspaceID, designID, user.ID, strings.TrimSpace(body.Remarks))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"version": version})
}

func (s *Server) handleUpdateDesignVersionStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	version, err := s.services.Versions.UpdateStatus(r.Context(), workspaceID, designID, r.PathValue("versionID"), body.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": version})
}

func (s *Server) handleDeleteDesignVersion(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignManage); !ok {
		return
	}
	if err := s.services.Versions.Delete(r.Context(), workspaceID, designID, r.PathValue("versionID")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAnalyzeDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	_, design, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignReview)
	if !ok {
		return
	}

	report, err := s.services.Analysis.AnalyzeDocument(design.Document)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if aiConfig, err := s.services.Identity.GetAIProviderConfigWithSecret(r.Context()); err == nil && aiConfig.Enabled && strings.TrimSpace(aiConfig.APIKey) != "" {
		review, err := runAIAnalysis(r.Context(), aiConfig, design.Document, report, s.cfg.AllowPrivateAIProviderURLs)
		if err != nil {
			review = analysis.AIReview{
				Status:        "failed",
				Provider:      aiConfig.Provider,
				Model:         aiConfig.Model,
				PromptVersion: analysis.PromptVersion,
				Error:         "AI synthesis failed: " + err.Error(),
			}
		}
		report = analysis.AttachAIReview(report, review)
	}
	writeJSON(w, http.StatusOK, map[string]any{"analysis": report})
}

func (s *Server) handleListDesignDocs(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignRead); !ok {
		return
	}
	docs, err := s.services.Docs.List(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"docs": docs})
}

func (s *Server) handleGetDesignDoc(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	docID := r.PathValue("docID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignRead); !ok {
		return
	}
	doc, err := s.services.Docs.Get(r.Context(), workspaceID, designID, docID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc})
}

func (s *Server) handleCreateDesignDoc(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignEdit); !ok {
		return
	}
	var body struct {
		Title  string `json:"title"`
		Body   string `json:"body"`
		Format string `json:"format"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	doc, err := s.services.Docs.Create(r.Context(), workspaceID, designID, body.Title, body.Body, body.Format)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"doc": doc})
}

func (s *Server) handleUpdateDesignDoc(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	docID := r.PathValue("docID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignEdit); !ok {
		return
	}
	existing, err := s.services.Docs.Get(r.Context(), workspaceID, designID, docID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var body struct {
		Title  *string `json:"title"`
		Body   *string `json:"body"`
		Format *string `json:"format"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	title := existing.Title
	docBody := existing.Body
	format := existing.Format
	if body.Title != nil {
		title = *body.Title
	}
	if body.Body != nil {
		docBody = *body.Body
	}
	if body.Format != nil {
		format = *body.Format
	}
	doc, err := s.services.Docs.Update(r.Context(), workspaceID, designID, docID, title, docBody, format)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc})
}

func (s *Server) handleDeleteDesignDoc(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	docID := r.PathValue("docID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignEdit); !ok {
		return
	}
	if err := s.services.Docs.Delete(r.Context(), workspaceID, designID, docID); err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDesignComments(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignRead); !ok {
		return
	}
	comments, page, err := s.services.Reviews.ListCommentsPage(r.Context(), workspaceID, designID, pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": comments, "page": page})
}

func (s *Server) handleCreateDesignComment(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	user, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignComment)
	if !ok {
		return
	}
	var body struct {
		Body        string `json:"body"`
		ComponentID string `json:"componentId"`
		ConnectorID string `json:"connectorId"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	comment, err := s.services.Reviews.CreateComment(r.Context(), workspaceID, designID, user.ID, body.Body, body.ComponentID, body.ConnectorID)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"comment": comment})
}

func (s *Server) handleListDesignReviews(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignRead); !ok {
		return
	}
	reviews, page, err := s.services.Reviews.ListReviewsPage(r.Context(), workspaceID, designID, pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews, "page": page})
}

func (s *Server) handleCreateDesignReview(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	user, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignReview)
	if !ok {
		return
	}
	var body struct {
		VersionID   string   `json:"versionId"`
		ReviewerID  string   `json:"reviewerId"`
		ReviewerIDs []string `json:"reviewerIds"`
		Message     string   `json:"message"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	reviewerIDs := body.ReviewerIDs
	if len(reviewerIDs) == 0 && strings.TrimSpace(body.ReviewerID) != "" {
		reviewerIDs = []string{body.ReviewerID}
	}
	reviews, err := s.services.Reviews.CreateReviews(r.Context(), workspaceID, designID, body.VersionID, user.ID, reviewerIDs, body.Message)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reviews": reviews})
}

func (s *Server) handleUpdateDesignReview(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	reviewID := r.PathValue("reviewID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignReview); !ok {
		return
	}
	var body struct {
		Status  string `json:"status"`
		Summary string `json:"summary"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	review, err := s.services.Reviews.UpdateReview(r.Context(), workspaceID, designID, reviewID, body.Status, body.Summary)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": review})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCORSHeaders(w, r, s.cfg.AllowedOrigins)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && !isAllowedOrigin(origin, s.cfg.AllowedOrigins) {
			writeError(w, http.StatusForbidden, "origin is not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			"script-src 'self'",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data: blob:",
			"font-src 'self' data:",
			"connect-src 'self' ws: wss:",
			"object-src 'none'",
			"base-uri 'none'",
			"frame-ancestors 'none'",
			"form-action 'self'",
		}, "; "))
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.log.Error("panic recovered", "method", r.Method, "path", r.URL.Path, "error", recovered, "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseLogRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseLogRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *responseLogRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseLogRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	count, err := r.ResponseWriter.Write(body)
	r.bytes += count
	return count, err
}

func requestLogger(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &responseLogRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"bytes", recorder.bytes,
			"remote", remoteAddress(r.RemoteAddr),
		)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) error {
	if maxBytes <= 0 {
		maxBytes = 4 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request body must contain a single JSON document")
	}
	return nil
}

func pageOptionsFromRequest(r *http.Request) store.PageOptions {
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	return store.NormalizePageOptions(store.PageOptions{
		Query:  query.Get("query"),
		Cursor: query.Get("cursor"),
		Limit:  limit,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError {
		message = "internal server error"
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func remoteAddress(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}

func statusForDeleteError(err error) int {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not found") {
		return http.StatusNotFound
	}
	if strings.Contains(message, "not empty") {
		return http.StatusConflict
	}
	if strings.Contains(message, "cannot be deleted") || strings.Contains(message, "required") || strings.Contains(message, "at least") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func statusForCatalogError(err error) int {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "already exists") {
		return http.StatusConflict
	}
	if strings.Contains(message, "linked to") {
		return http.StatusConflict
	}
	return statusForDeleteError(err)
}

func writeCORSHeaders(w http.ResponseWriter, r *http.Request, allowedOrigins []string) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if isAllowedOrigin(origin, allowedOrigins) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Access-Control-Max-Age", "600")
}

func (s *Server) currentUser(r *http.Request) (domain.User, error) {
	return s.userFromToken(r.Context(), sessionToken(r))
}

func (s *Server) userFromToken(ctx context.Context, token string) (domain.User, error) {
	return s.services.Identity.UserBySessionToken(ctx, token)
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

const sessionCookieName = "stratum_session"

func (s *Server) sessionExpiresAt() time.Time {
	ttl := s.cfg.SessionTTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return time.Now().UTC().Add(ttl)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	expiresAt := s.sessionExpiresAt()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) passwordResetLink(r *http.Request, token string) string {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		if parsed, err := url.Parse(origin); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			link := *parsed
			link.Path = "/reset-password"
			link.RawQuery = ""
			query := link.Query()
			query.Set("token", token)
			link.RawQuery = query.Encode()
			return link.String()
		}
	}
	scheme := "http"
	if requestIsSecure(r) {
		scheme = "https"
	}
	host := r.Host
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.Split(forwardedProto, ",")[0]
	}
	link := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   "/reset-password",
	}
	query := link.Query()
	query.Set("token", token)
	link.RawQuery = query.Encode()
	return link.String()
}

func requestIsSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	_, ok := s.requireRole(w, r, "admin")
	return ok
}

func (s *Server) requireRole(w http.ResponseWriter, r *http.Request, minimumRole string) (domain.User, bool) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return domain.User{}, false
	}
	if !policy.RoleAllows(user.Role, minimumRole) {
		writeError(w, http.StatusForbidden, minimumRole+" role is required")
		return domain.User{}, false
	}
	return user, true
}

func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) bool {
	if _, err := s.currentUser(r); err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return false
	}
	return true
}

func (s *Server) requireWorkspaceAccess(w http.ResponseWriter, r *http.Request, workspaceID string, permission string) (domain.User, bool) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return domain.User{}, false
	}
	if !s.canAccessWorkspace(r.Context(), user, workspaceID, permission) {
		writeError(w, http.StatusForbidden, "workspace "+permission+" access is required")
		return domain.User{}, false
	}
	return user, true
}

func (s *Server) requireDesignAccess(w http.ResponseWriter, r *http.Request, workspaceID string, designID string, permission string) (domain.User, domain.Design, bool) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return domain.User{}, domain.Design{}, false
	}
	design, err := s.services.Designs.Get(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return domain.User{}, domain.Design{}, false
	}
	if !s.canAccessDesign(r.Context(), user, design, permission) {
		writeError(w, http.StatusForbidden, "design "+permission+" access is required")
		return domain.User{}, domain.Design{}, false
	}
	return user, design, true
}

func (s *Server) canAccessWorkspace(ctx context.Context, user domain.User, workspaceID string, permission string) bool {
	return s.authz.CanAccessWorkspace(ctx, user, workspaceID, permission)
}

func (s *Server) canAccessDesign(ctx context.Context, user domain.User, design domain.Design, permission string) bool {
	return s.authz.CanAccessDesign(ctx, user, design, permission)
}

func workspacePermissionLabels(access domain.WorkspaceAccess) []string {
	permissions := []string{}
	if access.CanRead {
		permissions = append(permissions, "Read")
	}
	if access.CanCreateDesign {
		permissions = append(permissions, "Create designs")
	}
	if access.CanManage {
		permissions = append(permissions, "Manage")
	}
	return permissions
}

func workspaceGroupPermissionLabels(access domain.WorkspaceGroupAccess) []string {
	permissions := []string{}
	if access.CanRead {
		permissions = append(permissions, "Read")
	}
	if access.CanCreateDesign {
		permissions = append(permissions, "Create designs")
	}
	if access.CanManage {
		permissions = append(permissions, "Manage")
	}
	return permissions
}

func designPermissionLabels(access domain.DesignAccess) []string {
	permissions := []string{}
	if access.CanRead {
		permissions = append(permissions, "Read")
	}
	if access.CanEdit {
		permissions = append(permissions, "Edit")
	}
	if access.CanComment {
		permissions = append(permissions, "Comment")
	}
	if access.CanReview {
		permissions = append(permissions, "Review")
	}
	if access.CanManage {
		permissions = append(permissions, "Manage")
	}
	return permissions
}

func designGroupPermissionLabels(access domain.DesignGroupAccess) []string {
	permissions := []string{}
	if access.CanRead {
		permissions = append(permissions, "Read")
	}
	if access.CanEdit {
		permissions = append(permissions, "Edit")
	}
	if access.CanComment {
		permissions = append(permissions, "Comment")
	}
	if access.CanReview {
		permissions = append(permissions, "Review")
	}
	if access.CanManage {
		permissions = append(permissions, "Manage")
	}
	return permissions
}

func accessStringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	for _, allowed := range allowedOrigins {
		if originPatternMatches(allowed, parsed) {
			return true
		}
	}
	return false
}

func originPatternMatches(pattern string, origin *url.URL) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.HasSuffix(pattern, ":*") {
		withoutWildcard := strings.TrimSuffix(pattern, ":*")
		parsed, err := url.Parse(withoutWildcard)
		if err != nil {
			return false
		}
		return parsed.Scheme == origin.Scheme && parsed.Hostname() == origin.Hostname()
	}
	parsed, err := url.Parse(pattern)
	if err != nil {
		return false
	}
	if parsed.Scheme != origin.Scheme {
		return false
	}
	if parsed.Hostname() != origin.Hostname() {
		return false
	}
	patternPort := parsed.Port()
	if patternPort == "*" {
		return true
	}
	return patternPort == origin.Port()
}

func validateProviderEndpoint(endpoint string, allowPrivateURLs bool) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" {
		return errors.New("provider URL is invalid")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("provider URL must use http or https")
	}
	if parsed.Scheme != "https" && !allowPrivateURLs {
		return errors.New("custom provider URL must use https")
	}
	if allowPrivateURLs {
		return nil
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil && isPrivateAddress(ip) {
		return errors.New("custom provider URL cannot target a private network address")
	}
	lookupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return fmt.Errorf("provider host could not be resolved")
	}
	for _, address := range addresses {
		if isPrivateAddress(address.IP) {
			return errors.New("custom provider URL cannot resolve to a private network address")
		}
	}
	return nil
}

func isPrivateAddress(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

func Shutdown(ctx context.Context, server *http.Server) error {
	return server.Shutdown(ctx)
}
