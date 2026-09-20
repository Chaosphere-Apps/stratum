package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"sort"
	"strings"
)

func (r *MemoryRepository) ListAIConversations(ctx context.Context, workspaceID string, designID string) ([]domain.AIConversation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if design, ok := r.designs[designID]; !ok || design.WorkspaceID != workspaceID {
		return nil, errors.New("design not found")
	}
	items := make([]domain.AIConversation, 0)
	for _, conversation := range r.aiConversations {
		if conversation.WorkspaceID == workspaceID && conversation.DesignID == designID {
			items = append(items, conversation)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (r *MemoryRepository) GetAIConversation(ctx context.Context, workspaceID string, designID string, conversationID string) (domain.AIConversation, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIConversation{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	conversation, ok := r.aiConversations[conversationID]
	if !ok || conversation.WorkspaceID != workspaceID || conversation.DesignID != designID {
		return domain.AIConversation{}, errors.New("AI conversation not found")
	}
	return conversation, nil
}

func (r *MemoryRepository) CreateAIConversation(ctx context.Context, conversation domain.AIConversation) (domain.AIConversation, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIConversation{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	design, ok := r.designs[conversation.DesignID]
	if !ok || design.WorkspaceID != conversation.WorkspaceID {
		return domain.AIConversation{}, errors.New("design not found")
	}
	if strings.TrimSpace(conversation.VersionID) != "" {
		found := false
		for _, version := range r.versions[conversation.DesignID] {
			if version.ID == conversation.VersionID {
				found = true
				break
			}
		}
		if !found {
			return domain.AIConversation{}, errors.New("design version not found")
		}
	}
	now := r.clock().UTC()
	conversation.ID = fmt.Sprintf("ai_conversation_%d", now.UnixNano())
	conversation.Title = strings.TrimSpace(conversation.Title)
	if conversation.Title == "" {
		conversation.Title = "Architecture discussion"
	}
	conversation.CreatedAt = now
	conversation.UpdatedAt = now
	r.aiConversations[conversation.ID] = conversation
	return conversation, nil
}

func (r *MemoryRepository) ListAIMessages(ctx context.Context, conversationID string, limit int) ([]domain.AIMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.aiConversations[conversationID]; !ok {
		return nil, errors.New("AI conversation not found")
	}
	items := make([]domain.AIMessage, 0)
	for _, message := range r.aiMessages {
		if message.ConversationID == conversationID {
			items = append(items, message)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items, nil
}

func (r *MemoryRepository) CreateAIMessage(ctx context.Context, message domain.AIMessage) (domain.AIMessage, error) {
	if err := ctx.Err(); err != nil {
		return domain.AIMessage{}, err
	}
	message.Content = strings.TrimSpace(message.Content)
	if message.Content == "" {
		return domain.AIMessage{}, errors.New("AI message content is required")
	}
	if message.Role != "user" && message.Role != "assistant" {
		return domain.AIMessage{}, errors.New("AI message role is invalid")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	conversation, ok := r.aiConversations[message.ConversationID]
	if !ok {
		return domain.AIMessage{}, errors.New("AI conversation not found")
	}
	now := r.clock().UTC()
	message.ID = fmt.Sprintf("ai_message_%d", now.UnixNano())
	message.CreatedAt = now
	r.aiMessages[message.ID] = message
	conversation.UpdatedAt = now
	r.aiConversations[conversation.ID] = conversation
	return message, nil
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
