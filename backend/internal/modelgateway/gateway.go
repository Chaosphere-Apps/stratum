package modelgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/system-design-evaluator/backend/internal/domain"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	System       string
	Messages     []Message
	JSONResponse bool
	MaxTokens    int
}

type Gateway interface {
	Complete(context.Context, domain.AIProviderConfig, Request) (string, error)
}

type ProviderHTTPError struct {
	StatusCode int
}

func (e *ProviderHTTPError) Error() string {
	return fmt.Sprintf("provider rejected the request (HTTP %d)", e.StatusCode)
}

type HTTPGateway struct {
	Client           *http.Client
	AllowPrivateURLs bool
}

func New(allowPrivateURLs bool) *HTTPGateway {
	return &HTTPGateway{
		Client:           NewProviderHTTPClient(30*time.Second, allowPrivateURLs),
		AllowPrivateURLs: allowPrivateURLs,
	}
}

// NewProviderHTTPClient keeps provider credentials on the configured endpoint and
// checks the address used for each connection, including after DNS changes.
func NewProviderHTTPClient(timeout time.Duration, allowPrivateURLs bool) *http.Client {
	return newProviderHTTPClient(timeout, allowPrivateURLs, net.DefaultResolver.LookupIPAddr)
}

func newProviderHTTPClient(timeout time.Duration, allowPrivateURLs bool, lookup func(context.Context, string) ([]net.IPAddr, error)) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// An environment proxy could bypass the destination address check.
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		if allowPrivateURLs {
			return (&net.Dialer{}).DialContext(ctx, network, address)
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("provider address is invalid")
		}
		var addresses []net.IPAddr
		if ip := net.ParseIP(host); ip != nil {
			addresses = []net.IPAddr{{IP: ip}}
		} else {
			addresses, err = lookup(ctx, host)
			if err != nil || len(addresses) == 0 {
				return nil, errors.New("provider host could not be resolved")
			}
		}
		for _, candidate := range addresses {
			if candidate.IP == nil || isPrivateAddress(candidate.IP) {
				return nil, errors.New("provider host resolves to a private network address")
			}
		}
		var dialErr error
		for _, candidate := range addresses {
			connection, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			dialErr = err
		}
		return nil, dialErr
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (g *HTTPGateway) Complete(ctx context.Context, config domain.AIProviderConfig, input Request) (string, error) {
	if !config.Enabled || strings.TrimSpace(config.APIKey) == "" {
		return "", errors.New("AI provider is not configured")
	}
	endpoint, err := ProviderBaseURL(config.Provider, config.BaseURL)
	if err != nil {
		return "", err
	}
	if err := ValidateEndpoint(ctx, endpoint, g.AllowPrivateURLs); err != nil {
		return "", err
	}
	if strings.EqualFold(config.Provider, "anthropic") {
		return g.completeAnthropic(ctx, endpoint, config, input)
	}
	if strings.EqualFold(config.Provider, "google") {
		return g.completeGoogle(ctx, endpoint, config, input)
	}
	return g.completeOpenAI(ctx, endpoint, config, input)
}

func (g *HTTPGateway) completeGoogle(ctx context.Context, endpoint string, config domain.AIProviderConfig, input Request) (string, error) {
	contents := make([]map[string]any, 0, len(input.Messages))
	for _, message := range input.Messages {
		role := "user"
		if strings.EqualFold(message.Role, "assistant") || strings.EqualFold(message.Role, "model") {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]string{{"text": message.Content}},
		})
	}
	generationConfig := map[string]any{"temperature": 0.1}
	if input.JSONResponse {
		generationConfig["responseMimeType"] = "application/json"
	}
	if input.MaxTokens > 0 {
		generationConfig["maxOutputTokens"] = input.MaxTokens
	}
	payload := map[string]any{
		"contents":         contents,
		"generationConfig": generationConfig,
	}
	if strings.TrimSpace(input.System) != "" {
		payload["systemInstruction"] = map[string]any{
			"parts": []map[string]string{{"text": input.System}},
		}
	}
	model := strings.TrimPrefix(strings.TrimSpace(config.Model), "models/")
	if model == "" {
		return "", errors.New("AI model is required")
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	path := strings.TrimRight(endpoint, "/") + "/models/" + url.PathEscape(model) + ":generateContent"
	if err := g.doJSON(ctx, path, config, payload, &parsed); err != nil {
		return "", err
	}
	for _, candidate := range parsed.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return strings.TrimSpace(part.Text), nil
			}
		}
	}
	return "", errors.New("provider returned an empty response")
}

