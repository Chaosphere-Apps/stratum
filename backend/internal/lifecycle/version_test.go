package lifecycle

import "testing"

func TestNormalizeAndValidateVersionStatuses(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"", VersionDraft},
		{" DRAFT ", VersionDraft},
		{"pending_review", VersionPendingReview},
		{"REVIEWED", VersionReviewed},
		{"live", VersionLive},
		{"unknown", VersionDraft},
	} {
		if got := NormalizeVersionStatus(test.input); got != test.want {
			t.Fatalf("NormalizeVersionStatus(%q) = %q, want %q", test.input, got, test.want)
		}
	}

	for _, status := range []string{VersionDraft, VersionPendingReview, VersionReviewed, VersionLive, " LIVE "} {
		if !IsVersionStatus(status) {
			t.Fatalf("expected %q to be valid", status)
		}
	}
	if IsVersionStatus("ready") {
		t.Fatal("unexpectedly accepted ready as a version status")
	}
}

func TestVersionStatusStateMachine(t *testing.T) {
	allowed := [][2]string{
		{VersionDraft, VersionPendingReview},
		{VersionPendingReview, VersionReviewed},
		{VersionPendingReview, VersionDraft},
		{VersionReviewed, VersionLive},
		{VersionReviewed, VersionDraft},
		{VersionLive, VersionReviewed},
		{VersionLive, VersionDraft},
		{VersionReviewed, VersionReviewed},
	}
	for _, transition := range allowed {
		if err := ValidateVersionStatusTransition(transition[0], transition[1]); err != nil {
			t.Fatalf("transition %s -> %s returned error: %v", transition[0], transition[1], err)
		}
		if !CanTransitionVersionStatus(transition[0], transition[1]) {
			t.Fatalf("transition %s -> %s should be allowed", transition[0], transition[1])
		}
	}

	blocked := [][2]string{
		{VersionDraft, VersionLive},
		{VersionDraft, VersionReviewed},
		{VersionPendingReview, VersionLive},
		{VersionLive, VersionPendingReview},
		{VersionReviewed, "archived"},
	}
	for _, transition := range blocked {
		if err := ValidateVersionStatusTransition(transition[0], transition[1]); err == nil {
			t.Fatalf("transition %s -> %s unexpectedly allowed", transition[0], transition[1])
		}
		if CanTransitionVersionStatus(transition[0], transition[1]) {
			t.Fatalf("transition %s -> %s should be blocked", transition[0], transition[1])
		}
	}
}

func TestLifecycleSideEffects(t *testing.T) {
	if got := StatusAfterDesignChange(VersionPendingReview); got != VersionDraft {
		t.Fatalf("pending review design change status = %q, want draft", got)
	}
	if got := StatusAfterDesignChange(VersionReviewed); got != VersionDraft {
		t.Fatalf("reviewed design change status = %q, want draft", got)
	}
	if got := StatusAfterDesignChange(VersionLive); got != VersionLive {
		t.Fatalf("live design change status = %q, want live", got)
	}
	if got := StatusAfterReviewRequest(VersionDraft); got != VersionPendingReview {
		t.Fatalf("draft review request status = %q, want pending_review", got)
	}
	if got := StatusAfterReviewRequest(VersionReviewed); got != VersionReviewed {
		t.Fatalf("reviewed review request status = %q, want reviewed", got)
	}
	if got := StatusAfterAllReviewsApproved(VersionPendingReview, 2, 0); got != VersionReviewed {
		t.Fatalf("all approvals status = %q, want reviewed", got)
	}
	if got := StatusAfterAllReviewsApproved(VersionPendingReview, 2, 1); got != VersionPendingReview {
		t.Fatalf("incomplete approvals status = %q, want pending_review", got)
	}
	if got := StatusAfterAllReviewsApproved(VersionPendingReview, 0, 0); got != VersionPendingReview {
		t.Fatalf("zero reviews status = %q, want pending_review", got)
	}
}
