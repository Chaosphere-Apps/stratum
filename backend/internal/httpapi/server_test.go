package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsAllowedOrigin(t *testing.T) {
	allowed := []string{"https://stratum.example.com", "http://localhost:*"}

	if !isAllowedOrigin("https://stratum.example.com", allowed) {
		t.Fatal("expected exact production origin to be allowed")
	}
	if !isAllowedOrigin("http://localhost:5173", allowed) {
		t.Fatal("expected wildcard localhost port to be allowed")
	}
	if isAllowedOrigin("https://evil.example.com", allowed) {
		t.Fatal("unexpectedly allowed unrelated origin")
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Workspace","extra":true}`))
	recorder := httptest.NewRecorder()

	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(recorder, request, 1024, &body); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestValidateProviderEndpointRejectsPrivateAddress(t *testing.T) {
	if err := validateProviderEndpoint("http://127.0.0.1:11434", false); err == nil {
		t.Fatal("expected private provider URL to be rejected")
	}
	if err := validateProviderEndpoint("http://127.0.0.1:11434", true); err != nil {
		t.Fatalf("expected private provider URL to be allowed when explicitly configured: %v", err)
	}
}
