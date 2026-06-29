package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/lifecycle"
)

type MemoryRepository struct {
	mu              sync.RWMutex
	workspaces      map[string]domain.Workspace
	designs         map[string]domain.Design
	versions        map[string][]domain.DesignVersion
	docs            map[string]domain.DesignDoc
	users           map[string]domain.User
	sessions        map[string]memorySession
	passwordReset   map[string]domain.PasswordResetToken
	accessGroups    map[string]domain.AccessGroup
	groupMembers    map[string]domain.AccessGroupMember
	workspaceACL    map[string]domain.WorkspaceAccess
	workspaceGACL   map[string]domain.WorkspaceGroupAccess
	designACL       map[string]domain.DesignAccess
	designGACL      map[string]domain.DesignGroupAccess
	signInConfig    domain.SignInConfig
	aiConfig        domain.AIProviderConfig
	mcpConfig       domain.MCPConfig
	telemetryConfig domain.TelemetryIntegrationConfig
	catalogAssets   map[string]domain.CatalogAsset
	comments        map[string]domain.DesignComment
	reviews         map[string]domain.DesignReviewRequest
	notifications   map[string]domain.Notification
	clock           func() time.Time
}

type memorySession struct {
	UserID     string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		workspaces:      make(map[string]domain.Workspace),
		designs:         make(map[string]domain.Design),
		versions:        make(map[string][]domain.DesignVersion),
		docs:            make(map[string]domain.DesignDoc),
		users:           make(map[string]domain.User),
		sessions:        make(map[string]memorySession),
		passwordReset:   make(map[string]domain.PasswordResetToken),
		accessGroups:    make(map[string]domain.AccessGroup),
		groupMembers:    make(map[string]domain.AccessGroupMember),
		workspaceACL:    make(map[string]domain.WorkspaceAccess),
		workspaceGACL:   make(map[string]domain.WorkspaceGroupAccess),
		designACL:       make(map[string]domain.DesignAccess),
		designGACL:      make(map[string]domain.DesignGroupAccess),
		signInConfig:    defaultSignInConfig(time.Now().UTC()),
		aiConfig:        defaultAIProviderConfig(time.Now().UTC()),
		mcpConfig:       defaultMCPConfig(time.Now().UTC()),
		telemetryConfig: defaultTelemetryIntegrationConfig(time.Now().UTC()),
		catalogAssets:   make(map[string]domain.CatalogAsset),
		comments:        make(map[string]domain.DesignComment),
		reviews:         make(map[string]domain.DesignReviewRequest),
		notifications:   make(map[string]domain.Notification),
		clock:           time.Now,
	}
}

func (r *MemoryRepository) GetOrCreateGuestWorkspace(ctx context.Context) (domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if workspace, ok := r.workspaces[domain.GuestWorkspaceID]; ok {
		return workspace, nil
	}

	{
		now := r.clock().UTC()
		workspace := domain.NewGuestWorkspace(now)
		r.workspaces[workspace.ID] = workspace
	}

	return r.workspaces[domain.GuestWorkspaceID], nil
}

func (r *MemoryRepository) ListWorkspaces(ctx context.Context) ([]domain.Workspace, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.Workspace, PageInfo, error) {
		return r.ListWorkspacesPage(ctx, options)
	})
}

func (r *MemoryRepository) ListWorkspacesPage(ctx context.Context, options PageOptions) ([]domain.Workspace, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	query := strings.ToLower(options.Query)

	r.mu.RLock()
	defer r.mu.RUnlock()

	workspaces := make([]domain.Workspace, 0, len(r.workspaces))
	for _, workspace := range r.workspaces {
		if query != "" && !strings.Contains(strings.ToLower(workspace.Name), query) {
			continue
		}
		workspaces = append(workspaces, workspace)
	}
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt)
	})
	page, info := PageFromSlice(workspaces, options)
	return page, info, nil
}

func (r *MemoryRepository) GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}
	if workspaceID == domain.GuestWorkspaceID {
		return r.GetOrCreateGuestWorkspace(ctx)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	workspace, ok := r.workspaces[workspaceID]
	if !ok {
		return domain.Workspace{}, errors.New("workspace not found")
	}
	return workspace, nil
}

