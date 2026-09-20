package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/lifecycle"
	"sort"
	"strings"
	"time"
)

func (r *MemoryRepository) ListDesigns(ctx context.Context, workspaceID string) ([]domain.Design, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.Design, PageInfo, error) {
		return r.ListDesignsPage(ctx, workspaceID, options)
	})
}

func (r *MemoryRepository) ListDesignsPage(ctx context.Context, workspaceID string, options PageOptions) ([]domain.Design, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	if workspaceID == "" {
		return nil, PageInfo{}, errors.New("workspace id is required")
	}
	options = NormalizePageOptions(options)
	query := strings.ToLower(options.Query)

	r.mu.RLock()
	defer r.mu.RUnlock()

	designs := make([]domain.Design, 0, len(r.designs))
	for _, design := range r.designs {
		if design.WorkspaceID == workspaceID {
			if query != "" && !strings.Contains(strings.ToLower(design.Name+" "+design.Title), query) {
				continue
			}
			designs = append(designs, design)
		}
	}

	sort.Slice(designs, func(i, j int) bool {
		return designs[i].UpdatedAt.After(designs[j].UpdatedAt)
	})

	page, info := PageFromSlice(designs, options)
	return page, info, nil
}

func (r *MemoryRepository) ListAccessibleDesignsPage(ctx context.Context, workspaceID string, scope AccessScope, options PageOptions) ([]domain.Design, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return nil, PageInfo{}, errors.New("workspace id is required")
	}
	options = NormalizePageOptions(options)
	query := strings.ToLower(options.Query)
	groups := stringSet(scope.GroupIDs)
	r.mu.RLock()
	defer r.mu.RUnlock()
	workspace, workspaceExists := r.workspaces[workspaceID]
	if !workspaceExists {
		return nil, PageInfo{}, errors.New("workspace not found")
	}
	workspaceManage := scope.IsAdmin || workspace.OwnerID == scope.UserID
	if access, ok := r.workspaceACL[accessKey(workspaceID, scope.UserID)]; ok && access.CanManage {
		workspaceManage = true
	}
	workspaceRead := workspaceManage
	if access, ok := r.workspaceACL[accessKey(workspaceID, scope.UserID)]; ok && (access.CanRead || access.CanCreateDesign) {
		workspaceRead = true
	}
	for _, access := range r.workspaceGACL {
		_, member := groups[access.GroupID]
		if access.WorkspaceID != workspaceID || !member {
			continue
		}
		workspaceManage = workspaceManage || access.CanManage
		workspaceRead = workspaceRead || access.CanRead || access.CanCreateDesign || access.CanManage
	}
	designs := make([]domain.Design, 0)
	for _, design := range r.designs {
		if design.WorkspaceID != workspaceID || (query != "" && !strings.Contains(strings.ToLower(design.Name+" "+design.Title), query)) {
			continue
		}
		canEdit := scope.IsAdmin || design.CreatedBy == scope.UserID || workspaceManage
		allowed := canEdit || design.Access == "public" || (design.Access == "workspace" && workspaceRead)
		if access, ok := r.designACL[accessKey(workspaceID, design.ID, scope.UserID)]; ok {
			allowed = allowed || access.CanRead || access.CanEdit || access.CanComment || access.CanReview || access.CanManage
			canEdit = canEdit || access.CanEdit || access.CanManage
		}
		for _, access := range r.designGACL {
			_, member := groups[access.GroupID]
			if access.WorkspaceID == workspaceID && access.DesignID == design.ID && member && (access.CanRead || access.CanEdit || access.CanComment || access.CanReview || access.CanManage) {
				allowed = true
				canEdit = canEdit || access.CanEdit || access.CanManage
			}
		}
		if allowed {
			design.EffectiveAccess = map[bool]string{true: "edit", false: "read"}[canEdit]
			designs = append(designs, design)
		}
	}
	sort.Slice(designs, func(i, j int) bool { return designs[i].UpdatedAt.After(designs[j].UpdatedAt) })
	page, info := PageFromSlice(designs, options)
	return page, info, nil
}

