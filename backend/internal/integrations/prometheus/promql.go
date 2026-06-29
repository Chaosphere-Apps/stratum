package prometheus

import (
	"fmt"
	"strings"
	"time"
)

type QuerySet struct {
	RequestTotal  string
	RequestFailed string
	ServerP95     string
	ClientP95     string
}

func BuildServiceGraphQueries(metrics MetricConfig) QuerySet {
	window := promDuration(metrics.QueryWindow)
	return QuerySet{
		RequestTotal: fmt.Sprintf(
			`sum by (client, server, connection_type) (increase(%s[%s]))`,
			sanitizeMetricName(metrics.RequestTotalMetric),
			window,
		),
		RequestFailed: fmt.Sprintf(
			`sum by (client, server, connection_type) (increase(%s[%s]))`,
			sanitizeMetricName(metrics.RequestFailedMetric),
			window,
		),
		ServerP95: fmt.Sprintf(
			`histogram_quantile(0.95, sum by (le, client, server, connection_type) (rate(%s[%s])))`,
			sanitizeMetricName(metrics.ServerLatencyMetric),
			window,
		),
		ClientP95: fmt.Sprintf(
			`histogram_quantile(0.95, sum by (le, client, server, connection_type) (rate(%s[%s])))`,
			sanitizeMetricName(metrics.ClientLatencyMetric),
			window,
		),
	}
}

func promDuration(duration time.Duration) string {
	if duration <= 0 {
		duration = defaultQueryWindow
	}
	seconds := int(duration.Seconds())
	if seconds%86400 == 0 {
		return fmt.Sprintf("%dd", seconds/86400)
	}
	if seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func sanitizeMetricName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultRequestTotalMetric
	}
	return name
}
