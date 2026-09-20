package modelgateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/system-design-evaluator/backend/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestOpenAICompletionUsesStructuredRequest(t *testing.T) {
	gateway := &HTTPGateway{
		AllowPrivateURLs: true,
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://provider.local/v1/chat/completions" {
				t.Fatalf("unexpected endpoint %s", request.URL)
			}
			if request.Header.Get("Authorization") != "Bearer secret" {
				t.Fatal("missing provider authorization")
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["response_format"] == nil || body["model"] != "model-a" {
				t.Fatalf("unexpected request body %#v", body)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"{\"answer\":\"ok\"}"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	result, err := gateway.Complete(context.Background(), domain.AIProviderConfig{
		Enabled: true, Provider: "custom", Model: "model-a", BaseURL: "http://provider.local/v1", APIKey: "secret",
	}, Request{System: "system", Messages: []Message{{Role: "user", Content: "question"}}, JSONResponse: true})
	if err != nil {
		t.Fatal(err)
	}
	if result != `{"answer":"ok"}` {
		t.Fatalf("unexpected completion %q", result)
	}
}

func TestCompletionRejectsUnconfiguredProviderAndPrivateEndpoint(t *testing.T) {
	gateway := New(false)
	if _, err := gateway.Complete(context.Background(), domain.AIProviderConfig{}, Request{}); err == nil {
		t.Fatal("expected unconfigured provider to fail")
	}
	_, err := gateway.Complete(context.Background(), domain.AIProviderConfig{
		Enabled: true, Provider: "custom", Model: "model-a", BaseURL: "http://127.0.0.1:9000", APIKey: "secret",
	}, Request{})
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected insecure private endpoint rejection, got %v", err)
	}
}
