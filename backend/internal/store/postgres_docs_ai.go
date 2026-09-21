package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
)

func (r *PostgresRepository) ListDesignDocs(ctx context.Context, workspaceID string, designID string) ([]domain.DesignDoc, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, design_id, title, body, format, created_by, created_at, updated_at
FROM design_docs
WHERE workspace_id = $1 AND design_id = $2
ORDER BY updated_at DESC
`, workspaceID, designID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []domain.DesignDoc{}
	for rows.Next() {
		doc, err := scanDesignDoc(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (r *PostgresRepository) GetDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) (domain.DesignDoc, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" || strings.TrimSpace(docID) == "" {
		return domain.DesignDoc{}, errors.New("workspace id, design id, and doc id are required")
	}
	row := r.pool.QueryRow(ctx, `
SELECT id, workspace_id, design_id, title, body, format, created_by, created_at, updated_at
FROM design_docs
WHERE workspace_id = $1 AND design_id = $2 AND id = $3
`, workspaceID, designID, docID)
	doc, err := scanDesignDoc(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignDoc{}, errors.New("design doc not found")
	}
	return doc, err
}

func (r *PostgresRepository) CreateDesignDoc(ctx context.Context, workspaceID string, designID string, title string, body string, format string) (domain.DesignDoc, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" {
		return domain.DesignDoc{}, errors.New("workspace id and design id are required")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignDoc{}, err
	}
	defer rollback(ctx, tx)

	if err := ensureDesignExists(ctx, tx, workspaceID, designID); err != nil {
		return domain.DesignDoc{}, err
	}
	now := r.clock().UTC()
	doc := domain.DesignDoc{
		ID:          fmt.Sprintf("doc_%d", now.UnixNano()),
		WorkspaceID: workspaceID,
		DesignID:    designID,
		Title:       normalizedDocTitle(title),
		Body:        body,
		Format:      normalizedDocFormat(format),
		CreatedBy:   r.firstUserID(ctx),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO design_docs (id, workspace_id, design_id, title, body, format, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
`, doc.ID, doc.WorkspaceID, doc.DesignID, doc.Title, doc.Body, doc.Format, doc.CreatedBy, now); err != nil {
		return domain.DesignDoc{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignDoc{}, err
	}
	return doc, nil
}

func (r *PostgresRepository) UpdateDesignDoc(ctx context.Context, workspaceID string, designID string, docID string, title string, body string, format string) (domain.DesignDoc, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" || strings.TrimSpace(docID) == "" {
		return domain.DesignDoc{}, errors.New("workspace id, design id, and doc id are required")
	}
	now := r.clock().UTC()
	row := r.pool.QueryRow(ctx, `
UPDATE design_docs
SET title = $1, body = $2, format = $3, updated_at = $4
WHERE workspace_id = $5 AND design_id = $6 AND id = $7
RETURNING id, workspace_id, design_id, title, body, format, created_by, created_at, updated_at
`, normalizedDocTitle(title), body, normalizedDocFormat(format), now, workspaceID, designID, docID)
	doc, err := scanDesignDoc(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignDoc{}, errors.New("design doc not found")
	}
	return doc, err
}

