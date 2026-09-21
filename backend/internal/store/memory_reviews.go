package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/lifecycle"
	"sort"
	"strings"
	"time"
)

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
			existing.Status = "requested"
			existing.Message = strings.TrimSpace(message)
			existing.Summary = ""
			existing.CompletedAt = nil
			existing.RequestedBy = requestedBy
			existing.UpdatedAt = now
			r.reviews[existing.ID] = existing
			r.addNotificationLocked(reviewer.ID, workspaceID, designID, "review_requested", "Review requested", requester.DisplayName+" requested your review on "+design.Name, now)
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
	} else if status == "changes_requested" && review.VersionID != "" {
		if _, err := r.updateDesignVersionStatusLocked(workspaceID, designID, review.VersionID, "draft", now); err != nil {
			return domain.DesignReviewRequest{}, err
		}
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
