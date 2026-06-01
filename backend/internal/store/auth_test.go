package store

import (
	"strings"
	"testing"
)

func TestPasswordHashAndVerifyBranches(t *testing.T) {
	if _, err := hashPassword("short"); err == nil {
		t.Fatal("expected short password to be rejected")
	}

	encoded, err := hashPassword("  password123  ")
	if err != nil {
		t.Fatalf("hashPassword returned error: %v", err)
	}
	if !verifyPassword("password123", encoded) {
		t.Fatal("expected trimmed password to verify")
	}
	if !verifyPassword("  password123  ", encoded) {
		t.Fatal("expected verifier to trim candidate password")
	}
	if verifyPassword("wrong-password", encoded) {
		t.Fatal("wrong password unexpectedly verified")
	}

	for name, hash := range map[string]string{
		"malformed":       "not-a-real-hash",
		"wrong scheme":    strings.Replace(encoded, "pbkdf2-sha256", "argon2id", 1),
		"bad iterations":  strings.Replace(encoded, "210000", "99999", 1),
		"invalid salt":    replaceHashPart(encoded, 2, "*not-base64*"),
		"invalid key":     replaceHashPart(encoded, 3, "*not-base64*"),
		"bad iteration":   replaceHashPart(encoded, 1, "not-a-number"),
		"missing segment": strings.Join(strings.Split(encoded, "$")[:3], "$"),
	} {
		t.Run(name, func(t *testing.T) {
			if verifyPassword("password123", hash) {
				t.Fatalf("hash %q unexpectedly verified", hash)
			}
		})
	}
}

func TestSessionTokenHashIsStableAndTrimmed(t *testing.T) {
	token, hashed, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken returned error: %v", err)
	}
	if token == "" || hashed == "" {
		t.Fatalf("token=%q hash=%q, want non-empty values", token, hashed)
	}
	if sessionTokenHash(token) != hashed {
		t.Fatal("session hash is not stable for the generated token")
	}
	if sessionTokenHash("  "+token+"  ") != hashed {
		t.Fatal("session hash should trim transport whitespace")
	}
}

func replaceHashPart(encoded string, index int, value string) string {
	parts := strings.Split(encoded, "$")
	parts[index] = value
	return strings.Join(parts, "$")
}
