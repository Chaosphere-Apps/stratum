package authn

import "testing"

func TestValidatePasswordUsesPublishedPolicy(t *testing.T) {
	policy := CurrentPasswordPolicy()
	if policy.MinimumLength != MinimumPasswordLength {
		t.Fatalf("published minimum = %d, want %d", policy.MinimumLength, MinimumPasswordLength)
	}
	if err := ValidatePassword("1234567"); err == nil {
		t.Fatal("password shorter than the published minimum was accepted")
	}
	if err := ValidatePassword("12345678"); err != nil {
		t.Fatalf("password meeting the published minimum was rejected: %v", err)
	}
}

func TestValidatePasswordCountsCharactersNotBytes(t *testing.T) {
	if err := ValidatePassword("🙂🙂🙂🙂"); err == nil {
		t.Fatal("four multi-byte characters must not satisfy an eight-character policy")
	}
}
