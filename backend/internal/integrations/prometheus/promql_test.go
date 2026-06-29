package prometheus

import (
	"strings"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/integrations"
)

func TestBuildServiceGraphQueries(t *testing.T) {
	queries := BuildServiceGraphQueries(MetricConfig{
		RequestTotalMetric:  "custom_request_total",
		RequestFailedMetric: "custom_request_failed_total",
		ServerLatencyMetric: "custom_server_bucket",
		ClientLatencyMetric: "custom_client_bucket",
		QueryWindow:         15 * time.Minute,
	})
	if !strings.Contains(queries.RequestTotal, "increase(custom_request_total[15m])") {
		t.Fatalf("request total query = %q", queries.RequestTotal)
	}
	if !strings.Contains(queries.RequestFailed, "increase(custom_request_failed_total[15m])") {
		t.Fatalf("request failed query = %q", queries.RequestFailed)
	}
	if !strings.Contains(queries.ServerP95, "histogram_quantile(0.95") || !strings.Contains(queries.ServerP95, "custom_server_bucket[15m]") {
		t.Fatalf("server p95 query = %q", queries.ServerP95)
	}
	if !strings.Contains(queries.ClientP95, "custom_client_bucket[15m]") {
		t.Fatalf("client p95 query = %q", queries.ClientP95)
	}
}

func TestMetricsFromIntegrationUsesDefaultsAndOverrides(t *testing.T) {
	metrics := MetricsFromIntegration(integrations.IntegrationConfig{
		Settings: map[string]string{
			settingRequestTotalMetric: "service_graph_calls_total",
			settingQueryWindow:        "7m",
		},
	})
	if metrics.RequestTotalMetric != "service_graph_calls_total" {
		t.Fatalf("request metric = %q", metrics.RequestTotalMetric)
	}
	if metrics.RequestFailedMetric != defaultRequestFailedMetric {
		t.Fatalf("failed metric = %q", metrics.RequestFailedMetric)
	}
	if metrics.QueryWindow != 7*time.Minute {
		t.Fatalf("query window = %s", metrics.QueryWindow)
	}
}