func (r *MemoryRepository) CreateWorkspace(ctx context.Context, name string) (domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Workspace{}, errors.New("workspace name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.clock().UTC()
	workspace := domain.Workspace{
		ID:        fmt.Sprintf("workspace_%d", now.UnixNano()),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.workspaces[workspace.ID] = workspace
	return workspace, nil
}

func (r *MemoryRepository) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(workspaceID) == "" {
		return errors.New("workspace id is required")
	}
	if workspaceID == domain.GuestWorkspaceID {
		return errors.New("default workspace cannot be deleted")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.workspaces[workspaceID]; !ok {
		return errors.New("workspace not found")
	}
	for _, design := range r.designs {
		if design.WorkspaceID == workspaceID {
			return errors.New("workspace is not empty")
		}
	}
	delete(r.workspaces, workspaceID)
	return nil
}

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
		ID:            designID,
		WorkspaceID:   workspaceID,
		Name:          name,
		Access:        "private",
		Title:         name,
		Document:      storedDocument,
		VersionNumber: 0,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
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
	r.designs[design.ID] = design
	r.markCurrentEditableVersionDraftLocked(design, now)

	if workspace, ok := r.workspaces[design.WorkspaceID]; ok {
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

func (r *MemoryRepository) ListDesignDocs(ctx context.Context, workspaceID string, designID string) ([]domain.DesignDoc, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if workspaceID == "" || designID == "" {
		return nil, errors.New("workspace id and design id are required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if design, ok := r.designs[designID]; !ok || design.WorkspaceID != workspaceID {
		return nil, errors.New("design not found")
	}
	docs := make([]domain.DesignDoc, 0, len(r.docs))
	for _, doc := range r.docs {
		if doc.WorkspaceID == workspaceID && doc.DesignID == designID {
			docs = append(docs, doc)
		}
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].UpdatedAt.After(docs[j].UpdatedAt)
	})
	return docs, nil
}

func (r *MemoryRepository) GetDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) (domain.DesignDoc, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignDoc{}, err
	}
	if workspaceID == "" || designID == "" || docID == "" {
		return domain.DesignDoc{}, errors.New("workspace id, design id, and doc id are required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	doc, ok := r.docs[docID]
	if !ok || doc.WorkspaceID != workspaceID || doc.DesignID != designID {
		return domain.DesignDoc{}, errors.New("design doc not found")
	}
	return doc, nil
}

func (r *MemoryRepository) CreateDesignDoc(ctx context.Context, workspaceID string, designID string, title string, body string, format string) (domain.DesignDoc, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignDoc{}, err
	}
	if workspaceID == "" || designID == "" {
		return domain.DesignDoc{}, errors.New("workspace id and design id are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if design, ok := r.designs[designID]; !ok || design.WorkspaceID != workspaceID {
		return domain.DesignDoc{}, errors.New("design not found")
	}
	now := r.clock().UTC()
	doc := domain.DesignDoc{
		ID:          fmt.Sprintf("doc_%d", now.UnixNano()),
		WorkspaceID: workspaceID,
		DesignID:    designID,
		Title:       normalizedDocTitle(title),
		Body:        body,
		Format:      normalizedDocFormat(format),
		CreatedBy:   r.firstUserIDLocked(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.docs[doc.ID] = doc
	return doc, nil
}

func (r *MemoryRepository) UpdateDesignDoc(ctx context.Context, workspaceID string, designID string, docID string, title string, body string, format string) (domain.DesignDoc, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignDoc{}, err
	}
	if workspaceID == "" || designID == "" || docID == "" {
		return domain.DesignDoc{}, errors.New("workspace id, design id, and doc id are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	doc, ok := r.docs[docID]
	if !ok || doc.WorkspaceID != workspaceID || doc.DesignID != designID {
		return domain.DesignDoc{}, errors.New("design doc not found")
	}
	doc.Title = normalizedDocTitle(title)
	doc.Body = body
	doc.Format = normalizedDocFormat(format)
	doc.UpdatedAt = r.clock().UTC()
	r.docs[doc.ID] = doc
	return doc, nil
}

func (r *MemoryRepository) DeleteDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if workspaceID == "" || designID == "" || docID == "" {
		return errors.New("workspace id, design id, and doc id are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	doc, ok := r.docs[docID]
	if !ok || doc.WorkspaceID != workspaceID || doc.DesignID != designID {
		return errors.New("design doc not found")
	}
	delete(r.docs, docID)
	return nil
}

func (r *MemoryRepository) ListAccessGroups(ctx context.Context) ([]domain.AccessGroup, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	groups := make([]domain.AccessGroup, 0, len(r.accessGroups))
	for _, group := range r.accessGroups {
		group.MemberCount = r.accessGroupMemberCountLocked(group.ID)
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name) })
	return groups, nil
}

func (r *MemoryRepository) CreateAccessGroup(ctx context.Context, group domain.AccessGroup) (domain.AccessGroup, error) {
	if err := ctx.Err(); err != nil {
		return domain.AccessGroup{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	name := strings.TrimSpace(group.Name)
	if name == "" {
		return domain.AccessGroup{}, errors.New("group name is required")
	}
	for _, existing := range r.accessGroups {
		if strings.EqualFold(existing.Name, name) {
			return domain.AccessGroup{}, errors.New("group name already exists")
		}
	}
	now := r.clock().UTC()
	group.ID = fmt.Sprintf("grp_%d", now.UnixNano())
	group.Name = name
	group.Description = strings.TrimSpace(group.Description)
	group.OktaGroupName = strings.TrimSpace(group.OktaGroupName)
	group.CreatedAt = now
	group.UpdatedAt = now
	r.accessGroups[group.ID] = group
	return group, nil
}

func (r *MemoryRepository) UpdateAccessGroup(ctx context.Context, groupID string, group domain.AccessGroup) (domain.AccessGroup, error) {
	if err := ctx.Err(); err != nil {
		return domain.AccessGroup{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	groupID = strings.TrimSpace(groupID)
	existing, ok := r.accessGroups[groupID]
	if !ok {
		return domain.AccessGroup{}, errors.New("group not found")
	}
	name := strings.TrimSpace(group.Name)
	if name == "" {
		return domain.AccessGroup{}, errors.New("group name is required")
	}
	for _, other := range r.accessGroups {
		if other.ID != groupID && strings.EqualFold(other.Name, name) {
			return domain.AccessGroup{}, errors.New("group name already exists")
		}
	}
	existing.Name = name
	existing.Description = strings.TrimSpace(group.Description)
	existing.OktaGroupName = strings.TrimSpace(group.OktaGroupName)
	existing.UpdatedAt = r.clock().UTC()
	existing.MemberCount = r.accessGroupMemberCountLocked(groupID)
	r.accessGroups[groupID] = existing
	return existing, nil
}

func (r *MemoryRepository) DeleteAccessGroup(ctx context.Context, groupID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	groupID = strings.TrimSpace(groupID)
	if _, ok := r.accessGroups[groupID]; !ok {
		return errors.New("group not found")
	}
	delete(r.accessGroups, groupID)
	for key, member := range r.groupMembers {
		if member.GroupID == groupID {
			delete(r.groupMembers, key)
		}
	}
	for key, access := range r.workspaceGACL {
		if access.GroupID == groupID {
			delete(r.workspaceGACL, key)
		}
	}
	for key, access := range r.designGACL {
		if access.GroupID == groupID {
			delete(r.designGACL, key)
		}
	}
	return nil
}

func (r *MemoryRepository) ListAccessGroupMembers(ctx context.Context, groupID string) ([]domain.AccessGroupMember, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.accessGroups[strings.TrimSpace(groupID)]; !ok {
		return nil, errors.New("group not found")
	}
	members := []domain.AccessGroupMember{}
	for _, member := range r.groupMembers {
		if member.GroupID == groupID {
			members = append(members, member)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	return members, nil
}

func (r *MemoryRepository) ReplaceAccessGroupMembers(ctx context.Context, groupID string, userIDs []string) ([]domain.AccessGroupMember, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	groupID = strings.TrimSpace(groupID)
	if _, ok := r.accessGroups[groupID]; !ok {
		return nil, errors.New("group not found")
	}
	now := r.clock().UTC()
	next := map[string]domain.AccessGroupMember{}
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, err := r.getUserLocked(userID); err != nil {
			return nil, err
		}
		key := accessKey(groupID, userID)
		member := domain.AccessGroupMember{GroupID: groupID, UserID: userID, AddedAt: now}
		if existing, ok := r.groupMembers[key]; ok {
			member.AddedAt = existing.AddedAt
		}
		next[key] = member
	}
	for key, member := range r.groupMembers {
		if member.GroupID == groupID {
			delete(r.groupMembers, key)
		}
	}
	for key, member := range next {
		r.groupMembers[key] = member
	}
	members := make([]domain.AccessGroupMember, 0, len(next))
	for _, member := range next {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	return members, nil
}

func (r *MemoryRepository) ListUserAccessGroupIDs(ctx context.Context, userID string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	groupIDs := []string{}
	for _, member := range r.groupMembers {
		if member.UserID == strings.TrimSpace(userID) {
			groupIDs = append(groupIDs, member.GroupID)
		}
	}
	sort.Strings(groupIDs)
	return groupIDs, nil
}

func (r *MemoryRepository) ListWorkspaceAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceAccess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("workspace id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	access := []domain.WorkspaceAccess{}
	for _, entry := range r.workspaceACL {
		if entry.WorkspaceID == workspaceID {
			access = append(access, entry)
		}
	}
	sort.Slice(access, func(i, j int) bool { return access[i].UserID < access[j].UserID })
	return access, nil
}

func (r *MemoryRepository) GrantWorkspaceAccess(ctx context.Context, access domain.WorkspaceAccess) (domain.WorkspaceAccess, error) {
	if err := ctx.Err(); err != nil {
		return domain.WorkspaceAccess{}, err
	}
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.UserID = strings.TrimSpace(access.UserID)
	if access.WorkspaceID == "" || access.UserID == "" {
		return domain.WorkspaceAccess{}, errors.New("workspace id and user id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[access.WorkspaceID]; !ok {
		return domain.WorkspaceAccess{}, errors.New("workspace not found")
	}
	if _, err := r.getUserLocked(access.UserID); err != nil {
		return domain.WorkspaceAccess{}, err
	}
	now := r.clock().UTC()
	key := accessKey(access.WorkspaceID, access.UserID)
	if existing, ok := r.workspaceACL[key]; ok {
		access.CreatedAt = existing.CreatedAt
	} else {
		access.CreatedAt = now
	}
	access.UpdatedAt = now
	r.workspaceACL[key] = access
	return access, nil
}

func (r *MemoryRepository) RevokeWorkspaceAccess(ctx context.Context, workspaceID string, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workspaceACL, accessKey(workspaceID, userID))
	return nil
}

func (r *MemoryRepository) ListWorkspaceGroupAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceGroupAccess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	access := []domain.WorkspaceGroupAccess{}
	for _, entry := range r.workspaceGACL {
		if entry.WorkspaceID == strings.TrimSpace(workspaceID) {
			access = append(access, entry)
		}
	}
	sort.Slice(access, func(i, j int) bool { return access[i].GroupID < access[j].GroupID })
	return access, nil
}

func (r *MemoryRepository) GrantWorkspaceGroupAccess(ctx context.Context, access domain.WorkspaceGroupAccess) (domain.WorkspaceGroupAccess, error) {
	if err := ctx.Err(); err != nil {
		return domain.WorkspaceGroupAccess{}, err
	}
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.GroupID = strings.TrimSpace(access.GroupID)
	if access.WorkspaceID == "" || access.GroupID == "" {
		return domain.WorkspaceGroupAccess{}, errors.New("workspace id and group id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[access.WorkspaceID]; !ok {
		return domain.WorkspaceGroupAccess{}, errors.New("workspace not found")
	}
	if _, ok := r.accessGroups[access.GroupID]; !ok {
		return domain.WorkspaceGroupAccess{}, errors.New("group not found")
	}
	now := r.clock().UTC()
	key := accessKey(access.WorkspaceID, access.GroupID)
	if existing, ok := r.workspaceGACL[key]; ok {
		access.CreatedAt = existing.CreatedAt
	} else {
		access.CreatedAt = now
	}
	access.UpdatedAt = now
	r.workspaceGACL[key] = access
	return access, nil
}

func (r *MemoryRepository) RevokeWorkspaceGroupAccess(ctx context.Context, workspaceID string, groupID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workspaceGACL, accessKey(workspaceID, groupID))
	return nil
}

func (r *MemoryRepository) ListDesignAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignAccess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	access := []domain.DesignAccess{}
	for _, entry := range r.designACL {
		if entry.WorkspaceID == workspaceID && entry.DesignID == designID {
			access = append(access, entry)
		}
	}
	sort.Slice(access, func(i, j int) bool { return access[i].UserID < access[j].UserID })
	return access, nil
}

func (r *MemoryRepository) GrantDesignAccess(ctx context.Context, access domain.DesignAccess) (domain.DesignAccess, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignAccess{}, err
	}
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.DesignID = strings.TrimSpace(access.DesignID)
	access.UserID = strings.TrimSpace(access.UserID)
	if access.WorkspaceID == "" || access.DesignID == "" || access.UserID == "" {
		return domain.DesignAccess{}, errors.New("workspace id, design id, and user id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	design, ok := r.designs[access.DesignID]
	if !ok || design.WorkspaceID != access.WorkspaceID {
		return domain.DesignAccess{}, errors.New("design not found")
	}
	if _, err := r.getUserLocked(access.UserID); err != nil {
		return domain.DesignAccess{}, err
	}
	now := r.clock().UTC()
	key := accessKey(access.WorkspaceID, access.DesignID, access.UserID)
	if existing, ok := r.designACL[key]; ok {
		access.CreatedAt = existing.CreatedAt
	} else {
		access.CreatedAt = now
	}
	access.UpdatedAt = now
	r.designACL[key] = access
	return access, nil
}

func (r *MemoryRepository) RevokeDesignAccess(ctx context.Context, workspaceID string, designID string, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.designACL, accessKey(workspaceID, designID, userID))
	return nil
}

func (r *MemoryRepository) ListDesignGroupAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignGroupAccess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	access := []domain.DesignGroupAccess{}
	for _, entry := range r.designGACL {
		if entry.WorkspaceID == workspaceID && entry.DesignID == designID {
			access = append(access, entry)
		}
	}
	sort.Slice(access, func(i, j int) bool { return access[i].GroupID < access[j].GroupID })
	return access, nil
}

func (r *MemoryRepository) GrantDesignGroupAccess(ctx context.Context, access domain.DesignGroupAccess) (domain.DesignGroupAccess, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignGroupAccess{}, err
	}
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.DesignID = strings.TrimSpace(access.DesignID)
	access.GroupID = strings.TrimSpace(access.GroupID)
	if access.WorkspaceID == "" || access.DesignID == "" || access.GroupID == "" {
		return domain.DesignGroupAccess{}, errors.New("workspace id, design id, and group id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	design, ok := r.designs[access.DesignID]
	if !ok || design.WorkspaceID != access.WorkspaceID {
		return domain.DesignGroupAccess{}, errors.New("design not found")
	}
	if _, ok := r.accessGroups[access.GroupID]; !ok {
		return domain.DesignGroupAccess{}, errors.New("group not found")
	}
	now := r.clock().UTC()
	key := accessKey(access.WorkspaceID, access.DesignID, access.GroupID)
	if existing, ok := r.designGACL[key]; ok {
		access.CreatedAt = existing.CreatedAt
	} else {
		access.CreatedAt = now
	}
	access.UpdatedAt = now
	r.designGACL[key] = access
	return access, nil
}

func (r *MemoryRepository) RevokeDesignGroupAccess(ctx context.Context, workspaceID string, designID string, groupID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.designGACL, accessKey(workspaceID, designID, groupID))
	return nil
}

func (r *MemoryRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.User, PageInfo, error) {
		return r.ListUsersPage(ctx, options)
	})
}

func (r *MemoryRepository) ListUsersPage(ctx context.Context, options PageOptions) ([]domain.User, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	query := strings.ToLower(options.Query)
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		if query != "" && !strings.Contains(strings.ToLower(user.DisplayName+" "+user.Email+" "+user.Role), query) {
			continue
		}
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	page, info := PageFromSlice(users, options)
	return page, info, nil
}

func (r *MemoryRepository) ListCatalogAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.CatalogAsset, PageInfo, error) {
		options.Query = query
		return r.ListCatalogAssetsPage(ctx, options)
	})
}

func (r *MemoryRepository) ListCatalogAssetsPage(ctx context.Context, options PageOptions) ([]domain.CatalogAsset, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	normalizedQuery := normalizedCatalogName(options.Query)
	r.mu.RLock()
	defer r.mu.RUnlock()

	assets := make([]domain.CatalogAsset, 0, len(r.catalogAssets))
	for _, asset := range r.catalogAssets {
		if normalizedQuery != "" && !strings.Contains(asset.NormalizedName, normalizedQuery) && !strings.Contains(strings.ToLower(asset.Name), strings.ToLower(strings.TrimSpace(options.Query))) {
			continue
		}
		asset.UsedInDesignCount = r.catalogAssetUsageCountLocked(asset.ID)
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool {
		if normalizedQuery != "" {
			leftExact := assets[i].NormalizedName == normalizedQuery
			rightExact := assets[j].NormalizedName == normalizedQuery
			if leftExact != rightExact {
				return leftExact
			}
		}
		return assets[i].Name < assets[j].Name
	})
	page, info := PageFromSlice(assets, options)
	return page, info, nil
}

func (r *MemoryRepository) CreateCatalogAsset(ctx context.Context, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	if err := ctx.Err(); err != nil {
		return domain.CatalogAsset{}, err
	}
	asset.Name = strings.TrimSpace(asset.Name)
	if asset.Name == "" {
		return domain.CatalogAsset{}, errors.New("catalog asset name is required")
	}
	asset.NormalizedName = normalizedCatalogName(asset.Name)
	if asset.Type == "" {
		asset.Type = "compute.service"
	}
	if asset.Criticality == "" {
		asset.Criticality = "medium"
	}
	if len(asset.Metadata) == 0 {
		asset.Metadata = json.RawMessage(`{}`)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.catalogAssets {
		if existing.NormalizedName == asset.NormalizedName {
			return domain.CatalogAsset{}, errors.New("catalog asset already exists")
		}
	}
	now := r.clock().UTC()
	asset.ID = fmt.Sprintf("asset_%d", now.UnixNano())
	if strings.TrimSpace(asset.CreatedBy) == "" {
		asset.CreatedBy = r.firstUserIDLocked()
	}
	asset.CreatedAt = now
	asset.UpdatedAt = now
	r.catalogAssets[asset.ID] = asset
	return asset, nil
}

func (r *MemoryRepository) UpdateCatalogAsset(ctx context.Context, assetID string, asset domain.CatalogAsset) (domain.CatalogAsset, error) {
	if err := ctx.Err(); err != nil {
		return domain.CatalogAsset{}, err
	}
	asset.Name = strings.TrimSpace(asset.Name)
	if asset.Name == "" {
		return domain.CatalogAsset{}, errors.New("catalog asset name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.catalogAssets[assetID]
	if !ok {
		return domain.CatalogAsset{}, errors.New("catalog asset not found")
	}
	normalized := normalizedCatalogName(asset.Name)
	for _, other := range r.catalogAssets {
		if other.ID != assetID && other.NormalizedName == normalized {
			return domain.CatalogAsset{}, errors.New("catalog asset already exists")
		}
	}
	existing.Name = asset.Name
	existing.NormalizedName = normalized
	existing.Type = strings.TrimSpace(asset.Type)
	if existing.Type == "" {
		existing.Type = "compute.service"
	}
	existing.Owner = strings.TrimSpace(asset.Owner)
	existing.Description = strings.TrimSpace(asset.Description)
	existing.Criticality = strings.TrimSpace(asset.Criticality)
	if existing.Criticality == "" {
		existing.Criticality = "medium"
	}
	existing.Tags = normalizedTags(asset.Tags)
	if len(asset.Metadata) > 0 {
		existing.Metadata = asset.Metadata
	}
	existing.UpdatedAt = r.clock().UTC()
	r.catalogAssets[assetID] = existing
	existing.UsedInDesignCount = r.catalogAssetUsageCountLocked(assetID)
	return existing, nil
}

func (r *MemoryRepository) DeleteCatalogAsset(ctx context.Context, assetID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.catalogAssets[assetID]; !ok {
		return errors.New("catalog asset not found")
	}
	if r.catalogAssetUsageCountLocked(assetID) > 0 {
		return errors.New("catalog asset is linked to one or more designs")
	}
	delete(r.catalogAssets, assetID)
	return nil
}

func normalizedCatalogName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(name)), " "))
}

func normalizedTags(tags []string) []string {
	seen := map[string]struct{}{}
	normalized := []string{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}

func (r *MemoryRepository) catalogAssetUsageCountLocked(assetID string) int {
	designIDs := map[string]struct{}{}
	needle := fmt.Sprintf(`"assetId":"%s"`, assetID)
	for _, design := range r.designs {
		if strings.Contains(string(design.Document), needle) {
			designIDs[design.ID] = struct{}{}
		}
	}
	return len(designIDs)
}

func (r *MemoryRepository) GetUser(ctx context.Context, userID string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getUserLocked(userID)
}

func (r *MemoryRepository) AuthenticateUser(ctx context.Context, email string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	email = strings.ToLower(strings.TrimSpace(email))
	for id, user := range r.users {
		if strings.EqualFold(user.Email, email) && user.Status != "disabled" && user.PasswordSet && verifyPassword(password, user.PasswordHash) {
			user.LastSeenAt = r.clock().UTC()
			r.users[id] = user
			return user, nil
		}
	}
	return domain.User{}, errors.New("invalid email or password")
}

func (r *MemoryRepository) CreateSession(ctx context.Context, userID string, expiresAt time.Time) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.getUserLocked(userID); err != nil {
		return "", err
	}
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	now := r.clock().UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(12 * time.Hour)
	}
	r.sessions[tokenHash] = memorySession{
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  expiresAt.UTC(),
	}
	return token, nil
}

