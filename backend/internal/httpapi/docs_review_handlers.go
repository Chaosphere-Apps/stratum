package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/system-design-evaluator/backend/internal/app"
	"github.com/system-design-evaluator/backend/internal/policy"
)

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
	user, _, ok := s.requireDesignAccess(w, r, workspaceID, designID, policy.DesignReview)
	if !ok {
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
	review, err := s.services.Reviews.UpdateReview(r.Context(), workspaceID, designID, reviewID, user, body.Status, body.Summary)
	if err != nil {
		if errors.Is(err, app.ErrReviewNotAssigned) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, statusForDeleteError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": review})
}
