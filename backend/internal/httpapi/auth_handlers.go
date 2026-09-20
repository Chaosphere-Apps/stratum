package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/system-design-evaluator/backend/internal/authn"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/policy"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"net/http"
	"strings"
	"time"
)

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
	designID := strings.TrimSpace(r.URL.Query().Get("designId"))
	if designID != "" {
		design, designErr := s.services.Designs.Get(r.Context(), workspaceID, designID)
		if designErr != nil {
			http.Error(w, "design not found", http.StatusNotFound)
			return
		}
		if !s.canAccessDesign(r.Context(), user, design, policy.DesignRead) {
			http.Error(w, "design read access is required", http.StatusForbidden)
			return
		}
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
	accessibleDesigns := snapshot.Designs[:0]
	for _, design := range snapshot.Designs {
		if s.canAccessDesign(ctx, user, design, policy.DesignRead) {
			accessibleDesigns = append(accessibleDesigns, design)
		}
	}
	snapshot.Designs = accessibleDesigns

	client := realtime.NewClient(conn, s.hub, s.log, workspaceID, user.ID, user.Role, policy.RoleAllows(user.Role, "admin")).WithPresence(user.DisplayName, designID)
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
	if !s.allowAuthAttempt(w, r, "first-admin", "", s.cfg.PasswordResetAttempts) {
		return
	}
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
	if !s.allowAuthAttempt(w, r, "admin-password", body.Email, s.cfg.PasswordResetAttempts) {
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
	if !s.allowAuthAttempt(w, r, "login", body.Email, s.cfg.LoginAttemptsPerWindow) {
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

func (s *Server) handleAuthConfig(w http.ResponseWriter, r *http.Request) {
	config, err := s.services.Identity.GetSignInConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sign-in configuration is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"localPasswordEnabled": config.LocalPasswordEnabled,
		"ssoEnabled":           config.SSOEnabled,
		"provider":             config.Provider,
		"passwordPolicy":       authn.CurrentPasswordPolicy(),
	})
}

func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	if !s.allowAuthAttempt(w, r, "oidc-start", "", s.cfg.LoginAttemptsPerWindow) {
		return
	}
	config, err := s.services.Identity.GetSignInConfigWithSecret(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "SSO configuration is unavailable")
		return
	}
	authorization, err := s.oidc.Begin(r.Context(), config)
	if err != nil {
		s.log.Warn("OIDC authorization could not start", "error", err)
		writeError(w, http.StatusServiceUnavailable, "SSO is not ready")
		return
	}
	if err := s.services.Identity.CreateOIDCFlow(r.Context(), domain.OIDCFlow{
		StateHash:    authorization.StateHash,
		Nonce:        authorization.Nonce,
		CodeVerifier: authorization.CodeVerifier,
		ExpiresAt:    authorization.ExpiresAt,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "could not initialize SSO")
		return
	}
	s.setOIDCStateCookie(w, r, authorization.State)
	http.Redirect(w, r, authorization.URL, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	stateCookie, err := r.Cookie(oidcStateCookieName)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(stateCookie.Value)) != 1 {
		s.redirectOIDCError(w, r, "invalid_state")
		return
	}
	flow, err := s.services.Identity.ConsumeOIDCFlow(r.Context(), state)
	if err != nil {
		s.redirectOIDCError(w, r, "expired_request")
		return
	}
	config, err := s.services.Identity.GetSignInConfigWithSecret(r.Context())
	if err != nil {
		s.redirectOIDCError(w, r, "configuration_unavailable")
		return
	}
	identity, err := s.oidc.Complete(r.Context(), config, r.URL.Query().Get("code"), flow)
	if err != nil {
		s.log.Warn("OIDC callback rejected", "error", err)
		s.redirectOIDCError(w, r, "identity_rejected")
		return
	}
	user, err := s.services.Identity.ResolveOIDCIdentity(r.Context(), identity, config.JITProvisioning)
	if err != nil {
		s.log.Warn("OIDC user provisioning rejected", "error", err)
		s.redirectOIDCError(w, r, "access_denied")
		return
	}
	token, err := s.services.Identity.CreateSession(r.Context(), user.ID, s.sessionExpiresAt())
	if err != nil {
		s.redirectOIDCError(w, r, "session_failed")
		return
	}
	s.clearOIDCStateCookie(w, r)
	s.setSessionCookie(w, r, token)
	http.Redirect(w, r, s.postLoginURL(config), http.StatusFound)
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
	if !s.allowAuthAttempt(w, r, "password-reset", "", s.cfg.PasswordResetAttempts) {
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
	resetLink := s.passwordResetLink(r, rawToken)
	if resetLink == "" {
		writeError(w, http.StatusServiceUnavailable, "PUBLIC_URL must be configured before reset links can be generated")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"resetLink": resetLink,
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

func (s *Server) handleVerifySignInConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	config, err := s.services.Identity.GetSignInConfigWithSecret(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sign-in configuration is unavailable")
		return
	}
	verifyContext, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.oidc.VerifyConfiguration(verifyContext, config); err != nil {
		writeError(w, http.StatusBadGateway, "OIDC discovery failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true})
}
