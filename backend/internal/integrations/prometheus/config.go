package prometheus

import (
	"strings"
	"time"

	"github.com/system-design-evaluator/backend/internal/integrations"
)

const (
	Kind = "prometheus"

	settingRequestTotalMetric  = "requestTotalMetric"
	settingRequestFailedMetric = "requestFailedMetric"
	settingServerLatencyMetric = "serverLatencyMetric"
	settingClientLatencyMetric = "clientLatencyMetric"
	settingQueryWindow         = "queryWindow"
	defaultRequestTotalMetric  = "traces_service_graph_request_total"
	defaultRequestFailedMetric = "traces_service_graph_request_failed_total"
	defaultServerLatencyMetric = "traces_service_graph_request_server_seconds_bucket"
	defaultClientLatencyMetric = "traces_service_graph_request_client_seconds_bucket"
	defaultQueryWindow         = 5 * time.Minute
	defaultTimeout             = 10 * time.Second
	defaultResponseLimitBytes  = 8 << 20
)

type MetricConfig struct {
	RequestTotalMetric  string
	RequestFailedMetric string
	ServerLatencyMetric string
	ClientLatencyMetric string
	QueryWindow         time.Duration
}

func MetricsFromIntegration(config integrations.IntegrationConfig) MetricConfig {
	window := defaultQueryWindow
	if parsed, err := time.ParseDuration(strings.TrimSpace(config.Settings[settingQueryWindow])); err == nil && parsed > 0 {
		window = parsed
	}
	return MetricConfig{
		RequestTotalMetric:  firstNonEmpty(config.Settings[settingRequestTotalMetric], defaultRequestTotalMetric),
		RequestFailedMetric: firstNonEmpty(config.Settings[settingRequestFailedMetric], defaultRequestFailedMetric),
		ServerLatencyMetric: firstNonEmpty(config.Settings[settingServerLatencyMetric], defaultServerLatencyMetric),
		ClientLatencyMetric: firstNonEmpty(config.Settings[settingClientLatencyMetric], defaultClientLatencyMetric),
		QueryWindow:         window,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
