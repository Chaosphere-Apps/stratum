package integrations

import "errors"

var (
	ErrNilProvider       = errors.New("integration provider is nil")
	ErrEmptyProviderKind = errors.New("integration provider kind is empty")
	ErrDuplicateProvider = errors.New("integration provider already registered")
)