func (r *MemoryRepository) GetUserBySessionToken(ctx context.Context, token string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	tokenHash := sessionTokenHash(token)
	session, ok := r.sessions[tokenHash]
	if !ok {
		return domain.User{}, errors.New("session not found")
	}
	now := r.clock().UTC()
	if !session.ExpiresAt.After(now) {
		delete(r.sessions, tokenHash)
		return domain.User{}, errors.New("session expired")
	}
	session.LastSeenAt = now
	r.sessions[tokenHash] = session
	return r.getUserLocked(session.UserID)
}

func (r *MemoryRepository) DeleteSession(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionTokenHash(token))
	return nil
}

func (r *MemoryRepository) PasswordSetupRequired(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.users) == 0 {
		return false, nil
	}
	for _, user := range r.users {
		if user.PasswordSet {
			return false, nil
		}
	}
	return true, nil
}

func (r *MemoryRepository) SetInitialAdminPassword(ctx context.Context, email string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	requiresSetup := len(r.users) > 0
	for _, user := range r.users {
		if user.PasswordSet {
			requiresSetup = false
			break
		}
	}
	if !requiresSetup {
		return domain.User{}, errors.New("password setup is not available")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	for id, user := range r.users {
		if strings.EqualFold(user.Email, email) && user.Role == "admin" && user.Status != "disabled" {
			user.PasswordHash = passwordHash
			user.PasswordSet = true
			user.UpdatedAt = r.clock().UTC()
			r.users[id] = user
			return user, nil
		}
	}
	return domain.User{}, errors.New("admin user not found")
}

func (r *MemoryRepository) CreateFirstAdmin(ctx context.Context, displayName string, email string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.users) > 0 {
		return domain.User{}, errors.New("organization already has users")
	}
	user, err := r.createUserLocked(displayName, email, "admin", password)
	if err != nil {
		return domain.User{}, err
	}
	r.claimLegacyGuestDataLocked(user.ID)
	return user, nil
}

