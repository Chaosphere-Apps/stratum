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
)

type MemoryRepository struct {
	mu            sync.RWMutex
	workspaces    map[string]domain.Workspace
	designs       map[string]domain.Design
	versions      map[string][]domain.DesignVersion
	docs          map[string]domain.DesignDoc
	users         map[string]domain.User
	sessions      map[string]memorySession
	signInConfig  domain.SignInConfig
	aiConfig      domain.AIProviderConfig
	catalogAssets map[string]domain.CatalogAsset
	comments      map[string]domain.DesignComment
	reviews       map[string]domain.DesignReviewRequest
	notifications map[string]domain.Notification
	clock         func() time.Time
}

type memorySession struct {
	UserID     string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		workspaces:    make(map[string]domain.Workspace),
		designs:       make(map[string]domain.Design),
		versions:      make(map[string][]domain.DesignVersion),
		docs:          make(map[string]domain.DesignDoc),
		users:         make(map[string]domain.User),
		sessions:      make(map[string]memorySession),
		signInConfig:  defaultSignInConfig(time.Now().UTC()),
		aiConfig:      defaultAIProviderConfig(time.Now().UTC()),
		catalogAssets: make(map[string]domain.CatalogAsset),
		comments:      make(map[string]domain.DesignComment),
		reviews:       make(map[string]domain.DesignReviewRequest),
		notifications: make(map[string]domain.Notification),
		clock:         time.Now,
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := r.GetOrCreateGuestWorkspace(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	workspaces := make([]domain.Workspace, 0, len(r.workspaces))
	for _, workspace := range r.workspaces {
		workspaces = append(workspaces, workspace)
	}
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].UpdatedAt.After(workspaces[j].UpdatedAt)
	})
	return workspaces, nil
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if workspaceID == "" {
		return nil, errors.New("workspace id is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	designs := make([]domain.Design, 0, len(r.designs))
	for _, design := range r.designs {
		if design.WorkspaceID == workspaceID {
			designs = append(designs, design)
		}
	}

	sort.Slice(designs, func(i, j int) bool {
		return designs[i].UpdatedAt.After(designs[j].UpdatedAt)
	})

	return designs, nil
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
		VersionNumber: 1,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r.designs[design.ID] = design
	r.versions[design.ID] = append(r.versions[design.ID], domain.DesignVersion{
		ID:            fmt.Sprintf("%s_v1", design.ID),
		DesignID:      design.ID,
		WorkspaceID:   design.WorkspaceID,
		VersionNumber: 1,
		Document:      append([]byte(nil), design.Document...),
		CreatedBy:     design.CreatedBy,
		CreatedAt:     now,
	})
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
	versionNumber := 1
	if exists {
		design.CreatedAt = existing.CreatedAt
		design.CreatedBy = existing.CreatedBy
		design.Name = existing.Name
		design.Access = existing.Access
		design.Title = existing.Title
		versionNumber = existing.VersionNumber + 1
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
	}
	if design.Title == "" {
		design.Title = design.Name
	}
	design.VersionNumber = versionNumber
	design.UpdatedAt = now
	r.designs[design.ID] = design

	if workspace, ok := r.workspaces[design.WorkspaceID]; ok {
		workspace.UpdatedAt = now
		r.workspaces[workspace.ID] = workspace
	}

	version := domain.DesignVersion{
		ID:             fmt.Sprintf("%s_v%d", design.ID, versionNumber),
		DesignID:       design.ID,
		WorkspaceID:    design.WorkspaceID,
		VersionNumber:  versionNumber,
		Document:       append([]byte(nil), design.Document...),
		CanvasSnapshot: append([]byte(nil), design.CanvasSnapshot...),
		CreatedBy:      design.CreatedBy,
		CreatedAt:      now,
	}
	r.versions[design.ID] = append(r.versions[design.ID], version)

	return design, nil
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if workspaceID == "" || designID == "" {
		return nil, errors.New("workspace id and design id are required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return nil, errors.New("design not found")
	}

	versions := append([]domain.DesignVersion(nil), r.versions[designID]...)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].VersionNumber > versions[j].VersionNumber
	})
	return versions, nil
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

func (r *MemoryRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users, nil
}

func (r *MemoryRepository) ListCatalogAssets(ctx context.Context, query string) ([]domain.CatalogAsset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalizedQuery := normalizedCatalogName(query)
	r.mu.RLock()
	defer r.mu.RUnlock()

	assets := make([]domain.CatalogAsset, 0, len(r.catalogAssets))
	for _, asset := range r.catalogAssets {
		if normalizedQuery != "" && !strings.Contains(asset.NormalizedName, normalizedQuery) && !strings.Contains(strings.ToLower(asset.Name), strings.ToLower(strings.TrimSpace(query))) {
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
	return assets, nil
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

func (r *MemoryRepository) ListDesignComments(ctx context.Context, workspaceID string, designID string) ([]domain.DesignComment, error) {
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
	comments := make([]domain.DesignComment, 0, len(r.comments))
	for _, comment := range r.comments {
		if comment.WorkspaceID == workspaceID && comment.DesignID == designID {
			comments = append(comments, comment)
		}
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].CreatedAt.After(comments[j].CreatedAt)
	})
	return comments, nil
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
	reviews := make([]domain.DesignReviewRequest, 0, len(r.reviews))
	for _, review := range r.reviews {
		if review.WorkspaceID == workspaceID && review.DesignID == designID {
			reviews = append(reviews, review)
		}
	}
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].UpdatedAt.After(reviews[j].UpdatedAt)
	})
	return reviews, nil
}

func (r *MemoryRepository) CreateDesignReviewRequest(ctx context.Context, workspaceID string, designID string, requestedBy string, reviewerID string, message string) (domain.DesignReviewRequest, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignReviewRequest{}, err
	}
	if strings.TrimSpace(reviewerID) == "" {
		return domain.DesignReviewRequest{}, errors.New("reviewer id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	design, ok := r.designs[designID]
	if !ok || design.WorkspaceID != workspaceID {
		return domain.DesignReviewRequest{}, errors.New("design not found")
	}
	now := r.clock().UTC()
	if requestedBy == "" {
		requestedBy = r.firstUserIDLocked()
	}
	requester, err := r.getUserLocked(requestedBy)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	reviewer, err := r.getUserLocked(reviewerID)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	review := domain.DesignReviewRequest{
		ID:          fmt.Sprintf("review_%d", now.UnixNano()),
		WorkspaceID: workspaceID,
		DesignID:    designID,
		RequestedBy: requestedBy,
		ReviewerID:  reviewerID,
		Status:      "requested",
		Message:     strings.TrimSpace(message),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.reviews[review.ID] = review
	r.addNotificationLocked(reviewer.ID, workspaceID, designID, "review_requested", "Review requested", requester.DisplayName+" requested your review on "+design.Name, now)
	return review, nil
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

func sanitizeAIProviderConfig(config domain.AIProviderConfig) domain.AIProviderConfig {
	config.APIKey = ""
	return config
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
