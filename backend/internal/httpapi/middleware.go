package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/policy"
	"github.com/system-design-evaluator/backend/internal/store"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

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

const oidcStateCookieName = "stratum_oidc_state"

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
		Secure:   s.requestIsSecure(r),
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
		Secure:   s.requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) setOIDCStateCookie(w http.ResponseWriter, r *http.Request, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    state,
		Path:     "/api/auth/oidc/callback",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   s.requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearOIDCStateCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    "",
		Path:     "/api/auth/oidc/callback",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		Secure:   s.requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) postLoginURL(config domain.SignInConfig) string {
	if base := strings.TrimSpace(s.cfg.PublicURL); base != "" {
		return strings.TrimRight(base, "/") + "/"
	}
	if redirect, err := url.Parse(config.RedirectURI); err == nil && redirect.Scheme != "" && redirect.Host != "" {
		return (&url.URL{Scheme: redirect.Scheme, Host: redirect.Host, Path: "/"}).String()
	}
	return "/"
}

func (s *Server) redirectOIDCError(w http.ResponseWriter, r *http.Request, code string) {
	config, _ := s.services.Identity.GetSignInConfig(r.Context())
	target, err := url.Parse(s.postLoginURL(config))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "SSO sign-in failed")
		return
	}
	target.Path = "/login"
	query := target.Query()
	query.Set("authError", code)
	target.RawQuery = query.Encode()
	s.clearOIDCStateCookie(w, r)
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (s *Server) passwordResetLink(r *http.Request, token string) string {
	configured := strings.TrimSpace(s.cfg.PublicURL)
	if configured == "" && len(s.cfg.AllowedOrigins) == 1 {
		configured = strings.TrimSpace(s.cfg.AllowedOrigins[0])
	}
	if configured != "" {
		if parsed, err := url.Parse(configured); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			parsed.Path = "/reset-password"
			query := parsed.Query()
			query.Set("token", token)
			parsed.RawQuery = query.Encode()
			return parsed.String()
		}
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if parsedIP := net.ParseIP(strings.Trim(host, "[]")); !strings.EqualFold(host, "localhost") && (parsedIP == nil || !parsedIP.IsLoopback()) {
		return ""
	}
	link := url.URL{
		Scheme: "http",
		Host:   r.Host,
		Path:   "/reset-password",
	}
	query := link.Query()
	query.Set("token", token)
	link.RawQuery = query.Encode()
	return link.String()
}

func (s *Server) requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if configured, err := url.Parse(s.cfg.PublicURL); err == nil && strings.EqualFold(configured.Scheme, "https") {
		return true
	}
	return s.isTrustedProxy(r) && strings.EqualFold(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0], "https")
}

func (s *Server) isTrustedProxy(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	for _, raw := range s.cfg.TrustedProxyCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func (s *Server) clientIP(r *http.Request) string {
	if s.isTrustedProxy(r) {
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
			return forwarded
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (s *Server) allowAuthAttempt(w http.ResponseWriter, r *http.Request, action string, identity string, limit int) bool {
	identity = strings.ToLower(strings.TrimSpace(identity))
	key := action + ":" + s.clientIP(r) + ":" + identity
	allowed, retry := s.authRate.Allow(key, limit, s.cfg.AuthRateLimitWindow)
	if allowed {
		return true
	}
	seconds := int(retry.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "too many authentication attempts; try again later")
	return false
}

func (s *Server) allowAIAttempt(w http.ResponseWriter, userID, designID string) bool {
	allowed, retry := s.aiRate.Allow("chat:"+userID+":"+designID, 30, 5*time.Minute)
	if allowed {
		return true
	}
	seconds := max(1, int(retry.Seconds()))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "AI request limit reached; try again shortly")
	return false
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