func (r *MemoryRepository) CreateUser(ctx context.Context, displayName string, email string, role string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return domain.User{}, errors.New("email is required")
	}
	if r.emailExistsLocked(email, "") {
		return domain.User{}, errors.New("a user with this email already exists")
	}
	if len(r.users) == 0 {
		return domain.User{}, errors.New("create the first admin before inviting users")
	}
	return r.createUserLocked(displayName, email, role, password)
}

func (r *MemoryRepository) UpdateUser(ctx context.Context, userID string, displayName string, email string, role string, status string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[userID]
	if !ok {
		return domain.User{}, errors.New("user not found")
	}
	if strings.TrimSpace(displayName) != "" {
		user.DisplayName = strings.TrimSpace(displayName)
	}
	if strings.TrimSpace(email) != "" {
		nextEmail := strings.ToLower(strings.TrimSpace(email))
		if r.emailExistsLocked(nextEmail, user.ID) {
			return domain.User{}, errors.New("a user with this email already exists")
		}
		user.Email = nextEmail
	}
	if strings.TrimSpace(role) != "" {
		user.Role = normalizedUserRole(role)
	}
	if strings.TrimSpace(status) != "" {
		user.Status = normalizedUserStatus(status)
	}
	if strings.TrimSpace(password) != "" {
		passwordHash, err := hashPassword(password)
		if err != nil {
			return domain.User{}, err
		}
		user.PasswordHash = passwordHash
		user.PasswordSet = true
	}
	user.UpdatedAt = r.clock().UTC()
	r.users[user.ID] = user
	return user, nil
}

