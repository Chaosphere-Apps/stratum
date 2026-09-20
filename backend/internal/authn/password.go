package authn

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const MinimumPasswordLength = 8

// PasswordPolicy is the public contract used by password-creation clients.
// Add new fields here when password validation gains additional constraints.
type PasswordPolicy struct {
	MinimumLength int `json:"minimumLength"`
}

func CurrentPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{MinimumLength: MinimumPasswordLength}
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(strings.TrimSpace(password)) < MinimumPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinimumPasswordLength)
	}
	return nil
}
