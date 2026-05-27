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
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/realtime"
)

type Server struct {
	cfg config.Config
	hub *realtime.Hub
	log *slog.Logger
	mux *http.ServeMux
}

func NewServer(cfg config.Config, hub *realtime.Hub, log *slog.Logger) *Server {
	server := &Server{
		cfg: cfg,
		hub: hub,
		log: log,
		mux: http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.recoverPanic(s.cors(requestLogger(s.log, s.mux))))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /ws", s.handleWorkspaceSocket)
	s.mux.HandleFunc("GET /api/setup/status", s.handleSetupStatus)
	s.mux.HandleFunc("POST /api/setup/first-admin", s.handleCreateFirstAdmin)
	s.mux.HandleFunc("POST /api/setup/admin-password", s.handleSetInitialAdminPassword)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/profile", s.handleProfile)
	s.mux.HandleFunc("GET /api/users", s.handleListUsers)
	s.mux.HandleFunc("POST /api/admin/users", s.handleCreateUser)
	s.mux.HandleFunc("PATCH /api/admin/users/{userID}", s.handleUpdateUser)
	s.mux.HandleFunc("DELETE /api/admin/users/{userID}", s.handleDeleteUser)
	s.mux.HandleFunc("GET /api/admin/sign-in", s.handleGetSignInConfig)
	s.mux.HandleFunc("PATCH /api/admin/sign-in", s.handleUpdateSignInConfig)
	s.mux.HandleFunc("GET /api/admin/ai-provider", s.handleGetAIProviderConfig)
	s.mux.HandleFunc("PATCH /api/admin/ai-provider", s.handleUpdateAIProviderConfig)
	s.mux.HandleFunc("GET /api/admin/mcp", s.handleGetMCPConfig)
	s.mux.HandleFunc("PATCH /api/admin/mcp", s.handleUpdateMCPConfig)
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
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs", s.handleListDesigns)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs", s.handleCreateDesign)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}", s.handleGetDesign)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/designs/{designID}", s.handleDeleteDesign)
	s.mux.HandleFunc("PATCH /api/workspaces/{workspaceID}/designs/{designID}", s.handleUpdateDesignMetadata)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/access", s.handleListDesignAccess)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/access", s.handleGrantDesignAccess)
	s.mux.HandleFunc("DELETE /api/workspaces/{workspaceID}/designs/{designID}/access/{userID}", s.handleRevokeDesignAccess)
	s.mux.HandleFunc("PUT /api/workspaces/{workspaceID}/designs/{designID}/document", s.handleSaveDesignDocument)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/analysis", s.handleAnalyzeDesign)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/versions", s.handleListDesignVersions)
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
	if _, err := s.hub.Repository().GetWorkspace(r.Context(), workspaceID); err != nil {
		http.Error(w, "workspace not found", http.StatusNotFound)
		return
	}
	if !s.canAccessWorkspace(r.Context(), user, workspaceID, workspacePermissionRead) {
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

	client := realtime.NewClient(conn, s.hub, s.log, workspaceID, user.ID, roleAllows(user.Role, "admin"))
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
	users, err := s.hub.Repository().ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	passwordSetupRequired, err := s.hub.Repository().PasswordSetupRequired(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requiresSetup": len(users) == 0, "requiresPasswordSetup": passwordSetupRequired})
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
	user, err := s.hub.Repository().CreateFirstAdmin(r.Context(), body.DisplayName, body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, err := s.hub.Repository().CreateSession(r.Context(), user.ID, s.sessionExpiresAt())
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
	user, err := s.hub.Repository().SetInitialAdminPassword(r.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, err := s.hub.Repository().CreateSession(r.Context(), user.ID, s.sessionExpiresAt())
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
	user, err := s.hub.Repository().AuthenticateUser(r.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	token, err := s.hub.Repository().CreateSession(r.Context(), user.ID, s.sessionExpiresAt())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r)
	if token != "" {
		_ = s.hub.Repository().DeleteSession(r.Context(), token)
	}
	s.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		users, listErr := s.hub.Repository().ListUsers(r.Context())
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
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if _, err := s.currentUser(r); err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return
	}
	users, err := s.hub.Repository().ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
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
	user, err := s.hub.Repository().CreateUser(r.Context(), body.DisplayName, body.Email, body.Role, body.Password)
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
	user, err := s.hub.Repository().UpdateUser(r.Context(), r.PathValue("userID"), body.DisplayName, body.Email, body.Role, body.Status, body.Password)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.hub.Repository().DeleteUser(r.Context(), r.PathValue("userID")); err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetSignInConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	config, err := s.hub.Repository().GetSignInConfig(r.Context())
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
	config, err := s.hub.Repository().UpdateSignInConfig(r.Context(), domain.SignInConfig{
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
	config, err := s.hub.Repository().GetAIProviderConfig(r.Context())
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
		current, err := s.hub.Repository().GetAIProviderConfigWithSecret(r.Context())
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
	config, err := s.hub.Repository().UpdateAIProviderConfig(r.Context(), domain.AIProviderConfig{
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
	config, err := s.hub.Repository().GetMCPConfig(r.Context())
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
	config, err := s.hub.Repository().UpdateMCPConfig(r.Context(), domain.MCPConfig{
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
	assets, err := s.hub.Repository().ListCatalogAssets(r.Context(), r.URL.Query().Get("query"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets})
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
	asset, err := s.hub.Repository().CreateCatalogAsset(r.Context(), domain.CatalogAsset{
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
	asset, err := s.hub.Repository().UpdateCatalogAsset(r.Context(), r.PathValue("assetID"), domain.CatalogAsset{
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
	if err := s.hub.Repository().DeleteCatalogAsset(r.Context(), r.PathValue("assetID")); err != nil {
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
	notifications, err := s.hub.Repository().ListNotifications(r.Context(), user.ID)
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
	if err := s.hub.Repository().MarkNotificationRead(r.Context(), user.ID, r.PathValue("notificationID")); err != nil {
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
	workspaces, err := s.hub.Repository().ListWorkspaces(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	filtered := make([]domain.Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		if s.canAccessWorkspace(r.Context(), user, workspace.ID, workspacePermissionRead) {
			filtered = append(filtered, workspace)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": filtered})
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
	workspace, err := s.hub.Repository().CreateWorkspace(r.Context(), body.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, _ = s.hub.Repository().GrantWorkspaceAccess(r.Context(), domain.WorkspaceAccess{
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
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, workspacePermissionManage); !ok {
		return
	}
	if err := s.hub.Repository().DeleteWorkspace(r.Context(), workspaceID); err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, workspacePermissionManage); !ok {
		return
	}
	access, err := s.hub.Repository().ListWorkspaceAccess(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleGrantWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, workspacePermissionManage); !ok {
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
	access, err := s.hub.Repository().GrantWorkspaceAccess(r.Context(), domain.WorkspaceAccess{
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
	if _, ok := s.requireWorkspaceAccess(w, r, workspaceID, workspacePermissionManage); !ok {
		return
	}
	if err := s.hub.Repository().RevokeWorkspaceAccess(r.Context(), workspaceID, r.PathValue("userID")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDesigns(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	user, ok := s.requireWorkspaceAccess(w, r, workspaceID, workspacePermissionRead)
	if !ok {
		return
	}
	designs, err := s.hub.Repository().ListDesigns(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filtered := make([]domain.Design, 0, len(designs))
	for _, design := range designs {
		if s.canAccessDesign(r.Context(), user, design, designPermissionRead) {
			filtered = append(filtered, design)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"designs": filtered})
}

func (s *Server) handleCreateDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	user, ok := s.requireWorkspaceAccess(w, r, workspaceID, workspacePermissionCreateDesign)
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
	design, err := s.hub.Repository().CreateDesign(r.Context(), workspaceID, body.Name, body.Document, user.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, _ = s.hub.Repository().GrantDesignAccess(r.Context(), domain.DesignAccess{
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
	_, design, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionRead)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"design": design})
}

func (s *Server) handleDeleteDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionManage); !ok {
		return
	}
	if err := s.hub.Repository().DeleteDesign(r.Context(), workspaceID, designID); err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateDesignMetadata(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionManage); !ok {
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
	design, err := s.hub.Repository().UpdateDesignMetadata(r.Context(), workspaceID, designID, body.Name, body.Access)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"design": design})
}

func (s *Server) handleListDesignAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionManage); !ok {
		return
	}
	access, err := s.hub.Repository().ListDesignAccess(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": access})
}

func (s *Server) handleGrantDesignAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionManage); !ok {
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
	access, err := s.hub.Repository().GrantDesignAccess(r.Context(), domain.DesignAccess{
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
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionManage); !ok {
		return
	}
	if err := s.hub.Repository().RevokeDesignAccess(r.Context(), workspaceID, designID, r.PathValue("userID")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSaveDesignDocument(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	_, existing, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionEdit)
	if !ok {
		return
	}
	var body struct {
		Document       json.RawMessage `json:"document"`
		CanvasSnapshot json.RawMessage `json:"canvasSnapshot"`
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
		CreatedBy:      existing.CreatedBy,
		CreatedAt:      existing.CreatedAt,
	}
	if domain.ValidCanvasSnapshot(body.CanvasSnapshot) {
		design.CanvasSnapshot = body.CanvasSnapshot
	}
	updated, err := s.hub.Repository().UpsertDesign(r.Context(), design)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"design": updated})
}

func (s *Server) handleListDesignVersions(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionRead); !ok {
		return
	}
	versions, err := s.hub.Repository().ListDesignVersions(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (s *Server) handleAnalyzeDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	_, design, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionReview)
	if !ok {
		return
	}

	report, err := analysis.New().Analyze(design.Document)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if aiConfig, err := s.hub.Repository().GetAIProviderConfigWithSecret(r.Context()); err == nil && aiConfig.Enabled && strings.TrimSpace(aiConfig.APIKey) != "" {
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
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionRead); !ok {
		return
	}
	docs, err := s.hub.Repository().ListDesignDocs(r.Context(), workspaceID, designID)
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
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionRead); !ok {
		return
	}
	doc, err := s.hub.Repository().GetDesignDoc(r.Context(), workspaceID, designID, docID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc})
}

func (s *Server) handleCreateDesignDoc(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionEdit); !ok {
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
	doc, err := s.hub.Repository().CreateDesignDoc(r.Context(), workspaceID, designID, body.Title, body.Body, body.Format)
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
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionEdit); !ok {
		return
	}
	existing, err := s.hub.Repository().GetDesignDoc(r.Context(), workspaceID, designID, docID)
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
	doc, err := s.hub.Repository().UpdateDesignDoc(r.Context(), workspaceID, designID, docID, title, docBody, format)
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
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionEdit); !ok {
		return
	}
	if err := s.hub.Repository().DeleteDesignDoc(r.Context(), workspaceID, designID, docID); err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDesignComments(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionRead); !ok {
		return
	}
	comments, err := s.hub.Repository().ListDesignComments(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": comments})
}

func (s *Server) handleCreateDesignComment(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	user, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionComment)
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
	comment, err := s.hub.Repository().CreateDesignComment(r.Context(), workspaceID, designID, user.ID, body.Body, body.ComponentID, body.ConnectorID)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"comment": comment})
}

func (s *Server) handleListDesignReviews(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionRead); !ok {
		return
	}
	reviews, err := s.hub.Repository().ListDesignReviewRequests(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

func (s *Server) handleCreateDesignReview(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	user, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionReview)
	if !ok {
		return
	}
	var body struct {
		ReviewerID string `json:"reviewerId"`
		Message    string `json:"message"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	review, err := s.hub.Repository().CreateDesignReviewRequest(r.Context(), workspaceID, designID, user.ID, body.ReviewerID, body.Message)
	if err != nil {
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"review": review})
}

func (s *Server) handleUpdateDesignReview(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	reviewID := r.PathValue("reviewID")
	if _, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, designPermissionReview); !ok {
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
	review, err := s.hub.Repository().UpdateDesignReviewRequest(r.Context(), workspaceID, designID, reviewID, body.Status, body.Summary)
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

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.log.Error("panic recovered", "method", r.Method, "path", r.URL.Path, "error", recovered)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestLogger(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) error {
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

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
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
	if token == "" {
		return domain.User{}, errors.New("session is required")
	}
	return s.hub.Repository().GetUserBySessionToken(ctx, token)
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
	if !roleAllows(user.Role, minimumRole) {
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

const (
	workspacePermissionRead         = "read"
	workspacePermissionCreateDesign = "create_design"
	workspacePermissionManage       = "manage"
	designPermissionRead            = "read"
	designPermissionEdit            = "edit"
	designPermissionComment         = "comment"
	designPermissionReview          = "review"
	designPermissionManage          = "manage"
)

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
	design, err := s.hub.Repository().GetDesign(r.Context(), workspaceID, designID)
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
	if roleAllows(user.Role, "admin") {
		return true
	}
	entries, err := s.hub.Repository().ListWorkspaceAccess(ctx, workspaceID)
	if err != nil {
		return false
	}
	if len(entries) == 0 {
		return roleAllows(user.Role, "architect")
	}
	for _, entry := range entries {
		if entry.UserID == user.ID && workspaceAccessAllows(entry, permission) {
			return true
		}
	}
	return false
}

func (s *Server) canAccessDesign(ctx context.Context, user domain.User, design domain.Design, permission string) bool {
	if roleAllows(user.Role, "admin") || design.CreatedBy == user.ID {
		return true
	}
	if permission == designPermissionRead && strings.EqualFold(design.Access, "public") {
		return true
	}
	if s.canAccessWorkspace(ctx, user, design.WorkspaceID, workspacePermissionManage) {
		return true
	}
	if permission == designPermissionRead && strings.EqualFold(design.Access, "workspace") && s.canAccessWorkspace(ctx, user, design.WorkspaceID, workspacePermissionRead) {
		return true
	}
	entries, err := s.hub.Repository().ListDesignAccess(ctx, design.WorkspaceID, design.ID)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.UserID == user.ID && designAccessAllows(entry, permission) {
			return true
		}
	}
	return false
}

func workspaceAccessAllows(access domain.WorkspaceAccess, permission string) bool {
	if access.CanManage {
		return true
	}
	switch permission {
	case workspacePermissionRead:
		return access.CanRead || access.CanCreateDesign
	case workspacePermissionCreateDesign:
		return access.CanCreateDesign
	case workspacePermissionManage:
		return access.CanManage
	default:
		return false
	}
}

func designAccessAllows(access domain.DesignAccess, permission string) bool {
	if access.CanManage {
		return true
	}
	switch permission {
	case designPermissionRead:
		return access.CanRead || access.CanEdit || access.CanComment || access.CanReview
	case designPermissionEdit:
		return access.CanEdit
	case designPermissionComment:
		return access.CanComment || access.CanEdit || access.CanReview
	case designPermissionReview:
		return access.CanReview || access.CanEdit
	case designPermissionManage:
		return access.CanManage
	default:
		return false
	}
}

func roleAllows(actualRole string, minimumRole string) bool {
	return roleRank(actualRole) >= roleRank(minimumRole)
}

func roleRank(role string) int {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "admin":
		return 4
	case "architect":
		return 3
	case "reviewer":
		return 2
	case "member":
		return 1
	default:
		return 0
	}
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