func (r *MemoryRepository) CreatePasswordResetToken(ctx context.Context, userID string, expiresAt time.Time) (string, domain.PasswordResetToken, error) {
	if err := ctx.Err(); err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	user, err := r.getUserLocked(userID)
	if err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	if user.Status == "disabled" {
		return "", domain.PasswordResetToken{}, errors.New("user is disabled")
	}
	token, tokenHash, err := newPasswordResetToken()
	if err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	now := r.clock().UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(time.Hour)
	}
	reset := domain.PasswordResetToken{
		Token:     tokenHash,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: expiresAt.UTC(),
	}
	r.passwordReset[tokenHash] = reset
	return token, reset, nil
}

func (r *MemoryRepository) ResetPasswordWithToken(ctx context.Context, token string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	tokenHash := sessionTokenHash(token)
	reset, ok := r.passwordReset[tokenHash]
	now := r.clock().UTC()
	if !ok || reset.UsedAt != nil || !reset.ExpiresAt.After(now) {
		delete(r.passwordReset, tokenHash)
		return domain.User{}, errors.New("password reset link is invalid or expired")
	}
	user, ok := r.users[reset.UserID]
	if !ok || user.Status == "disabled" {
		return domain.User{}, errors.New("password reset link is invalid or expired")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	user.PasswordHash = passwordHash
	user.PasswordSet = true
	user.UpdatedAt = now
	r.users[user.ID] = user
	reset.UsedAt = &now
	r.passwordReset[tokenHash] = reset
	return user, nil
}

func (r *MemoryRepository) DeleteUser(ctx context.Context, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	if user.Role == "admin" && r.adminCountLocked() <= 1 {
		return errors.New("at least one admin is required")
	}
	delete(r.users, userID)
	return nil
}

func (r *MemoryRepository) GetSignInConfig(ctx context.Context) (domain.SignInConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.SignInConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.signInConfig, nil
}

func (r *MemoryRepository) UpdateSignInConfig(ctx context.Context, config domain.SignInConfig, clientSecret string) (domain.SignInConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.SignInConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.signInConfig
	current.LocalPasswordEnabled = config.LocalPasswordEnabled
	current.SSOEnabled = config.SSOEnabled
	current.Provider = strings.TrimSpace(config.Provider)
	current.OktaDomain = strings.TrimSpace(config.OktaDomain)
	current.Issuer = strings.TrimSpace(config.Issuer)
	current.ClientID = strings.TrimSpace(config.ClientID)
	current.RedirectURI = strings.TrimSpace(config.RedirectURI)
	current.PostLogoutRedirectURI = strings.TrimSpace(config.PostLogoutRedirectURI)
	current.Scopes = normalizedScopes(config.Scopes)
	current.GroupsClaim = strings.TrimSpace(config.GroupsClaim)
	current.AdminGroup = strings.TrimSpace(config.AdminGroup)
	current.ReviewerGroup = strings.TrimSpace(config.ReviewerGroup)
	current.JITProvisioning = config.JITProvisioning
	if strings.TrimSpace(clientSecret) != "" {
		current.ClientSecretSet = true
	}
	current.UpdatedAt = r.clock().UTC()
	r.signInConfig = current
	return current, nil
}

func (r *MemoryRepository) GetAIProviderConfig(ctx context.Context) (domain.AIProviderConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIProviderConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return sanitizeAIProviderConfig(r.aiConfig), nil
}

func (r *MemoryRepository) GetAIProviderConfigWithSecret(ctx context.Context) (domain.AIProviderConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIProviderConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.aiConfig, nil
}

func (r *MemoryRepository) UpdateAIProviderConfig(ctx context.Context, config domain.AIProviderConfig, apiKey string) (domain.AIProviderConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIProviderConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.aiConfig
	current.Enabled = config.Enabled
	current.Provider = normalizedAIProvider(config.Provider)
	current.Model = strings.TrimSpace(config.Model)
	current.BaseURL = strings.TrimSpace(config.BaseURL)
	if strings.TrimSpace(apiKey) != "" {
		current.APIKey = strings.TrimSpace(apiKey)
		current.APIKeySet = true
		current.VerifiedAt = r.clock().UTC()
	}
	if current.Model == "" {
		current.Model = defaultAIModel(current.Provider)
	}
	current.UpdatedAt = r.clock().UTC()
	r.aiConfig = current
	return sanitizeAIProviderConfig(current), nil
}

func (r *MemoryRepository) GetMCPConfig(ctx context.Context) (domain.MCPConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.MCPConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mcpConfig, nil
}

func (r *MemoryRepository) UpdateMCPConfig(ctx context.Context, config domain.MCPConfig) (domain.MCPConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.MCPConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.mcpConfig
	current.Enabled = config.Enabled
	current.EndpointPath = normalizedMCPPath(config.EndpointPath)
	current.ReadCatalog = config.ReadCatalog
	current.ReadDesigns = config.ReadDesigns
	current.CreateDraftDesign = config.CreateDraftDesign
	current.RunAnalysis = config.RunAnalysis
	current.FetchImpactReport = config.FetchImpactReport
	current.RequireAdminConsent = config.RequireAdminConsent
	current.UpdatedAt = r.clock().UTC()
	r.mcpConfig = current
	return current, nil
}

func (r *MemoryRepository) GetTelemetryIntegrationConfig(ctx context.Context) (domain.TelemetryIntegrationConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return sanitizeTelemetryIntegrationConfig(r.telemetryConfig), nil
}

func (r *MemoryRepository) GetTelemetryIntegrationConfigWithSecret(ctx context.Context) (domain.TelemetryIntegrationConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.telemetryConfig, nil
}

func (r *MemoryRepository) UpdateTelemetryIntegrationConfig(ctx context.Context, config domain.TelemetryIntegrationConfig, secret string) (domain.TelemetryIntegrationConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.telemetryConfig
	next, err := normalizedTelemetryIntegrationConfig(current, config, secret, r.clock().UTC())
	if err != nil {
		return domain.TelemetryIntegrationConfig{}, err
	}
	r.telemetryConfig = next
	return sanitizeTelemetryIntegrationConfig(next), nil
}

func (r *MemoryRepository) ListDesignComments(ctx context.Context, workspaceID string, designID string) ([]domain.DesignComment, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.DesignComment, PageInfo, error) {
		return r.ListDesignCommentsPage(ctx, workspaceID, designID, options)
	})
}

