package store

import (
	"context"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestMemoryRepositoryRejectsDuplicateUserEmail(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	if _, err := repo.CreateUser(context.Background(), "Duplicate", "ADMIN@example.com", "member", ""); err == nil || !strings.Contains(err.Error(), "email already exists") {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

func TestMemoryRepositoryRejectsDuplicateEmailOnUpdate(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	user, err := repo.CreateUser(context.Background(), "Tarun", "tarun@example.com", "member", "")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if _, err := repo.UpdateUser(context.Background(), user.ID, "", "admin@example.com", "", "", ""); err == nil || !strings.Contains(err.Error(), "email already exists") {
		t.Fatalf("expected duplicate email update error, got %v", err)
	}
}

func TestMemoryRepositoryLocksRepeatedInvalidLocalLogin(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	repo.clock = func() time.Time { return now }
	if _, err := repo.CreateFirstAdmin(context.Background(), "Admin", "admin@example.com", "correct-password"); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := repo.AuthenticateUser(context.Background(), "admin@example.com", "wrong-password"); err == nil {
			t.Fatalf("attempt %d unexpectedly authenticated", attempt+1)
		}
	}
	if _, err := repo.AuthenticateUser(context.Background(), "admin@example.com", "correct-password"); err == nil {
		t.Fatal("locked account unexpectedly authenticated")
	}
	now = now.Add(16 * time.Minute)
	if _, err := repo.AuthenticateUser(context.Background(), "admin@example.com", "correct-password"); err != nil {
		t.Fatalf("authentication after lock expiry returned error: %v", err)
	}
}

func TestOIDCFlowIsSingleUseAndMappedGroupsSynchronize(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, time.August, 28, 10, 0, 0, 0, time.UTC)
	repo.clock = func() time.Time { return now }
	state := "browser-state"
	flow := domain.OIDCFlow{StateHash: sessionTokenHash(state), Nonce: "nonce", CodeVerifier: "verifier", ExpiresAt: now.Add(time.Minute)}
	if err := repo.CreateOIDCFlow(context.Background(), flow); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ConsumeOIDCFlow(context.Background(), state); err != nil {
		t.Fatalf("first consume returned error: %v", err)
	}
	if _, err := repo.ConsumeOIDCFlow(context.Background(), state); err == nil {
		t.Fatal("OIDC state was reusable")
	}
	group, err := repo.CreateAccessGroup(context.Background(), domain.AccessGroup{Name: "Platform", OktaGroupName: "okta-platform"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := repo.ResolveOIDCUser(context.Background(), domain.OIDCIdentity{Provider: "okta", Subject: "subject", Email: "user@example.com", DisplayName: "OIDC User", Role: "member"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SyncUserAccessGroups(context.Background(), user.ID, []string{"OKTA-PLATFORM"}); err != nil {
		t.Fatal(err)
	}
	groupIDs, err := repo.ListUserAccessGroupIDs(context.Background(), user.ID)
	if err != nil || len(groupIDs) != 1 || groupIDs[0] != group.ID {
		t.Fatalf("mapped groups = %#v, err=%v", groupIDs, err)
	}
	if err := repo.SyncUserAccessGroups(context.Background(), user.ID, nil); err != nil {
		t.Fatal(err)
	}
	groupIDs, _ = repo.ListUserAccessGroupIDs(context.Background(), user.ID)
	if len(groupIDs) != 0 {
		t.Fatalf("stale mapped groups remain: %#v", groupIDs)
	}
}
