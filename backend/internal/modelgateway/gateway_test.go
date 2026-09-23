package modelgateway

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/system-design-evaluator/backend/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestProviderClientRejectsPrivateAddressAtDialTime(t *testing.T) {
	client := newProviderHTTPClient(0, false, func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "provider.example" {
			t.Errorf("unexpected lookup for %q", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	})
	_, err := client.Get("https://provider.example/v1/chat/completions")
	if err == nil || !strings.Contains(err.Error(), "private network address") {
		t.Fatalf("expected private DNS answer to be blocked at dial time, got %v", err)
	}
}

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

func TestGoogleCompletionUsesGeminiRequestAndAPIKeyHeader(t *testing.T) {
	gateway := &HTTPGateway{
		AllowPrivateURLs: true,
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://provider.local/v1beta/models/gemini-test:generateContent" {
				t.Fatalf("unexpected endpoint %s", request.URL)
			}
			if request.Header.Get("x-goog-api-key") != "google-secret" {
				t.Fatal("missing Google provider API key")
			}
			if request.Header.Get("Authorization") != "" {
				t.Fatal("Google API key must not be sent as bearer authorization")
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["systemInstruction"] == nil || body["contents"] == nil {
				t.Fatalf("unexpected Gemini request body %#v", body)
			}
			generationConfig, ok := body["generationConfig"].(map[string]any)
			if !ok || generationConfig["responseMimeType"] != "application/json" {
				t.Fatalf("expected JSON response configuration, got %#v", body["generationConfig"])
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"answer\":\"ok\"}"}]}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	result, err := gateway.Complete(context.Background(), domain.AIProviderConfig{
		Enabled: true, Provider: "google", Model: "gemini-test", BaseURL: "http://provider.local/v1beta", APIKey: "google-secret",
	}, Request{System: "system", Messages: []Message{{Role: "user", Content: "question"}}, JSONResponse: true, MaxTokens: 200})
	if err != nil {
		t.Fatal(err)
	}
	if result != `{"answer":"ok"}` {
		t.Fatalf("unexpected completion %q", result)
	}
}

func TestGoogleProviderDefaults(t *testing.T) {
	endpoint, err := ProviderBaseURL("google", "")
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "https://generativelanguage.googleapis.com/v1beta" {
		t.Fatalf("unexpected Google endpoint %q", endpoint)
	}
}

func TestAnthropicCompletionUsesProviderProtocol(t *testing.T) {
	gateway := &HTTPGateway{
		AllowPrivateURLs: true,
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://provider.local/v1/messages" {
				t.Fatalf("unexpected endpoint %s", request.URL)
			}
			if request.Header.Get("x-api-key") != "anthropic-secret" || request.Header.Get("anthropic-version") == "" {
				t.Fatalf("missing Anthropic authentication headers: %#v", request.Header)
			}
			if request.Header.Get("Authorization") != "" {
				t.Fatal("Anthropic key must not be sent as bearer authorization")
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["model"] != "claude-test" || body["system"] != "architecture rules" || body["max_tokens"] != float64(900) {
				t.Fatalf("unexpected Anthropic request body %#v", body)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"content":[{"type":"tool_use","text":"ignore"},{"type":"text","text":"  grounded answer  "}]}`)), Header: make(http.Header)}, nil
		})},
	}
	result, err := gateway.Complete(context.Background(), domain.AIProviderConfig{
		Enabled: true, Provider: "anthropic", Model: "claude-test", BaseURL: "http://provider.local/v1", APIKey: "anthropic-secret",
	}, Request{System: "architecture rules", Messages: []Message{{Role: "user", Content: "review"}}, MaxTokens: 900})
	if err != nil || result != "grounded answer" {
		t.Fatalf("Anthropic completion result=%q err=%v", result, err)
	}
}

func TestProviderEndpointValidationRejectsUnsafeDestinations(t *testing.T) {
	for _, scenario := range []struct {
		name, endpoint, want string
	}{
		{name: "missing host", endpoint: "https:///v1", want: "invalid"},
		{name: "unsupported scheme", endpoint: "file://provider/v1", want: "http or https"},
		{name: "plain HTTP", endpoint: "http://provider.example/v1", want: "must use https"},
		{name: "loopback", endpoint: "https://127.0.0.1/v1", want: "private network"},
		{name: "private IPv4", endpoint: "https://10.2.3.4/v1", want: "private network"},
		{name: "link local", endpoint: "https://169.254.1.2/v1", want: "private network"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if err := ValidateEndpoint(context.Background(), scenario.endpoint, false); err == nil || !strings.Contains(err.Error(), scenario.want) {
				t.Fatalf("ValidateEndpoint(%q) error=%v, want %q", scenario.endpoint, err, scenario.want)
			}
		})
	}
	if err := ValidateEndpoint(context.Background(), "http://127.0.0.1:9000/v1", true); err != nil {
		t.Fatalf("explicit private-provider override should allow local endpoint: %v", err)
	}
}

func TestProviderRejectionIncludesSafeHTTPStatus(t *testing.T) {
	gateway := &HTTPGateway{
		AllowPrivateURLs: true,
		Client: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"secret provider detail"}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	_, err := gateway.Complete(context.Background(), domain.AIProviderConfig{
		Enabled: true, Provider: "custom", Model: "model-a", BaseURL: "http://provider.local/v1", APIKey: "secret",
	}, Request{Messages: []Message{{Role: "user", Content: "question"}}})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("expected actionable safe status, got %v", err)
	}
	if strings.Contains(err.Error(), "secret provider detail") {
		t.Fatalf("provider response body must not be exposed: %v", err)
	}
}

func TestTransientProviderFailureIsRetried(t *testing.T) {
	attempts := 0
	gateway := &HTTPGateway{
		AllowPrivateURLs: true,
		Client: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       io.NopCloser(strings.NewReader(`{"error":"temporarily overloaded"}`)),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	result, err := gateway.Complete(context.Background(), domain.AIProviderConfig{
		Enabled: true, Provider: "custom", Model: "model-a", BaseURL: "http://provider.local/v1", APIKey: "secret",
	}, Request{Messages: []Message{{Role: "user", Content: "question"}}})
	if err != nil || result != "ok" || attempts != 2 {
		t.Fatalf("transient provider request was not retried: result=%q attempts=%d err=%v", result, attempts, err)
	}
}
