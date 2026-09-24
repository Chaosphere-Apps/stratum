package app

import (
	"context"
	"errors"

	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
)

var ErrReviewNotAssigned = errors.New("only the assigned reviewer can update this review")

type DocsService struct {
	provider RepositoryProvider
}

func (s DocsService) repo() store.Repository { return s.provider.Repository() }

func (s DocsService) List(ctx context.Context, workspaceID string, designID string) ([]domain.DesignDoc, error) {
	return s.repo().ListDesignDocs(ctx, workspaceID, designID)
}

func (s DocsService) Get(ctx context.Context, workspaceID string, designID string, docID string) (domain.DesignDoc, error) {
	return s.repo().GetDesignDoc(ctx, workspaceID, designID, docID)
}

func (s DocsService) Create(ctx context.Context, workspaceID string, designID string, title string, body string, format string) (domain.DesignDoc, error) {
	return s.repo().CreateDesignDoc(ctx, workspaceID, designID, title, body, format)
}

func (s DocsService) Update(ctx context.Context, workspaceID string, designID string, docID string, title string, body string, format string) (domain.DesignDoc, error) {
	return s.repo().UpdateDesignDoc(ctx, workspaceID, designID, docID, title, body, format)
}

func (s DocsService) Delete(ctx context.Context, workspaceID string, designID string, docID string) error {
	return s.repo().DeleteDesignDoc(ctx, workspaceID, designID, docID)
}

type ReviewService struct {
	provider RepositoryProvider
}

func (s ReviewService) repo() store.Repository { return s.provider.Repository() }

func (s ReviewService) ListComments(ctx context.Context, workspaceID string, designID string) ([]domain.DesignComment, error) {
	return s.repo().ListDesignComments(ctx, workspaceID, designID)
}

func (s ReviewService) ListCommentsPage(ctx context.Context, workspaceID string, designID string, options store.PageOptions) ([]domain.DesignComment, store.PageInfo, error) {
	return s.repo().ListDesignCommentsPage(ctx, workspaceID, designID, options)
}

func (s ReviewService) CreateComment(ctx context.Context, workspaceID string, designID string, authorID string, body string, componentID string, connectorID string) (domain.DesignComment, error) {
	return s.repo().CreateDesignComment(ctx, workspaceID, designID, authorID, body, componentID, connectorID)
}

func (s ReviewService) ListReviews(ctx context.Context, workspaceID string, designID string) ([]domain.DesignReviewRequest, error) {
	return s.repo().ListDesignReviewRequests(ctx, workspaceID, designID)
}

func (s ReviewService) ListReviewsPage(ctx context.Context, workspaceID string, designID string, options store.PageOptions) ([]domain.DesignReviewRequest, store.PageInfo, error) {
	return s.repo().ListDesignReviewRequestsPage(ctx, workspaceID, designID, options)
}

func (s ReviewService) CreateReviews(ctx context.Context, workspaceID string, designID string, versionID string, requestedBy string, reviewerIDs []string, message string) ([]domain.DesignReviewRequest, error) {
	return s.repo().CreateDesignReviewRequests(ctx, workspaceID, designID, versionID, requestedBy, reviewerIDs, message)
}

func (s ReviewService) UpdateReview(ctx context.Context, workspaceID string, designID string, reviewID string, actor domain.User, status string, summary string) (domain.DesignReviewRequest, error) {
	reviews, err := s.repo().ListDesignReviewRequests(ctx, workspaceID, designID)
	if err != nil {
		return domain.DesignReviewRequest{}, err
	}
	for _, review := range reviews {
		if review.ID != reviewID {
			continue
		}
		if actor.Role != "admin" && review.ReviewerID != actor.ID {
			return domain.DesignReviewRequest{}, ErrReviewNotAssigned
		}
		return s.repo().UpdateDesignReviewRequest(ctx, workspaceID, designID, reviewID, status, summary)
	}
	return domain.DesignReviewRequest{}, errors.New("review request not found")
}