func (r *MemoryRepository) ListDesignCommentsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignComment, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	if workspaceID == "" || designID == "" {
		return nil, PageInfo{}, errors.New("workspace id and design id are required")
	}
	options = NormalizePageOptions(options)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if design, ok := r.designs[designID]; !ok || design.WorkspaceID != workspaceID {
		return nil, PageInfo{}, errors.New("design not found")
	}
	comments := make([]domain.DesignComment, 0, len(r.comments))
	for _, comment := range r.comments {
		if comment.WorkspaceID == workspaceID && comment.DesignID == designID {
			comments = append(comments, comment)
		}
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].CreatedAt.After(comments[j].CreatedAt)
	})
	page, info := PageFromSlice(comments, options)
	return page, info, nil
}

func (r *MemoryRepository) CreateDesignComment(ctx context.Context, workspaceID string, designID string, authorID string, body string, componentID string, connectorID string) (domain.DesignComment, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignComment{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return domain.DesignComment{}, errors.New("comment body is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return domain.DesignComment{}, errors.New("design not found")
	}
	now := r.clock().UTC()
	if authorID == "" {
		authorID = r.firstUserIDLocked()
	}
	author, err := r.getUserLocked(authorID)
	if err != nil {
		return domain.DesignComment{}, err
	}
	comment := domain.DesignComment{
		ID:          fmt.Sprintf("comment_%d", now.UnixNano()),
		WorkspaceID: workspaceID,
		DesignID:    designID,
		AuthorID:    authorID,
		Body:        body,
		ComponentID: strings.TrimSpace(componentID),
		ConnectorID: strings.TrimSpace(connectorID),
		CreatedAt:   now,
	}
	r.comments[comment.ID] = comment
	if design.CreatedBy != "" && design.CreatedBy != authorID {
		r.addNotificationLocked(design.CreatedBy, workspaceID, designID, "comment_added", "New comment", author.DisplayName+" commented on "+design.Name, now)
	}
	return comment, nil
}

func (r *MemoryRepository) ListDesignReviewRequests(ctx context.Context, workspaceID string, designID string) ([]domain.DesignReviewRequest, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.DesignReviewRequest, PageInfo, error) {
		return r.ListDesignReviewRequestsPage(ctx, workspaceID, designID, options)
	})
}

func (r *MemoryRepository) ListDesignReviewRequestsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignReviewRequest, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	if workspaceID == "" || designID == "" {
		return nil, PageInfo{}, errors.New("workspace id and design id are required")
	}
	options = NormalizePageOptions(options)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if design, ok := r.designs[designID]; !ok || design.WorkspaceID != workspaceID {
		return nil, PageInfo{}, errors.New("design not found")
	}
	reviews := make([]domain.DesignReviewRequest, 0, len(r.reviews))
	for _, review := range r.reviews {
		if review.WorkspaceID == workspaceID && review.DesignID == designID {
			reviews = append(reviews, review)
		}
	}
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].UpdatedAt.After(reviews[j].UpdatedAt)
	})
	page, info := PageFromSlice(reviews, options)
	return page, info, nil
}