func (r *MemoryRepository) GetDesign(ctx context.Context, workspaceID string, designID string) (domain.Design, error) {
	if err := ctx.Err(); err != nil {
		return domain.Design{}, err
	}
	if workspaceID == "" || designID == "" {
		return domain.Design{}, errors.New("workspace id and design id are required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return domain.Design{}, errors.New("design not found")
	}
	return design, nil
}

func (r *MemoryRepository) CreateDesign(ctx context.Context, workspaceID string, name string, document []byte, createdBy string) (domain.Design, error) {
	if err := ctx.Err(); err != nil {
		return domain.Design{}, err
	}
	if workspaceID == "" {
		return domain.Design{}, errors.New("workspace id is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Untitled system design"
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock().UTC()
	if createdBy == "" {
		createdBy = r.firstUserIDLocked()
	}
	if _, ok := r.workspaces[workspaceID]; !ok {
		if workspaceID != domain.GuestWorkspaceID {
			return domain.Design{}, errors.New("workspace not found")
		}
		r.workspaces[workspaceID] = domain.NewGuestWorkspace(now)
	}
	designID := fmt.Sprintf("design_%d", now.UnixNano())
	storedDocument, err := initialDesignDocument(document, designID, name, now)
	if err != nil {
		return domain.Design{}, err
	}

	design := domain.Design{
		ID:               designID,
		WorkspaceID:      workspaceID,
		Name:             name,
		Access:           "private",
		Title:            name,
		Document:         storedDocument,
		DocumentRevision: domain.DesignRevision(storedDocument),
		VersionNumber:    0,
		CreatedBy:        createdBy,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	r.designs[design.ID] = design
	if workspace, ok := r.workspaces[workspaceID]; ok {
		workspace.UpdatedAt = now
		r.workspaces[workspace.ID] = workspace
	}
	return design, nil
}

func (r *MemoryRepository) UpdateDesignMetadata(ctx context.Context, workspaceID string, designID string, name string, access string) (domain.Design, error) {
	if err := ctx.Err(); err != nil {
		return domain.Design{}, err
	}
	if workspaceID == "" || designID == "" {
		return domain.Design{}, errors.New("workspace id and design id are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return domain.Design{}, errors.New("design not found")
	}
	if trimmedName := strings.TrimSpace(name); trimmedName != "" {
		design.Name = trimmedName
		design.Title = trimmedName
	}
	if trimmedAccess := strings.TrimSpace(access); trimmedAccess != "" {
		design.Access = trimmedAccess
	}
	if design.Access == "" {
		design.Access = "private"
	}
	design.UpdatedAt = r.clock().UTC()
	r.designs[design.ID] = design
	return design, nil
}

func (r *MemoryRepository) UpsertDesign(ctx context.Context, design domain.Design) (domain.Design, error) {
	if err := ctx.Err(); err != nil {
		return domain.Design{}, err
	}
	if design.ID == "" {
		return domain.Design{}, errors.New("design id is required")
	}
	if design.WorkspaceID == "" {
		return domain.Design{}, errors.New("workspace id is required")
	}
	if len(design.Document) == 0 {
		return domain.Design{}, errors.New("design document is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock().UTC()
	existing, exists := r.designs[design.ID]
	if exists {
		design.CreatedAt = existing.CreatedAt
		design.CreatedBy = existing.CreatedBy
		design.Name = existing.Name
		design.Access = existing.Access
		design.Title = existing.Title
		design.VersionNumber = existing.VersionNumber
	} else {
		design.CreatedAt = now
		if design.CreatedBy == "" {
			design.CreatedBy = r.firstUserIDLocked()
		}
		if design.Name == "" {
			design.Name = design.Title
		}
		if design.Access == "" {
			design.Access = "private"
		}
		design.VersionNumber = 0
	}
	if design.Title == "" {
		design.Title = design.Name
	}
	design.UpdatedAt = now
	design.DocumentRevision = domain.DesignRevision(design.Document)
	r.designs[design.ID] = design

	if workspace, ok := r.workspaces[design.WorkspaceID]; ok {
		workspace.UpdatedAt = now
		r.workspaces[workspace.ID] = workspace
	}

	return design, nil
}

func (r *MemoryRepository) UpdateDesignDocument(ctx context.Context, workspaceID string, designID string, document []byte, canvasSnapshot []byte, expectedRevision string) (domain.Design, error) {
	if err := ctx.Err(); err != nil {
		return domain.Design{}, err
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" {
		return domain.Design{}, errors.New("workspace id and design id are required")
	}
	if len(document) == 0 || !json.Valid(document) {
		return domain.Design{}, errors.New("design document must be valid JSON")
	}
	if expectedRevision == "" {
		return domain.Design{}, ErrDesignConflict
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return domain.Design{}, errors.New("design not found")
	}
	if domain.DesignRevision(design.Document) != expectedRevision {
		return domain.Design{}, ErrDesignConflict
	}
	now := r.clock().UTC()
	if !now.After(design.UpdatedAt) {
		now = design.UpdatedAt.Add(time.Nanosecond)
	}
	design.Document = append(json.RawMessage(nil), document...)
	design.DocumentRevision = domain.DesignRevision(design.Document)
	if domain.ValidCanvasSnapshot(canvasSnapshot) {
		design.CanvasSnapshot = append(json.RawMessage(nil), canvasSnapshot...)
	}
	design.UpdatedAt = now
	r.designs[design.ID] = design
	if workspace, exists := r.workspaces[workspaceID]; exists {
		workspace.UpdatedAt = now
		r.workspaces[workspace.ID] = workspace
	}
	return design, nil
}

func (r *MemoryRepository) markCurrentEditableVersionDraftLocked(design domain.Design, now time.Time) {
	if design.VersionNumber <= 0 {
		return
	}
	for index := range r.versions[design.ID] {
		version := &r.versions[design.ID][index]
		if version.VersionNumber != design.VersionNumber {
			continue
		}
		version.Status = lifecycle.StatusAfterDesignChange(version.Status)
		if strings.TrimSpace(design.VersionRemarks) != "" {
			version.Remarks = strings.TrimSpace(design.VersionRemarks)
		}
		version.Document = append([]byte(nil), design.Document...)
		version.CanvasSnapshot = append([]byte(nil), design.CanvasSnapshot...)
		version.UpdatedAt = now
		return
	}
}

func (r *MemoryRepository) DeleteDesign(ctx context.Context, workspaceID string, designID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if workspaceID == "" || designID == "" {
		return errors.New("workspace id and design id are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return errors.New("design not found")
	}
	delete(r.designs, designID)
	delete(r.versions, designID)
	for docID, doc := range r.docs {
		if doc.DesignID == designID && doc.WorkspaceID == workspaceID {
			delete(r.docs, docID)
		}
	}
	for commentID, comment := range r.comments {
		if comment.DesignID == designID && comment.WorkspaceID == workspaceID {
			delete(r.comments, commentID)
		}
	}
	for reviewID, review := range r.reviews {
		if review.DesignID == designID && review.WorkspaceID == workspaceID {
			delete(r.reviews, reviewID)
		}
	}
	for notificationID, notification := range r.notifications {
		if notification.DesignID == designID && notification.WorkspaceID == workspaceID {
			delete(r.notifications, notificationID)
		}
	}
	for conversationID, conversation := range r.aiConversations {
		if conversation.DesignID != designID || conversation.WorkspaceID != workspaceID {
			continue
		}
		delete(r.aiConversations, conversationID)
		for messageID, message := range r.aiMessages {
			if message.ConversationID == conversationID {
				delete(r.aiMessages, messageID)
			}
		}
	}
	if workspace, ok := r.workspaces[workspaceID]; ok {
		workspace.UpdatedAt = r.clock().UTC()
		r.workspaces[workspaceID] = workspace
	}
	return nil
}

func (r *MemoryRepository) ListDesignVersions(ctx context.Context, workspaceID string, designID string) ([]domain.DesignVersion, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.DesignVersion, PageInfo, error) {
		return r.ListDesignVersionsPage(ctx, workspaceID, designID, options)
	})
}

func (r *MemoryRepository) ListDesignVersionsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignVersion, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	if workspaceID == "" || designID == "" {
		return nil, PageInfo{}, errors.New("workspace id and design id are required")
	}
	options = NormalizePageOptions(options)

	r.mu.RLock()
	defer r.mu.RUnlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return nil, PageInfo{}, errors.New("design not found")
	}

	versions := append([]domain.DesignVersion(nil), r.versions[designID]...)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].VersionNumber > versions[j].VersionNumber
	})
	page, info := PageFromSlice(versions, options)
	return page, info, nil
}

func (r *MemoryRepository) CreateDesignVersion(ctx context.Context, workspaceID string, designID string, createdBy string, remarks string) (domain.DesignVersion, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignVersion{}, err
	}
	if workspaceID == "" || designID == "" {
		return domain.DesignVersion{}, errors.New("workspace id and design id are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return domain.DesignVersion{}, errors.New("design not found")
	}
	now := r.clock().UTC()
	if createdBy == "" {
		createdBy = r.firstUserIDLocked()
	}
	versionNumber := len(r.versions[designID]) + 1
	version := domain.DesignVersion{
		ID:             fmt.Sprintf("%s_v%d", design.ID, versionNumber),
		DesignID:       design.ID,
		WorkspaceID:    design.WorkspaceID,
		VersionNumber:  versionNumber,
		Status:         "draft",
		Remarks:        strings.TrimSpace(remarks),
		Document:       append([]byte(nil), design.Document...),
		CanvasSnapshot: append([]byte(nil), design.CanvasSnapshot...),
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	r.versions[designID] = append(r.versions[designID], version)
	design.VersionNumber = versionNumber
	design.UpdatedAt = now
	r.designs[design.ID] = design
	return version, nil
}

func (r *MemoryRepository) UpdateDraftDesignVersion(ctx context.Context, workspaceID string, designID string, versionID string, document []byte, canvasSnapshot []byte, remarks string) (domain.DesignVersion, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignVersion{}, err
	}
	if !json.Valid(document) {
		return domain.DesignVersion{}, errors.New("design document must be valid JSON")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if design, ok := r.designs[designID]; !ok || design.WorkspaceID != workspaceID {
		return domain.DesignVersion{}, errors.New("design not found")
	}
	versions := r.versions[designID]
	for index := range versions {
		if versions[index].ID != versionID {
			continue
		}
		if versions[index].Status != "draft" {
			return domain.DesignVersion{}, errors.New("only draft versions can be overwritten")
		}
		versions[index].Document = append(json.RawMessage(nil), document...)
		if domain.ValidCanvasSnapshot(canvasSnapshot) {
			versions[index].CanvasSnapshot = append(json.RawMessage(nil), canvasSnapshot...)
		}
		if trimmed := strings.TrimSpace(remarks); trimmed != "" {
			versions[index].Remarks = trimmed
		}
		versions[index].UpdatedAt = r.clock().UTC()
		r.versions[designID] = versions
		return versions[index], nil
	}
	return domain.DesignVersion{}, errors.New("version not found")
}

func (r *MemoryRepository) UpdateDesignVersionStatus(ctx context.Context, workspaceID string, designID string, versionID string, status string) (domain.DesignVersion, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignVersion{}, err
	}
	requestedStatus := status
	status = NormalizeDesignVersionStatus(status)
	if workspaceID == "" || designID == "" || versionID == "" {
		return domain.DesignVersion{}, errors.New("workspace id, design id, and version id are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return domain.DesignVersion{}, errors.New("design not found")
	}
	now := r.clock().UTC()
	var currentStatus string
	found := false
	for index := range r.versions[designID] {
		version := &r.versions[designID][index]
		if version.ID == versionID && version.WorkspaceID == workspaceID {
			currentStatus = version.Status
			found = true
			break
		}
	}
	if !found {
		return domain.DesignVersion{}, errors.New("version not found")
	}
	if err := ValidateDesignVersionStatusTransition(currentStatus, requestedStatus); err != nil {
		return domain.DesignVersion{}, err
	}
	return r.updateDesignVersionStatusLocked(workspaceID, designID, versionID, status, now)
}

func (r *MemoryRepository) updateDesignVersionStatusLocked(workspaceID string, designID string, versionID string, status string, now time.Time) (domain.DesignVersion, error) {
	for index := range r.versions[designID] {
		version := &r.versions[designID][index]
		if status == "live" && version.ID != versionID && version.Status == "live" {
			version.Status = "reviewed"
			version.UpdatedAt = now
		}
	}
	for index := range r.versions[designID] {
		version := &r.versions[designID][index]
		if version.ID == versionID && version.WorkspaceID == workspaceID {
			version.Status = status
			version.UpdatedAt = now
			return *version, nil
		}
	}
	return domain.DesignVersion{}, errors.New("version not found")
}

func (r *MemoryRepository) DeleteDesignVersion(ctx context.Context, workspaceID string, designID string, versionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if workspaceID == "" || designID == "" || versionID == "" {
		return errors.New("workspace id, design id, and version id are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return errors.New("design not found")
	}

	versions := r.versions[designID]
	nextVersions := versions[:0]
	deleted := false
	maxVersionNumber := 0
	for _, version := range versions {
		if version.ID == versionID && version.WorkspaceID == workspaceID {
			deleted = true
			continue
		}
		if version.VersionNumber > maxVersionNumber {
			maxVersionNumber = version.VersionNumber
		}
		nextVersions = append(nextVersions, version)
	}
	if !deleted {
		return errors.New("version not found")
	}
	r.versions[designID] = nextVersions
	for reviewID, review := range r.reviews {
		if review.WorkspaceID == workspaceID && review.DesignID == designID && review.VersionID == versionID {
			delete(r.reviews, reviewID)
		}
	}
	design.VersionNumber = maxVersionNumber
	design.UpdatedAt = r.clock().UTC()
	r.designs[designID] = design
	return nil
}
