package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/lifecycle"
	"strings"
	"time"
)

func (r *PostgresRepository) ListDesignComments(ctx context.Context, workspaceID string, designID string) ([]domain.DesignComment, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.DesignComment, PageInfo, error) {
		return r.ListDesignCommentsPage(ctx, workspaceID, designID, options)
	})
}

func (r *PostgresRepository) ListDesignCommentsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignComment, PageInfo, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, design_id, author_id, body, component_id, connector_id, created_at
FROM design_comments
WHERE workspace_id = $1 AND design_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4
`, workspaceID, designID, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()

	comments := []domain.DesignComment{}
	for rows.Next() {
		comment, err := scanDesignComment(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(comments, options, offset)
	return items, page, nil
}

func (r *PostgresRepository) CreateDesignComment(ctx context.Context, workspaceID string, designID string, authorID string, body string, componentID string, connectorID string) (domain.DesignComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return domain.DesignComment{}, errors.New("comment body is required")
	}
	if authorID == "" {
		authorID = r.firstUserID(ctx)
	}
	author, err := r.GetUser(ctx, authorID)
	if err != nil {
		return domain.DesignComment{}, err
	}
	authorID = author.ID

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignComment{}, err
	}
	defer rollback(ctx, tx)

	design, err := selectDesignForUpdate(ctx, tx, workspaceID, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignComment{}, errors.New("design not found")
	}
	if err != nil {
		return domain.DesignComment{}, err
	}

	now := r.clock().UTC()
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
	if _, err := tx.Exec(ctx, `
