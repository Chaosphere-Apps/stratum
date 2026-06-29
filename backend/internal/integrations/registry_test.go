package integrations

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeProvider struct {
	kind string
}

func (provider fakeProvider) Kind() string {
	return provider.kind
}

func (provider fakeProvider) TestConnection(context.Context, IntegrationConfig) error {
	return nil
}

func (provider fakeProvider) Fetch(context.Context, FetchTopologyRequest) (RawTopologyResult, error) {
	return RawTopologyResult{}, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	provider := fakeProvider{kind: "prometheus"}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	got, ok := registry.Get("prometheus")
	if !ok {
		t.Fatal("expected provider to be registered")
	}
	if got.Kind() != "prometheus" {
		t.Fatalf("provider kind = %q, want prometheus", got.Kind())
	}
}

func TestRegistryRejectsInvalidProviders(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(nil); !errors.Is(err, ErrNilProvider) {
		t.Fatalf("nil provider error = %v, want %v", err, ErrNilProvider)
	}
	if err := registry.Register(fakeProvider{}); !errors.Is(err, ErrEmptyProviderKind) {
		t.Fatalf("empty kind error = %v, want %v", err, ErrEmptyProviderKind)
	}
}

func TestRegistryRejectsDuplicateProvider(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(fakeProvider{kind: "prometheus"}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if err := registry.Register(fakeProvider{kind: "prometheus"}); !errors.Is(err, ErrDuplicateProvider) {
		t.Fatalf("duplicate error = %v, want %v", err, ErrDuplicateProvider)
	}
}

func TestIntegrationConfigRedacted(t *testing.T) {
	config := IntegrationConfig{
		Auth: AuthConfig{
			BearerToken: "secret-token",
			Password:    "secret-password",
			Headers: map[string]string{
				"X-Api-Key": "secret-header",
			},
		},
	}
	redacted := config.Redacted()
	if redacted.Auth.BearerToken != "" || redacted.Auth.Password != "" || redacted.Auth.Headers["X-Api-Key"] != "" {
		t.Fatalf("Redacted did not remove secret values: %#v", redacted.Auth)
	}
	if !reflect.DeepEqual(config.Auth.Headers, map[string]string{"X-Api-Key": "secret-header"}) {
		t.Fatalf("Redacted mutated original config headers: %#v", config.Auth.Headers)
	}
}
