package httpapi

import (
	"context"
	"encoding/json"
	"github.com/system-design-evaluator/backend/internal/policy"
	"github.com/system-design-evaluator/backend/internal/store"
	"net/http"
	"os"
	urlpath "path"
	"path/filepath"
	"strings"
)

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