INSERT INTO design_comments (id, workspace_id, design_id, author_id, body, component_id, connector_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`, comment.ID, comment.WorkspaceID, comment.DesignID, comment.AuthorID, comment.Body, comment.ComponentID, comment.ConnectorID, comment.CreatedAt); err != nil {
		return domain.DesignComment{}, err
	}
	if design.CreatedBy != "" && design.CreatedBy != authorID {
		if err := insertNotification(ctx, tx, notificationFor(design.CreatedBy, workspaceID, designID, "comment_added", "New comment", author.DisplayName+" commented on "+design.Name, now)); err != nil {
			return domain.DesignComment{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignComment{}, err
	}
	return comment, nil
}

func (r *PostgresRepository) ListDesignReviewRequests(ctx context.Context, workspaceID string, designID string) ([]domain.DesignReviewRequest, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.DesignReviewRequest, PageInfo, error) {
		return r.ListDesignReviewRequestsPage(ctx, workspaceID, designID, options)
	})
}

func (r *PostgresRepository) ListDesignReviewRequestsPage(ctx context.Context, workspaceID string, designID string, options PageOptions) ([]domain.DesignReviewRequest, PageInfo, error) {
	if _, err := r.GetDesign(ctx, workspaceID, designID); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	offset := OffsetFromCursor(options.Cursor)
	rows, err := r.pool.Query(ctx, `
SELECT id, workspace_id, design_id, version_id, version_number, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at
FROM design_review_requests
WHERE workspace_id = $1 AND design_id = $2
ORDER BY updated_at DESC
LIMIT $3 OFFSET $4
`, workspaceID, designID, options.Limit+1, offset)
	if err != nil {
		return nil, PageInfo{}, err
	}
	defer rows.Close()

	reviews := []domain.DesignReviewRequest{}
	for rows.Next() {
		review, err := scanDesignReviewRequest(rows)
		if err != nil {
			return nil, PageInfo{}, err
		}
		reviews = append(reviews, review)
	}
	if err := rows.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	items, page := pageFromFetched(reviews, options, offset)
	return items, page, nil
}

func (r *PostgresRepository) CreateDesignReviewRequests(ctx context.Context, workspaceID string, designID string, versionID string, requestedBy string, reviewerIDs []string, message string) ([]domain.DesignReviewRequest, error) {
	reviewerIDs = normalizeReviewerIDs(reviewerIDs)
	if len(reviewerIDs) == 0 {
		return nil, errors.New("at least one reviewer id is required")
	}
	if requestedBy == "" {
		requestedBy = r.firstUserID(ctx)
	}
	requester, err := r.GetUser(ctx, requestedBy)
	if err != nil {
		return nil, err
	}
	requestedBy = requester.ID

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)

	design, err := selectDesignForUpdate(ctx, tx, workspaceID, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("design not found")
	}
	if err != nil {
		return nil, err
	}

	now := r.clock().UTC()
	version, err := resolveDesignVersionTx(ctx, tx, workspaceID, designID, versionID, design.VersionNumber)
	if err != nil {
		return nil, err
	}
	if lifecycle.NormalizeVersionStatus(version.Status) != lifecycle.VersionDraft {
		return nil, errors.New("only draft versions can be submitted for review")
	}
	reviews := make([]domain.DesignReviewRequest, 0, len(reviewerIDs))
	for index, reviewerID := range reviewerIDs {
		reviewer, err := r.GetUser(ctx, reviewerID)
		if err != nil {
			return nil, err
		}
		reviewerID = reviewer.ID
		existingRow := tx.QueryRow(ctx, `
SELECT id, workspace_id, design_id, version_id, version_number, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at
FROM design_review_requests
WHERE workspace_id = $1 AND design_id = $2 AND version_id = $3 AND reviewer_id = $4
`, workspaceID, designID, version.ID, reviewerID)
		existing, scanErr := scanDesignReviewRequest(existingRow)
		if scanErr == nil {
			existing.Status = "requested"
			existing.Message = strings.TrimSpace(message)
			existing.Summary = ""
			existing.CompletedAt = nil
			existing.UpdatedAt = now
			if _, err := tx.Exec(ctx, `
UPDATE design_review_requests
SET requested_by = $1, status = 'requested', message = $2, summary = '', updated_at = $3, completed_at = NULL
WHERE id = $4
`, requestedBy, existing.Message, now, existing.ID); err != nil {
				return nil, err
			}
			if err := insertNotification(ctx, tx, notificationFor(reviewerID, workspaceID, designID, "review_requested", "Review requested", requester.DisplayName+" requested your review on "+design.Name, now)); err != nil {
				return nil, err
			}
			reviews = append(reviews, existing)
			continue
		}
		if !errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, scanErr
		}
		review := domain.DesignReviewRequest{
			ID:            fmt.Sprintf("review_%d_%d", now.UnixNano(), index),
			WorkspaceID:   workspaceID,
			DesignID:      designID,
			VersionID:     version.ID,
			VersionNumber: version.VersionNumber,
			RequestedBy:   requestedBy,
			ReviewerID:    reviewerID,
			Status:        "requested",
			Message:       strings.TrimSpace(message),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO design_review_requests (id, workspace_id, design_id, version_id, version_number, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, '', $10, $10, NULL)
`, review.ID, review.WorkspaceID, review.DesignID, review.VersionID, review.VersionNumber, review.RequestedBy, review.ReviewerID, review.Status, review.Message, now); err != nil {
			return nil, err
		}
		if err := insertNotification(ctx, tx, notificationFor(reviewerID, workspaceID, designID, "review_requested", "Review requested", requester.DisplayName+" requested your review on "+design.Name, now)); err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	nextStatus := lifecycle.StatusAfterReviewRequest(version.Status)
	if nextStatus != version.Status {
		if _, err := updateDesignVersionStatusTx(ctx, tx, workspaceID, designID, version.ID, nextStatus, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return reviews, nil
}

func (r *PostgresRepository) UpdateDesignReviewRequest(ctx context.Context, workspaceID string, designID string, reviewID string, status string, summary string) (domain.DesignReviewRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	defer rollback(ctx, tx)

	design, err := selectDesignForUpdate(ctx, tx, workspaceID, designID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignReviewRequest{}, errors.New("design not found")
	}
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}

	row := tx.QueryRow(ctx, `
SELECT id, workspace_id, design_id, version_id, version_number, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at
FROM design_review_requests
WHERE workspace_id = $1 AND design_id = $2 AND id = $3
FOR UPDATE
`, workspaceID, designID, reviewID)
	existing, err := scanDesignReviewRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DesignReviewRequest{}, errors.New("review request not found")
	}
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}

	now := r.clock().UTC()
	nextStatus := normalizedReviewStatus(status, existing.Status)
	reviewer, err := r.GetUser(ctx, existing.ReviewerID)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	var completedAt *time.Time
	if nextStatus == "approved" || nextStatus == "changes_requested" {
		completedAt = &now
	}
	row = tx.QueryRow(ctx, `
UPDATE design_review_requests
SET status = $1, summary = $2, updated_at = $3, completed_at = $4
WHERE workspace_id = $5 AND design_id = $6 AND id = $7
RETURNING id, workspace_id, design_id, version_id, version_number, requested_by, reviewer_id, status, message, summary, created_at, updated_at, completed_at
`, nextStatus, strings.TrimSpace(summary), now, completedAt, workspaceID, designID, reviewID)
	review, err := scanDesignReviewRequest(row)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	if nextStatus == "approved" && review.VersionID != "" {
		if err := markVersionReviewedIfApprovedTx(ctx, tx, workspaceID, designID, review.VersionID, now); err != nil {
			return domain.DesignReviewRequest{}, err
		}
	} else if nextStatus == "changes_requested" && review.VersionID != "" {
		if _, err := updateDesignVersionStatusTx(ctx, tx, workspaceID, designID, review.VersionID, lifecycle.VersionDraft, now); err != nil {
			return domain.DesignReviewRequest{}, err
		}
	}
	if design.CreatedBy != "" && design.CreatedBy != review.ReviewerID {
		if err := insertNotification(ctx, tx, notificationFor(design.CreatedBy, workspaceID, designID, "review_updated", "Review updated", reviewer.DisplayName+" marked review as "+strings.ReplaceAll(nextStatus, "_", " "), now)); err != nil {
			return domain.DesignReviewRequest{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DesignReviewRequest{}, err
	}
	return review, nil
}

func markVersionReviewedIfApprovedTx(ctx context.Context, tx pgx.Tx, workspaceID string, designID string, versionID string, now time.Time) error {
	var incompleteCount int
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)
FROM design_review_requests
WHERE workspace_id = $1 AND design_id = $2 AND version_id = $3 AND status <> 'approved'
`, workspaceID, designID, versionID).Scan(&incompleteCount); err != nil {
		return err
	}
	if incompleteCount != 0 {
		return nil
	}
	var reviewCount int
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)
FROM design_review_requests
WHERE workspace_id = $1 AND design_id = $2 AND version_id = $3
`, workspaceID, designID, versionID).Scan(&reviewCount); err != nil {
		return err
	}
	if reviewCount == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
UPDATE design_versions
SET status = 'reviewed', updated_at = $1
WHERE workspace_id = $2 AND design_id = $3 AND id = $4 AND status = 'pending_review'
`, now, workspaceID, designID, versionID)
	return err
}

func (r *PostgresRepository) ListNotifications(ctx context.Context, userID string) ([]domain.Notification, error) {
	if strings.TrimSpace(userID) == "" {
		userID = r.firstUserID(ctx)
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, user_id, workspace_id, design_id, type, title, body, read, created_at
FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 80
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := []domain.Notification{}
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (r *PostgresRepository) MarkNotificationRead(ctx context.Context, userID string, notificationID string) error {
	if strings.TrimSpace(userID) == "" {
		userID = r.firstUserID(ctx)
	}
	tag, err := r.pool.Exec(ctx, `UPDATE notifications SET read = TRUE WHERE user_id = $1 AND id = $2`, userID, notificationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("notification not found")
	}
	return nil
}