func (r *PostgresRepository) DeleteDesignDoc(ctx context.Context, workspaceID string, designID string, docID string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(designID) == "" || strings.TrimSpace(docID) == "" {
		return errors.New("workspace id, design id, and doc id are required")
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM design_docs WHERE workspace_id = $1 AND design_id = $2 AND id = $3`, workspaceID, designID, docID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("design doc not found")
	}
	return nil
}

func (r *PostgresRepository) ListAIConversations(ctx context.Context, workspaceID string, designID string) ([]domain.AIConversation, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, design_id, COALESCE(version_id, ''), title, access_mode, created_by, created_at, updated_at
FROM ai_conversations
WHERE workspace_id = $1 AND design_id = $2
ORDER BY updated_at DESC
`, workspaceID, designID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.AIConversation{}
	for rows.Next() {
		var item domain.AIConversation
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.DesignID, &item.VersionID, &item.Title, &item.AccessMode, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetAIConversation(ctx context.Context, workspaceID string, designID string, conversationID string) (domain.AIConversation, error) {
	var item domain.AIConversation
	err := r.pool.QueryRow(ctx, `
SELECT id, workspace_id, design_id, COALESCE(version_id, ''), title, access_mode, created_by, created_at, updated_at
FROM ai_conversations
WHERE workspace_id = $1 AND design_id = $2 AND id = $3
`, workspaceID, designID, conversationID).Scan(
		&item.ID, &item.WorkspaceID, &item.DesignID, &item.VersionID, &item.Title, &item.AccessMode, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AIConversation{}, errors.New("AI conversation not found")
	}
	return item, err
}

func (r *PostgresRepository) CreateAIConversation(ctx context.Context, conversation domain.AIConversation) (domain.AIConversation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AIConversation{}, err
	}
	defer rollback(ctx, tx)
	if err := ensureDesignExists(ctx, tx, conversation.WorkspaceID, conversation.DesignID); err != nil {
		return domain.AIConversation{}, err
	}
	if strings.TrimSpace(conversation.VersionID) != "" {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM design_versions WHERE workspace_id = $1 AND design_id = $2 AND id = $3)`, conversation.WorkspaceID, conversation.DesignID, conversation.VersionID).Scan(&exists); err != nil {
			return domain.AIConversation{}, err
		}
		if !exists {
			return domain.AIConversation{}, errors.New("design version not found")
		}
	}
	now := r.clock().UTC()
	conversation.ID = fmt.Sprintf("chat_%d", now.UnixNano())
	conversation.Title = strings.TrimSpace(conversation.Title)
	if conversation.Title == "" {
		conversation.Title = "Architecture discussion"
	}
	conversation.AccessMode = normalizedAIConversationAccess(conversation.AccessMode)
	conversation.CreatedAt = now
	conversation.UpdatedAt = now
	var versionID any
	if strings.TrimSpace(conversation.VersionID) != "" {
		versionID = conversation.VersionID
	}
	_, err = tx.Exec(ctx, `
INSERT INTO ai_conversations (id, workspace_id, design_id, version_id, title, access_mode, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
`, conversation.ID, conversation.WorkspaceID, conversation.DesignID, versionID, conversation.Title, conversation.AccessMode, conversation.CreatedBy, now)
	if err != nil {
		return domain.AIConversation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AIConversation{}, err
	}
	return conversation, nil
}

func (r *PostgresRepository) UpdateAIConversationAccess(ctx context.Context, workspaceID string, designID string, conversationID string, accessMode string) (domain.AIConversation, error) {
	var item domain.AIConversation
	err := r.pool.QueryRow(ctx, `
UPDATE ai_conversations
SET access_mode = $4, updated_at = $5
WHERE workspace_id = $1 AND design_id = $2 AND id = $3
RETURNING id, workspace_id, design_id, COALESCE(version_id, ''), title, access_mode, created_by, created_at, updated_at
`, workspaceID, designID, conversationID, normalizedAIConversationAccess(accessMode), r.clock().UTC()).Scan(
		&item.ID, &item.WorkspaceID, &item.DesignID, &item.VersionID, &item.Title, &item.AccessMode, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AIConversation{}, errors.New("AI conversation not found")
	}
	return item, err
}

func (r *PostgresRepository) ListAIMessages(ctx context.Context, conversationID string, limit int) ([]domain.AIMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, conversation_id, role, content, references_json, provider, model, COALESCE(created_by, ''), created_at
FROM (
    SELECT id, conversation_id, role, content, references_json, provider, model, created_by, created_at
    FROM ai_messages
    WHERE conversation_id = $1
    ORDER BY created_at DESC
    LIMIT $2
) recent
ORDER BY created_at ASC
`, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.AIMessage{}
	for rows.Next() {
		var item domain.AIMessage
		var references []byte
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.Role, &item.Content, &references, &item.Provider, &item.Model, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(references, &item.References); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateAIMessage(ctx context.Context, message domain.AIMessage) (domain.AIMessage, error) {
	message.Content = strings.TrimSpace(message.Content)
	if message.Content == "" {
		return domain.AIMessage{}, errors.New("AI message content is required")
	}
	if message.Role != "user" && message.Role != "assistant" {
		return domain.AIMessage{}, errors.New("AI message role is invalid")
	}
	references, err := json.Marshal(message.References)
	if err != nil {
		return domain.AIMessage{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AIMessage{}, err
	}
	defer rollback(ctx, tx)
	now := r.clock().UTC()
	message.ID = fmt.Sprintf("ai_message_%d", now.UnixNano())
	message.CreatedAt = now
	var createdBy any
	if strings.TrimSpace(message.CreatedBy) != "" {
		createdBy = message.CreatedBy
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO ai_messages (id, conversation_id, role, content, references_json, provider, model, created_by, created_at)
SELECT $1, id, $3, $4, $5, $6, $7, $8, $9 FROM ai_conversations WHERE id = $2
`, message.ID, message.ConversationID, message.Role, message.Content, references, message.Provider, message.Model, createdBy, now)
	if err != nil {
		return domain.AIMessage{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.AIMessage{}, errors.New("AI conversation not found")
	}
	if _, err := tx.Exec(ctx, `UPDATE ai_conversations SET updated_at = $1 WHERE id = $2`, now, message.ConversationID); err != nil {
		return domain.AIMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AIMessage{}, err
	}
	return message, nil
}
