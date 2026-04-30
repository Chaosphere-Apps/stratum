package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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
	s.mux.HandleFunc("GET /api/profile", s.handleProfile)
	s.mux.HandleFunc("POST /api/ai/provider/verify", s.handleVerifyAIProvider)
	s.mux.HandleFunc("GET /api/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("POST /api/workspaces", s.handleCreateWorkspace)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs", s.handleListDesigns)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs", s.handleCreateDesign)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}", s.handleGetDesign)
	s.mux.HandleFunc("PATCH /api/workspaces/{workspaceID}/designs/{designID}", s.handleUpdateDesignMetadata)
	s.mux.HandleFunc("PUT /api/workspaces/{workspaceID}/designs/{designID}/document", s.handleSaveDesignDocument)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/designs/{designID}/analysis", s.handleAnalyzeDesign)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/designs/{designID}/versions", s.handleListDesignVersions)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleWorkspaceSocket(w http.ResponseWriter, r *http.Request) {
	workspaceID := strings.TrimSpace(r.URL.Query().Get("workspaceId"))
	if workspaceID == "" {
		workspaceID = domain.GuestWorkspaceID
	}
	if _, err := s.hub.Repository().GetWorkspace(r.Context(), workspaceID); err != nil {
		http.Error(w, "workspace not found", http.StatusNotFound)
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

	client := realtime.NewClient(conn, s.hub, s.log, workspaceID)
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

func (s *Server) handleProfile(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          domain.GuestUserID,
		"displayName": "Guest Designer",
		"role":        "guest",
	})
}

func (s *Server) handleVerifyAIProvider(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.hub.Repository().ListWorkspaces(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusCreated, map[string]any{"workspace": workspace})
}

func (s *Server) handleListDesigns(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designs, err := s.hub.Repository().ListDesigns(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"designs": designs})
}

func (s *Server) handleCreateDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	var body struct {
		Name     string          `json:"name"`
		Document json.RawMessage `json:"document"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	design, err := s.hub.Repository().CreateDesign(r.Context(), workspaceID, body.Name, body.Document)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"design": design})
}

func (s *Server) handleGetDesign(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
	design, err := s.hub.Repository().GetDesign(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"design": design})
}

func (s *Server) handleUpdateDesignMetadata(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
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

func (s *Server) handleSaveDesignDocument(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	designID := r.PathValue("designID")
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
	existing, err := s.hub.Repository().GetDesign(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
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
	design, err := s.hub.Repository().GetDesign(r.Context(), workspaceID, designID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	report, err := analysis.New().Analyze(design.Document)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"analysis": report})
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

func writeCORSHeaders(w http.ResponseWriter, r *http.Request, allowedOrigins []string) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if isAllowedOrigin(origin, allowedOrigins) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Access-Control-Max-Age", "600")
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