func (g *HTTPGateway) completeOpenAI(ctx context.Context, endpoint string, config domain.AIProviderConfig, input Request) (string, error) {
	messages := make([]Message, 0, len(input.Messages)+1)
	if strings.TrimSpace(input.System) != "" {
		messages = append(messages, Message{Role: "system", Content: input.System})
	}
	messages = append(messages, input.Messages...)
	payload := map[string]any{
		"model":       strings.TrimSpace(config.Model),
		"temperature": 0.1,
		"messages":    messages,
	}
	if input.JSONResponse {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	if input.MaxTokens > 0 {
		payload["max_tokens"] = input.MaxTokens
	}
	var parsed struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := g.doJSON(ctx, endpoint+"/chat/completions", config, payload, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("provider returned an empty response")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (g *HTTPGateway) completeAnthropic(ctx context.Context, endpoint string, config domain.AIProviderConfig, input Request) (string, error) {
	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2200
	}
	payload := map[string]any{
		"model":       strings.TrimSpace(config.Model),
		"max_tokens":  maxTokens,
		"temperature": 0.1,
		"system":      input.System,
		"messages":    input.Messages,
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := g.doJSON(ctx, endpoint+"/messages", config, payload, &parsed); err != nil {
		return "", err
	}
	for _, content := range parsed.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			return strings.TrimSpace(content.Text), nil
		}
	}
	return "", errors.New("provider returned an empty response")
}

func (g *HTTPGateway) doJSON(ctx context.Context, endpoint string, config domain.AIProviderConfig, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := g.Client
	if client == nil {
		client = NewProviderHTTPClient(30*time.Second, g.AllowPrivateURLs)
	}
	for attempt := 0; attempt < 3; attempt++ {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if requestErr != nil {
			return errors.New("provider URL is invalid")
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")
		if strings.EqualFold(config.Provider, "anthropic") {
			request.Header.Set("x-api-key", strings.TrimSpace(config.APIKey))
			request.Header.Set("anthropic-version", "2023-06-01")
		} else if strings.EqualFold(config.Provider, "google") {
			request.Header.Set("x-goog-api-key", strings.TrimSpace(config.APIKey))
		} else {
			request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(config.APIKey))
		}
		response, requestErr := client.Do(request)
		if requestErr != nil {
			if attempt < 2 && waitForProviderRetry(ctx, attempt) {
				continue
			}
			return errors.New("provider could not be reached")
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			status := response.StatusCode
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
			_ = response.Body.Close()
			if isTransientProviderStatus(status) && attempt < 2 && waitForProviderRetry(ctx, attempt) {
				continue
			}
			return &ProviderHTTPError{StatusCode: status}
		}
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(target)
		_ = response.Body.Close()
		if decodeErr != nil {
			return errors.New("provider returned invalid JSON")
		}
		return nil
	}
	return errors.New("provider could not be reached")
}

func isTransientProviderStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func waitForProviderRetry(ctx context.Context, attempt int) bool {
	timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func ProviderBaseURL(provider string, baseURL string) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint != "" {
		return endpoint, nil
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "https://api.openai.com/v1", nil
	case "anthropic":
		return "https://api.anthropic.com/v1", nil
	case "openrouter":
		return "https://openrouter.ai/api/v1", nil
	case "google":
		return "https://generativelanguage.googleapis.com/v1beta", nil
	default:
		return "", errors.New("base URL is required for custom providers")
	}
}

func ValidateEndpoint(ctx context.Context, endpoint string, allowPrivateURLs bool) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" {
		return errors.New("provider URL is invalid")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("provider URL must use http or https")
	}
	if parsed.Scheme != "https" && !allowPrivateURLs {
		return errors.New("custom provider URL must use https")
	}
	if allowPrivateURLs {
		return nil
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil && isPrivateAddress(ip) {
		return errors.New("custom provider URL cannot target a private network address")
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return errors.New("provider host could not be resolved")
	}
	for _, address := range addresses {
		if isPrivateAddress(address.IP) {
			return errors.New("custom provider URL cannot resolve to a private network address")
		}
	}
	return nil
}

func isPrivateAddress(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}
