package prometheus

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/system-design-evaluator/backend/internal/integrations"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestClientQueryAppliesAuthAndEncodesQuery(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/query" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); !strings.Contains(got, "up") {
			t.Fatalf("query = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":"success","data":{"result":[]}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	body, err := NewClient(httpClient).Query(context.Background(), integrations.IntegrationConfig{
		BaseURL: "http://prometheus.example",
		Auth: integrations.AuthConfig{
			Mode:        "bearer",
			BearerToken: "token",
		},
	}, "up")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if !strings.Contains(string(body), `"success"`) {
		t.Fatalf("body = %s", body)
	}
}

func TestClientRejectsInvalidURL(t *testing.T) {
	_, err := NewClient(nil).Query(context.Background(), integrations.IntegrationConfig{BaseURL: "file:///tmp/prometheus"}, "up")
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
}

func TestClientEnforcesResponseLimit(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("123456")),
			Header:     make(http.Header),
		}, nil
	})}
	client := NewClient(httpClient)
	client.responseLimitBytes = 5
	_, err := client.Query(context.Background(), integrations.IntegrationConfig{BaseURL: "http://prometheus.example"}, "up")
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("error = %v, want response limit error", err)
	}
}