func (r *MemoryRepository) CreateDesignReviewRequests(ctx context.Context, workspaceID string, designID string, versionID string, requestedBy string, reviewerIDs []string, message string) ([]domain.DesignReviewRequest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reviewerIDs = normalizeReviewerIDs(reviewerIDs)
	if len(reviewerIDs) == 0 {
		return nil, errors.New("at least one reviewer id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return nil, errors.New("design not found")
	}
	version, err := r.resolveDesignVersionLocked(workspaceID, designID, versionID, design.VersionNumber)
	if err != nil {
		return nil, err
	}
	now := r.clock().UTC()
	if requestedBy == "" {
		requestedBy = r.firstUserIDLocked()
	}
	requester, err := r.getUserLocked(requestedBy)
	if err != nil {
		return nil, err
	}
	reviews := make([]domain.DesignReviewRequest, 0, len(reviewerIDs))
	for index, reviewerID := range reviewerIDs {
		reviewer, err := r.getUserLocked(reviewerID)
		if err != nil {
			return nil, err
		}
		if existing, ok := r.findReviewForVersionReviewerLocked(workspaceID, designID, version.ID, reviewer.ID); ok {
			reviews = append(reviews, existing)
			continue
		}
		review := domain.DesignReviewRequest{
			ID:            fmt.Sprintf("review_%d_%d", now.UnixNano(), index),
			WorkspaceID:   workspaceID,
			DesignID:      designID,
			VersionID:     version.ID,
			VersionNumber: version.VersionNumber,
			RequestedBy:   requestedBy,
			ReviewerID:    reviewer.ID,
			Status:        "requested",
			Message:       strings.TrimSpace(message),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		r.reviews[review.ID] = review
		r.addNotificationLocked(reviewer.ID, workspaceID, designID, "review_requested", "Review requested", requester.DisplayName+" requested your review on "+design.Name, now)
		reviews = append(reviews, review)
	}
	nextStatus := lifecycle.StatusAfterReviewRequest(version.Status)
	if nextStatus != version.Status {
		if _, err := r.updateDesignVersionStatusLocked(workspaceID, designID, version.ID, nextStatus, now); err != nil {
			return nil, err
		}
	}
	return reviews, nil
}

func (r *MemoryRepository) UpdateDesignReviewRequest(ctx context.Context, workspaceID string, designID string, reviewID string, status string, summary string) (domain.DesignReviewRequest, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignReviewRequest{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	review, ok := r.reviews[reviewID]
	if !ok || review.WorkspaceID != workspaceID || review.DesignID != designID {
		return domain.DesignReviewRequest{}, errors.New("review request not found")
	}
	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return domain.DesignReviewRequest{}, errors.New("design not found")
	}
	status = normalizedReviewStatus(status, review.Status)
	reviewer, err := r.getUserLocked(review.ReviewerID)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	now := r.clock().UTC()
	review.Status = status
	review.Summary = strings.TrimSpace(summary)
	review.UpdatedAt = now
	if status == "approved" || status == "changes_requested" {
		review.CompletedAt = &now
	}
	r.reviews[review.ID] = review
	if status == "approved" && review.VersionID != "" {
		r.markVersionReviewedIfApprovedLocked(workspaceID, designID, review.VersionID, now)
	}
	if design.CreatedBy != "" && design.CreatedBy != review.ReviewerID {
		r.addNotificationLocked(design.CreatedBy, workspaceID, designID, "review_updated", "Review updated", reviewer.DisplayName+" marked review as "+strings.ReplaceAll(status, "_", " "), now)
	}
	return review, nil
}

func (r *MemoryRepository) ListNotifications(ctx context.Context, userID string) ([]domain.Notification, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if userID == "" {
		userID = r.firstUserIDLocked()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	notifications := make([]domain.Notification, 0, len(r.notifications))
	for _, notification := range r.notifications {
		if notification.UserID == userID {
			notifications = append(notifications, notification)
		}
	}
	sort.Slice(notifications, func(i, j int) bool {
		return notifications[i].CreatedAt.After(notifications[j].CreatedAt)
	})
	return notifications, nil
}

func (r *MemoryRepository) MarkNotificationRead(ctx context.Context, userID string, notificationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if userID == "" {
		userID = r.firstUserIDLocked()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	notification, ok := r.notifications[notificationID]
	if !ok || notification.UserID != userID {
		return errors.New("notification not found")
	}
	notification.Read = true
	r.notifications[notification.ID] = notification
	return nil
}

func (r *MemoryRepository) addNotificationLocked(userID string, workspaceID string, designID string, notificationType string, title string, body string, now time.Time) {
	if strings.TrimSpace(userID) == "" {
		return
	}
	notification := domain.Notification{
		ID:          fmt.Sprintf("notification_%d_%s", now.UnixNano(), userID),
		UserID:      userID,
		WorkspaceID: workspaceID,
		DesignID:    designID,
		Type:        notificationType,
		Title:       title,
		Body:        body,
		Read:        false,
		CreatedAt:   now,
	}
	r.notifications[notification.ID] = notification
}

func (r *MemoryRepository) createUserLocked(displayName string, email string, role string, password string) (domain.User, error) {
	displayName = strings.TrimSpace(displayName)
	email = strings.ToLower(strings.TrimSpace(email))
	if displayName == "" {
		return domain.User{}, errors.New("display name is required")
	}
	if email == "" {
		return domain.User{}, errors.New("email is required")
	}
	for _, user := range r.users {
		if strings.EqualFold(user.Email, email) {
			return domain.User{}, errors.New("a user with this email already exists")
		}
	}
	passwordHash := ""
	if strings.TrimSpace(password) != "" {
		var err error
		passwordHash, err = hashPassword(password)
		if err != nil {
			return domain.User{}, err
		}
	}
	now := r.clock().UTC()
	user := domain.User{
		ID:           fmt.Sprintf("user_%d_%d", now.UnixNano(), len(r.users)+1),
		DisplayName:  displayName,
		Email:        email,
		PasswordSet:  passwordHash != "",
		PasswordHash: passwordHash,
		Role:         normalizedUserRole(role),
		Status:       "active",
		LastSeenAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r.users[user.ID] = user
	return user, nil
}

func (r *MemoryRepository) emailExistsLocked(email string, exceptUserID string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, user := range r.users {
		if user.ID != exceptUserID && strings.EqualFold(user.Email, email) {
			return true
		}
	}
	return false
}

func (r *MemoryRepository) getUserLocked(userID string) (domain.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = r.firstUserIDLocked()
	}
	user, ok := r.users[userID]
	if !ok || user.Status == "disabled" {
		return domain.User{}, errors.New("user not found")
	}
	return user, nil
}

func (r *MemoryRepository) firstUserIDLocked() string {
	var first domain.User
	for _, user := range r.users {
		if user.Status == "disabled" {
			continue
		}
		if first.ID == "" || user.CreatedAt.Before(first.CreatedAt) {
			first = user
		}
	}
	return first.ID
}

func (r *MemoryRepository) adminCountLocked() int {
	count := 0
	for _, user := range r.users {
		if user.Role == "admin" && user.Status != "disabled" {
			count++
		}
	}
	return count
}

func (r *MemoryRepository) claimLegacyGuestDataLocked(userID string) {
	for id, design := range r.designs {
		if design.CreatedBy == "" || design.CreatedBy == "guest-user" {
			design.CreatedBy = userID
			r.designs[id] = design
		}
	}
	for designID, versions := range r.versions {
		for index, version := range versions {
			if version.CreatedBy == "" || version.CreatedBy == "guest-user" {
				version.CreatedBy = userID
				versions[index] = version
			}
		}
		r.versions[designID] = versions
	}
	for id, doc := range r.docs {
		if doc.CreatedBy == "" || doc.CreatedBy == "guest-user" {
			doc.CreatedBy = userID
			r.docs[id] = doc
		}
	}
	for id, comment := range r.comments {
		if comment.AuthorID == "" || comment.AuthorID == "guest-user" {
			comment.AuthorID = userID
			r.comments[id] = comment
		}
	}
	for id, notification := range r.notifications {
		if notification.UserID == "" || notification.UserID == "guest-user" {
			notification.UserID = userID
			r.notifications[id] = notification
		}
	}
}

func defaultSignInConfig(now time.Time) domain.SignInConfig {
	return domain.SignInConfig{
		LocalPasswordEnabled: true,
		SSOEnabled:           false,
		Provider:             "okta",
		Scopes:               "openid profile email groups",
		GroupsClaim:          "groups",
		JITProvisioning:      true,
		UpdatedAt:            now,
	}
}

func defaultAIProviderConfig(now time.Time) domain.AIProviderConfig {
	return domain.AIProviderConfig{
		Enabled:   false,
		Provider:  "openai",
		Model:     "gpt-4.1",
		BaseURL:   "",
		UpdatedAt: now,
	}
}

func defaultMCPConfig(now time.Time) domain.MCPConfig {
	return domain.MCPConfig{
		Enabled:             false,
		EndpointPath:        "/mcp",
		ReadCatalog:         true,
		ReadDesigns:         true,
		CreateDraftDesign:   true,
		RunAnalysis:         true,
		FetchImpactReport:   false,
		RequireAdminConsent: true,
		UpdatedAt:           now,
	}
}

func defaultTelemetryIntegrationConfig(now time.Time) domain.TelemetryIntegrationConfig {
	return domain.TelemetryIntegrationConfig{
		Enabled:             false,
		Provider:            "prometheus",
		DisplayName:         "Prometheus Service Graph",
		AuthMode:            "none",
		QueryWindow:         "5m",
		RequestTotalMetric:  "traces_service_graph_request_total",
		RequestFailedMetric: "traces_service_graph_request_failed_total",
		ServerLatencyMetric: "traces_service_graph_request_server_seconds_bucket",
		ClientLatencyMetric: "traces_service_graph_request_client_seconds_bucket",
		Filters: domain.TelemetryIntegrationFilter{
			IncludeExternal:        true,
			IncludeDatabaseClients: true,
		},
		UpdatedAt: now,
	}
}

func sanitizeAIProviderConfig(config domain.AIProviderConfig) domain.AIProviderConfig {
	config.APIKey = ""
	return config
}

func sanitizeTelemetryIntegrationConfig(config domain.TelemetryIntegrationConfig) domain.TelemetryIntegrationConfig {
	config.Secret = ""
	return config
}

func normalizedTelemetryIntegrationConfig(current domain.TelemetryIntegrationConfig, input domain.TelemetryIntegrationConfig, secret string, now time.Time) (domain.TelemetryIntegrationConfig, error) {
	next := current
	next.Enabled = input.Enabled
	next.Provider = normalizedTelemetryProvider(input.Provider)
	next.DisplayName = strings.TrimSpace(input.DisplayName)
	if next.DisplayName == "" {
		next.DisplayName = "Prometheus Service Graph"
	}
	next.BaseURL = strings.TrimSpace(input.BaseURL)
	next.AuthMode = normalizedTelemetryAuthMode(input.AuthMode)
	next.CustomHeaderName = strings.TrimSpace(input.CustomHeaderName)
	next.QueryWindow = strings.TrimSpace(input.QueryWindow)
	if next.QueryWindow == "" {
		next.QueryWindow = "5m"
	}
	next.RequestTotalMetric = strings.TrimSpace(input.RequestTotalMetric)
	if next.RequestTotalMetric == "" {
		next.RequestTotalMetric = "traces_service_graph_request_total"
	}
	next.RequestFailedMetric = strings.TrimSpace(input.RequestFailedMetric)
	if next.RequestFailedMetric == "" {
		next.RequestFailedMetric = "traces_service_graph_request_failed_total"
	}
	next.ServerLatencyMetric = strings.TrimSpace(input.ServerLatencyMetric)
	if next.ServerLatencyMetric == "" {
		next.ServerLatencyMetric = "traces_service_graph_request_server_seconds_bucket"
	}
	next.ClientLatencyMetric = strings.TrimSpace(input.ClientLatencyMetric)
	if next.ClientLatencyMetric == "" {
		next.ClientLatencyMetric = "traces_service_graph_request_client_seconds_bucket"
	}
	next.Filters = normalizedTelemetryFilters(input.Filters)
	if strings.TrimSpace(secret) != "" {
		next.Secret = strings.TrimSpace(secret)
		next.SecretSet = true
	} else {
		next.SecretSet = current.SecretSet && strings.TrimSpace(current.Secret) != ""
	}
	if next.Enabled && next.BaseURL == "" {
		return domain.TelemetryIntegrationConfig{}, errors.New("prometheus URL is required when telemetry integration is enabled")
	}
	if next.AuthMode == "custom_header" && next.CustomHeaderName == "" {
		return domain.TelemetryIntegrationConfig{}, errors.New("custom header name is required for custom header authentication")
	}
	next.UpdatedAt = now
	return next, nil
}

func normalizedTelemetryFilters(filters domain.TelemetryIntegrationFilter) domain.TelemetryIntegrationFilter {
	return domain.TelemetryIntegrationFilter{
		Namespaces:             normalizedStringList(filters.Namespaces),
		Services:               normalizedStringList(filters.Services),
		ExcludeServices:        normalizedStringList(filters.ExcludeServices),
		ExcludeEndpoints:       normalizedStringList(filters.ExcludeEndpoints),
		RequiredLabels:         normalizedStringMap(filters.RequiredLabels),
		MinimumRequestsPerSec:  filters.MinimumRequestsPerSec,
		IncludeExternal:        filters.IncludeExternal,
		IncludeDatabaseClients: filters.IncludeDatabaseClients,
	}
}

func normalizedStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func normalizedStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cleaned := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			cleaned[key] = value
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func normalizedTelemetryProvider(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "prometheus":
		return "prometheus"
	default:
		return "prometheus"
	}
}

func normalizedTelemetryAuthMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "bearer":
		return "bearer"
	case "basic":
		return "basic"
	case "custom_header":
		return "custom_header"
	default:
		return "none"
	}
}

func normalizedMCPPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func accessKey(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		cleaned = append(cleaned, strings.TrimSpace(part))
	}
	return strings.Join(cleaned, "\x00")
}

