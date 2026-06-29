package integrations

import (
	"context"
	"time"
)

type IntegrationConfig struct {
	ID          string
	Kind        string
	DisplayName string
	Enabled     bool
	BaseURL     string
	Auth        AuthConfig
	Filters     FilterConfig
	Timeout     time.Duration
	Settings    map[string]string
}

type AuthConfig struct {
	Mode        string
	BearerToken string
	Username    string
	Password    string
	Headers     map[string]string
}

type FilterConfig struct {
	NamespaceAllowlist []string
	NamespaceBlocklist []string
	ServiceAllowlist   []string
	ServiceBlocklist   []string
	LabelAllowlist     []string
	LabelBlocklist     []string
	EndpointBlocklist  []string
	ConnectionTypes    []string
	Environment        string
	MinimumRequests    float64
}

type FetchTopologyRequest struct {
	Integration IntegrationConfig
	WindowStart time.Time
	WindowEnd   time.Time
}

type RawTopologyResult struct {
	Source      string
	FetchedAt   time.Time
	ContentType string
	Payload     []byte
	Warnings    []string
}

type TopologyProvider interface {
	Kind() string
	TestConnection(ctx context.Context, config IntegrationConfig) error
	Fetch(ctx context.Context, request FetchTopologyRequest) (RawTopologyResult, error)
}

func (config IntegrationConfig) Redacted() IntegrationConfig {
	config.Auth.BearerToken = ""
	config.Auth.Password = ""
	if config.Auth.Headers != nil {
		headers := make(map[string]string, len(config.Auth.Headers))
		for key := range config.Auth.Headers {
			headers[key] = ""
		}
		config.Auth.Headers = headers
	}
	return config
}
