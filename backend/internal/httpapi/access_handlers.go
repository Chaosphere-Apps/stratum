package httpapi

import (
	"github.com/system-design-evaluator/backend/internal/app"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/policy"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return
	}
	workspaces, page, err := s.services.Workspaces.ListAccessiblePage(r.Context(), user, pageOptionsFromRequest(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces, "page": page})
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireRole(w, r, "architect")
	if !ok {
		return
	}
	var body struct {
		Name   string `json:"name"`
		Shares []struct {
			PrincipalType string `json:"principalType"`
			PrincipalID   string `json:"principalId"`
			AccessLevel   string `json:"accessLevel"`
		} `json:"shares"`
	}
	if err := decodeJSON(w, r, s.cfg.MaxRequestBodyBytes, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	shares := make([]app.WorkspaceShare, 0, len(body.Shares))
	for _, share := range body.Shares {
		shares = append(shares, app.WorkspaceShare{
			PrincipalType: share.PrincipalType,
			PrincipalID:   share.PrincipalID,
			AccessLevel:   share.AccessLevel,
		})
	}
	workspace, err := s.services.Workspaces.CreateWithAccess(r.Context(), body.Name, user.ID, shares)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"workspace": workspace})
}

func (s *Server) handleListWorkspaceSharePrincipals(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session is required")
		return
	}
	users, err := s.services.Identity.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	groups, err := s.services.ACL.ListGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type principal struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	principals := make([]principal, 0, len(users)+len(groups))
	for _, candidate := range users {
		if candidate.ID == user.ID || candidate.Status != "active" {
			continue
		}
		principals = append(principals, principal{ID: candidate.ID, Type: "user", Name: candidate.DisplayName, Description: candidate.Email})
	}
	for _, group := range groups {
		principals = append(principals, principal{ID: group.ID, Type: "group", Name: group.Name, Description: group.Description})
	}
	writeJSON(w, http.StatusOK, map[string]any{"principals": principals})
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
