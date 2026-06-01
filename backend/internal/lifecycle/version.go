package lifecycle

import (
	"fmt"
	"strings"
)

const (
	VersionDraft         = "draft"
	VersionPendingReview = "pending_review"
	VersionReviewed      = "reviewed"
	VersionLive          = "live"
)

var validVersionStatuses = map[string]struct{}{
	VersionDraft:         {},
	VersionPendingReview: {},
	VersionReviewed:      {},
	VersionLive:          {},
}

func NormalizeVersionStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if _, ok := validVersionStatuses[normalized]; ok {
		return normalized
	}
	return VersionDraft
}

func IsVersionStatus(status string) bool {
	_, ok := validVersionStatuses[strings.ToLower(strings.TrimSpace(status))]
	return ok
}

func NextVersionStatuses(status string) []string {
	switch NormalizeVersionStatus(status) {
	case VersionDraft:
		return []string{VersionPendingReview}
	case VersionPendingReview:
		return []string{VersionReviewed, VersionDraft}
	case VersionReviewed:
		return []string{VersionLive, VersionDraft}
	case VersionLive:
		return []string{VersionReviewed, VersionDraft}
	default:
		return nil
	}
}

func CanTransitionVersionStatus(from string, to string) bool {
	if !IsVersionStatus(to) {
		return false
	}
	from = NormalizeVersionStatus(from)
	to = NormalizeVersionStatus(to)
	if from == to || to == VersionDraft {
		return true
	}
	for _, allowed := range NextVersionStatuses(from) {
		if allowed == to {
			return true
		}
	}
	return false
}

func ValidateVersionStatusTransition(from string, to string) error {
	if !IsVersionStatus(to) {
		return fmt.Errorf("invalid version status %q", strings.TrimSpace(to))
	}
	if CanTransitionVersionStatus(from, to) {
		return nil
	}
	return fmt.Errorf("invalid version status transition from %s to %s", NormalizeVersionStatus(from), NormalizeVersionStatus(to))
}

func StatusAfterDesignChange(status string) string {
	switch NormalizeVersionStatus(status) {
	case VersionDraft, VersionPendingReview, VersionReviewed:
		return VersionDraft
	case VersionLive:
		return VersionLive
	default:
		return VersionDraft
	}
}

func StatusAfterReviewRequest(status string) string {
	if NormalizeVersionStatus(status) == VersionDraft {
		return VersionPendingReview
	}
	return NormalizeVersionStatus(status)
}

func StatusAfterAllReviewsApproved(status string, reviewCount int, incompleteReviewCount int) string {
	if NormalizeVersionStatus(status) != VersionPendingReview || reviewCount == 0 || incompleteReviewCount != 0 {
		return NormalizeVersionStatus(status)
	}
	return VersionReviewed
}
