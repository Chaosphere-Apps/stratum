package prometheus

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/system-design-evaluator/backend/internal/integrations"
)

type Client struct {
	httpClient         *http.Client
	responseLimitBytes int64
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, responseLimitBytes: defaultResponseLimitBytes}
}

func (client *Client) Query(ctx context.Context, config integrations.IntegrationConfig, query string) ([]byte, error) {
	endpoint, err := prometheusQueryURL(config.BaseURL, query)
	if err != nil {
		return nil, err
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(queryCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyAuth(request, config.Auth)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("prometheus query returned status %d", response.StatusCode)
	}
	limit := client.responseLimitBytes
	if limit <= 0 {
		limit = defaultResponseLimitBytes
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("prometheus response exceeded %d bytes", limit)
	}
	return body, nil
}

func prometheusQueryURL(baseURL string, query string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("prometheus URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("prometheus URL host is required")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/v1/query"
	values := parsed.Query()
	values.Set("query", query)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func applyAuth(request *http.Request, auth integrations.AuthConfig) {
	switch strings.ToLower(strings.TrimSpace(auth.Mode)) {
	case "bearer":
		if auth.BearerToken != "" {
			request.Header.Set("Authorization", "Bearer "+auth.BearerToken)
		}
	case "basic":
		if auth.Username != "" || auth.Password != "" {
			request.SetBasicAuth(auth.Username, auth.Password)
		}
	}
	for key, value := range auth.Headers {
		if strings.TrimSpace(key) != "" && value != "" {
			request.Header.Set(key, value)
		}
	}
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
