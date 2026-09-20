package store

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"testing"
)

func TestMemoryRepositoryReviewRequestsAreVersionScopedAndBulkCreated(t *testing.T) {
	repo := NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	reviewerOne, err := repo.CreateUser(context.Background(), "Reviewer One", "reviewer1@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser reviewer one returned error: %v", err)
	}
	reviewerTwo, err := repo.CreateUser(context.Background(), "Reviewer Two", "reviewer2@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser reviewer two returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Reviewable", nil, admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	version, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion returned error: %v", err)
	}

	reviews, err := repo.CreateDesignReviewRequests(context.Background(), workspace.ID, design.ID, version.ID, admin.ID, []string{reviewerOne.ID, reviewerTwo.ID, reviewerOne.ID}, "please review")
	if err != nil {
		t.Fatalf("CreateDesignReviewRequests returned error: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("review count = %d, want 2", len(reviews))
	}
	for _, review := range reviews {
		if review.VersionID != version.ID || review.VersionNumber != version.VersionNumber || review.Status != "requested" {
			t.Fatalf("review version/status = %#v, want version %q requested", review, version.ID)
		}
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	if versions[0].Status != "pending_review" {
		t.Fatalf("version status = %q, want pending_review", versions[0].Status)
	}

	if _, err := repo.UpdateDesignReviewRequest(context.Background(), workspace.ID, design.ID, reviews[0].ID, "approved", "ok"); err != nil {
		t.Fatalf("UpdateDesignReviewRequest first approval returned error: %v", err)
	}
	versions, err = repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions after first approval returned error: %v", err)
	}
	if versions[0].Status != "pending_review" {
		t.Fatalf("version status after one approval = %q, want pending_review", versions[0].Status)
	}
	if _, err := repo.UpdateDesignReviewRequest(context.Background(), workspace.ID, design.ID, reviews[1].ID, "approved", "ok"); err != nil {
		t.Fatalf("UpdateDesignReviewRequest second approval returned error: %v", err)
	}
	versions, err = repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions after all approvals returned error: %v", err)
	}
	if versions[0].Status != "reviewed" {
		t.Fatalf("version status after all approvals = %q, want reviewed", versions[0].Status)
	}
}

func TestMemoryRepositoryReviewChangesRequestedDoesNotMarkVersionReviewed(t *testing.T) {
	repo := NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	reviewer, err := repo.CreateUser(context.Background(), "Reviewer", "reviewer@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser reviewer returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Reviewable", nil, admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	version, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion returned error: %v", err)
	}
	reviews, err := repo.CreateDesignReviewRequests(context.Background(), workspace.ID, design.ID, version.ID, admin.ID, []string{reviewer.ID}, "please review")
	if err != nil {
		t.Fatalf("CreateDesignReviewRequests returned error: %v", err)
	}
	if _, err := repo.UpdateDesignReviewRequest(context.Background(), workspace.ID, design.ID, reviews[0].ID, "changes_requested", "needs work"); err != nil {
		t.Fatalf("UpdateDesignReviewRequest changes requested returned error: %v", err)
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	if versions[0].Status != "draft" {
		t.Fatalf("version status = %q, want draft after changes requested", versions[0].Status)
	}
	resubmitted, err := repo.CreateDesignReviewRequests(context.Background(), workspace.ID, design.ID, version.ID, admin.ID, []string{reviewer.ID}, "updated design")
	if err != nil {
		t.Fatalf("resubmit review returned error: %v", err)
	}
	if len(resubmitted) != 1 || resubmitted[0].ID != reviews[0].ID || resubmitted[0].Status != "requested" || resubmitted[0].Summary != "" {
		t.Fatalf("resubmitted review = %#v", resubmitted)
	}
	versions, err = repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil || versions[0].Status != "pending_review" {
		t.Fatalf("resubmitted version = %#v, err=%v", versions, err)
	}
}

func TestMemoryRepositoryOnlyOneLiveVersion(t *testing.T) {
	repo := NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Versioned", nil, admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	versionOne, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion one returned error: %v", err)
	}
	versionTwo, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion two returned error: %v", err)
	}
	for _, version := range []domain.DesignVersion{versionOne, versionTwo} {
		if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, design.ID, version.ID, "pending_review"); err != nil {
			t.Fatalf("UpdateDesignVersionStatus pending for %s returned error: %v", version.ID, err)
		}
		if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, design.ID, version.ID, "reviewed"); err != nil {
			t.Fatalf("UpdateDesignVersionStatus reviewed for %s returned error: %v", version.ID, err)
		}
	}
	if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, design.ID, versionOne.ID, "live"); err != nil {
		t.Fatalf("UpdateDesignVersionStatus live one returned error: %v", err)
	}
	if _, err := repo.UpdateDesignVersionStatus(context.Background(), workspace.ID, design.ID, versionTwo.ID, "live"); err != nil {
		t.Fatalf("UpdateDesignVersionStatus live two returned error: %v", err)
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	liveCount := 0
	for _, version := range versions {
		if version.Status == "live" {
			liveCount++
			if version.ID != versionTwo.ID {
				t.Fatalf("live version id = %q, want %q", version.ID, versionTwo.ID)
			}
		}
		if version.ID == versionOne.ID && version.Status != "reviewed" {
			t.Fatalf("previous live version status = %q, want reviewed", version.Status)
		}
	}
	if liveCount != 1 {
		t.Fatalf("live version count = %d, want 1", liveCount)
	}
}

func TestMemoryRepositoryDeletesDesignVersionAndVersionReviews(t *testing.T) {
	repo := NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	reviewer, err := repo.CreateUser(context.Background(), "Reviewer", "reviewer@example.com", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser reviewer returned error: %v", err)
	}
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	design, err := repo.CreateDesign(context.Background(), workspace.ID, "Reviewable", nil, admin.ID)
	if err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}
	versionOne, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion one returned error: %v", err)
	}
	versionTwo, err := repo.CreateDesignVersion(context.Background(), workspace.ID, design.ID, admin.ID, "")
	if err != nil {
		t.Fatalf("CreateDesignVersion two returned error: %v", err)
	}
	if _, err := repo.CreateDesignReviewRequests(context.Background(), workspace.ID, design.ID, versionTwo.ID, admin.ID, []string{reviewer.ID}, "please review"); err != nil {
		t.Fatalf("CreateDesignReviewRequests returned error: %v", err)
	}

	if err := repo.DeleteDesignVersion(context.Background(), workspace.ID, design.ID, versionTwo.ID); err != nil {
		t.Fatalf("DeleteDesignVersion returned error: %v", err)
	}
	versions, err := repo.ListDesignVersions(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignVersions returned error: %v", err)
	}
	if len(versions) != 1 || versions[0].ID != versionOne.ID {
		t.Fatalf("versions = %#v, want only version one", versions)
	}
	reviews, err := repo.ListDesignReviewRequests(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("ListDesignReviewRequests returned error: %v", err)
	}
	if len(reviews) != 0 {
		t.Fatalf("review count after version delete = %d, want 0", len(reviews))
	}
	updatedDesign, err := repo.GetDesign(context.Background(), workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("GetDesign returned error: %v", err)
	}
	if updatedDesign.VersionNumber != versionOne.VersionNumber {
		t.Fatalf("design version number = %d, want %d", updatedDesign.VersionNumber, versionOne.VersionNumber)
	}
}