func (r *MemoryRepository) accessGroupMemberCountLocked(groupID string) int {
	count := 0
	for _, member := range r.groupMembers {
		if member.GroupID == groupID {
			count++
		}
	}
	return count
}

func normalizedAIProvider(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "anthropic":
		return "anthropic"
	case "openrouter":
		return "openrouter"
	case "custom":
		return "custom"
	default:
		return "openai"
	}
}

func defaultAIModel(provider string) string {
	switch normalizedAIProvider(provider) {
	case "anthropic":
		return "claude-3-5-sonnet-latest"
	case "openrouter":
		return "openai/gpt-4.1"
	default:
		return "gpt-4.1"
	}
}

func normalizedDocTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled doc"
	}
	return title
}

func normalizedUserRole(role string) string {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "admin":
		return "admin"
	case "architect":
		return "architect"
	case "reviewer":
		return "reviewer"
	default:
		return "member"
	}
}

func normalizedUserStatus(status string) string {
	if strings.TrimSpace(strings.ToLower(status)) == "disabled" {
		return "disabled"
	}
	return "active"
}

func normalizedScopes(scopes string) string {
	scopes = strings.Join(strings.Fields(scopes), " ")
	if scopes == "" {
		return "openid profile email groups"
	}
	return scopes
}

func normalizedDocFormat(format string) string {
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		return "html"
	}
	return format
}

func normalizedReviewStatus(status string, fallback string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "requested", "pending":
		return "requested"
	case "approved":
		return "approved"
	case "changes_requested":
		return "changes_requested"
	default:
		if fallback == "" {
			return "requested"
		}
		return fallback
	}
}

func normalizeReviewerIDs(reviewerIDs []string) []string {
	seen := make(map[string]struct{}, len(reviewerIDs))
	normalized := make([]string, 0, len(reviewerIDs))
	for _, reviewerID := range reviewerIDs {
		reviewerID = strings.TrimSpace(reviewerID)
		if reviewerID == "" {
			continue
		}
		if _, ok := seen[reviewerID]; ok {
			continue
		}
		seen[reviewerID] = struct{}{}
		normalized = append(normalized, reviewerID)
	}
	return normalized
}

func (r *MemoryRepository) resolveDesignVersionLocked(workspaceID string, designID string, versionID string, fallbackVersionNumber int) (domain.DesignVersion, error) {
	versionID = strings.TrimSpace(versionID)
	for _, version := range r.versions[designID] {
		if version.WorkspaceID != workspaceID {
			continue
		}
		if versionID != "" && version.ID == versionID {
			return version, nil
		}
		if versionID == "" && fallbackVersionNumber > 0 && version.VersionNumber == fallbackVersionNumber {
			return version, nil
		}
	}
	if versionID == "" {
		return domain.DesignVersion{}, errors.New("design version is required before requesting review")
	}
	return domain.DesignVersion{}, errors.New("version not found")
}

func (r *MemoryRepository) findReviewForVersionReviewerLocked(workspaceID string, designID string, versionID string, reviewerID string) (domain.DesignReviewRequest, bool) {
	for _, review := range r.reviews {
		if review.WorkspaceID == workspaceID && review.DesignID == designID && review.VersionID == versionID && review.ReviewerID == reviewerID {
			return review, true
		}
	}
	return domain.DesignReviewRequest{}, false
}

func (r *MemoryRepository) markVersionReviewedIfApprovedLocked(workspaceID string, designID string, versionID string, now time.Time) {
	reviewCount := 0
	for _, review := range r.reviews {
		if review.WorkspaceID != workspaceID || review.DesignID != designID || review.VersionID != versionID {
			continue
		}
		reviewCount++
		if review.Status != "approved" {
			return
		}
	}
	if reviewCount == 0 {
		return
	}
	nextStatus := lifecycle.StatusAfterAllReviewsApproved(lifecycle.VersionPendingReview, reviewCount, 0)
	for index := range r.versions[designID] {
		version := &r.versions[designID][index]
		if version.ID == versionID && version.WorkspaceID == workspaceID && version.Status == lifecycle.VersionPendingReview {
			version.Status = nextStatus
			version.UpdatedAt = now
			return
		}
	}
}

func initialDesignDocument(document []byte, designID string, name string, now time.Time) (json.RawMessage, error) {
	if len(document) > 0 {
		if !json.Valid(document) {
			return nil, errors.New("design document must be valid JSON")
		}
		if strings.TrimSpace(string(document)) == "null" {
			document = nil
		} else {
			return append([]byte(nil), document...), nil
		}
	}

	parsed := map[string]any{
		"schemaVersion": "sde-ui/v0.1",
		"requirementBrief": map[string]any{
			"useCase":                   "",
			"functionalRequirements":    "",
			"nonFunctionalRequirements": "",
			"targetRps":                 nil,
			"availabilityRequirement":   "",
			"sla":                       "",
			"problemStatement":          "",
			"primaryActors":             "",
			"trafficNotes":              "",
			"consistencyNotes":          "",
			"openQuestions":             "",
		},
		"components": []any{},
		"connectors": []any{},
		"journeys":   []any{},
	}
	parsed["id"] = designID
	parsed["title"] = name
	parsed["updatedAt"] = now.Format(time.RFC3339Nano)
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}
