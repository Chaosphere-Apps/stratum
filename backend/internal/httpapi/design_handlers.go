package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/system-design-evaluator/backend/internal/analysis"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/policy"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
	"net/http"
	"strings"
)

func (s *Server) handleListDesigns(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceID")
	user, ok := s.requireWorkspaceAccess(w, r, workspaceID, policy.WorkspaceRead)
	if !ok {
		return
	}
	designs, page, err := s.services.Designs.ListAccessiblePage(r.Context(), workspaceID, user, pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"designs": designs, "page": page})
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
		BaseRevision   string          `json:"baseRevision"`
		VersionRemarks string          `json:"versionRemarks"`
		VersionID      string          `json:"versionId"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.Document) == 0 || !json.Valid(body.Document) {
		writeError(w, http.StatusBadRequest, "design document must be valid JSON")
		return
	}
	canvasSnapshot := existing.CanvasSnapshot
	if domain.ValidCanvasSnapshot(body.CanvasSnapshot) {
		canvasSnapshot = body.CanvasSnapshot
	}
	updated, err := s.services.Designs.UpdateDocument(r.Context(), workspaceID, designID, body.Document, canvasSnapshot, body.BaseRevision)
	if err != nil {
		if errors.Is(err, store.ErrDesignConflict) {
			current, currentErr := s.services.Designs.Get(r.Context(), workspaceID, designID)
			if currentErr == nil {
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "design": current})
				return
			}
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.hub != nil {
		if encoded, encodeErr := json.Marshal(realtime.DesignUpdatedPayload{Design: updated}); encodeErr == nil {
			s.hub.Broadcast(r.Context(), workspaceID, realtime.Envelope{Type: realtime.MessageDesignUpdated, Payload: encoded})
		} else {
			s.log.Error("failed to encode design update broadcast", "workspaceId", workspaceID, "designId", designID, "error", encodeErr)
		}
	}
	response := map[string]any{"design": updated}
	if versionID := strings.TrimSpace(body.VersionID); versionID != "" {
		version, err := s.services.Versions.UpdateDraft(r.Context(), workspaceID, designID, versionID, updated.Document, updated.CanvasSnapshot, body.VersionRemarks)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		response["version"] = version
	}
	writeJSON(w, http.StatusOK, response)
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
	var body struct {
		VersionID string `json:"versionId"`
		Focus     string `json:"focus"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	document := design.Document
	options := analysis.ReviewOptions{Focus: normalizedReviewFocus(body.Focus)}
	if strings.TrimSpace(body.VersionID) != "" {
		version, err := s.designVersionByID(r.Context(), workspaceID, designID, body.VersionID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		document = version.Document
		options.VersionID = version.ID
		options.VersionNumber = version.VersionNumber
	}

	report, err := s.services.Analysis.AnalyzeDocument(document)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if aiConfig, err := s.services.Identity.GetAIProviderConfigWithSecret(r.Context()); err == nil && aiConfig.Enabled && strings.TrimSpace(aiConfig.APIKey) != "" {
		review, err := runAIAnalysis(r.Context(), s.ai, aiConfig, document, report, options)
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

func normalizedReviewFocus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "security", "scalability", "reliability", "data", "operability", "cost":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "full"
	}
}

func (s *Server) designVersionByID(ctx context.Context, workspaceID string, designID string, versionID string) (domain.DesignVersion, error) {
	versions, err := s.services.Versions.List(ctx, workspaceID, designID)
	if err != nil {
		return domain.DesignVersion{}, err
	}
	for _, version := range versions {
		if version.ID == versionID {
			return version, nil
		}
	}
	return domain.DesignVersion{}, errors.New("design version not found")
}
