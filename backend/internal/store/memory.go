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
	mu         sync.RWMutex
	workspaces map[string]domain.Workspace
	designs    map[string]domain.Design
	versions   map[string][]domain.DesignVersion
	docs       map[string]domain.DesignDoc
	clock      func() time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		workspaces: make(map[string]domain.Workspace),
		designs:    make(map[string]domain.Design),
		versions:   make(map[string][]domain.DesignVersion),
		docs:       make(map[string]domain.DesignDoc),
		clock:      time.Now,
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

func (r *MemoryRepository) CreateDesign(ctx context.Context, workspaceID string, name string, document []byte) (domain.Design, error) {
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
		CreatedBy:     domain.GuestUserID,
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
			design.CreatedBy = domain.GuestUserID
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
		CreatedBy:   domain.GuestUserID,
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

func normalizedDocTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled doc"
	}
	return title
}

func normalizedDocFormat(format string) string {
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		return "html"
	}
	return format
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
