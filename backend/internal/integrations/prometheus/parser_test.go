package prometheus

import (
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/integrations"
)

func TestParseServiceGraphMergesPrometheusVectors(t *testing.T) {
	start := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	graph, err := ParseServiceGraph(QueryBundle{
		RequestTotal: []byte(`{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{"metric": {"client": "Checkout", "server": "Postgres", "connection_type": "database", "db.system": "postgresql"}, "value": [1770000000, "300"]},
					{"metric": {"client": "Checkout", "server": "Kafka", "connection_type": "messaging_system", "messaging.system": "kafka"}, "value": [1770000000, "60"]}
				]
			}
		}`),
		RequestFailed: []byte(`{
			"status": "success",
			"data": {"resultType": "vector", "result": [
				{"metric": {"client": "Checkout", "server": "Postgres", "connection_type": "database"}, "value": [1770000000, "6"]}
			]}
		}`),
		ServerP95: []byte(`{
			"status": "success",
			"data": {"resultType": "vector", "result": [
				{"metric": {"client": "Checkout", "server": "Postgres", "connection_type": "database"}, "value": [1770000000, "0.042"]}
			]}
		}`),
	}, "prometheus", start, end, integrations.FilterConfig{})
	if err != nil {
		t.Fatalf("ParseServiceGraph returned error: %v", err)
	}
	if len(graph.Calls) != 2 {
		t.Fatalf("calls = %#v, want two", graph.Calls)
	}
	var databaseCallFound bool
	for _, call := range graph.Calls {
		if call.From == "Checkout" && call.To == "Postgres" {
			databaseCallFound = true
			if call.RequestRate == nil || *call.RequestRate != 1 {
				t.Fatalf("request rate = %#v, want 1 rps", call.RequestRate)
			}
			if call.ErrorRate == nil || *call.ErrorRate != 0.02 {
				t.Fatalf("error rate = %#v, want 0.02 rps", call.ErrorRate)
			}
			if call.P95LatencyMs == nil || *call.P95LatencyMs != 42 {
				t.Fatalf("p95 latency = %#v, want 42 ms", call.P95LatencyMs)
			}
		}
	}
	if !databaseCallFound {
		t.Fatalf("database call not found in %#v", graph.Calls)
	}
	if len(graph.Services) != 3 {
		t.Fatalf("services = %#v, want three", graph.Services)
	}
}

func TestParseServiceGraphFiltersEndpointsAndWarnsForMissingLabels(t *testing.T) {
	start := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	graph, err := ParseServiceGraph(QueryBundle{
		RequestTotal: []byte(`{
			"status": "success",
			"data": {"resultType": "vector", "result": [
				{"metric": {"client": "Gateway", "server": "Health", "connection_type": "http", "http.route": "/healthz"}, "value": [1770000000, "300"]},
				{"metric": {"client": "Gateway", "connection_type": "http"}, "value": [1770000000, "300"]}
			]}
		}`),
	}, "prometheus", start, start.Add(5*time.Minute), integrations.FilterConfig{EndpointBlocklist: []string{"/health"}})
	if err != nil {
		t.Fatalf("ParseServiceGraph returned error: %v", err)
	}
	if len(graph.Calls) != 0 {
		t.Fatalf("calls = %#v, want none", graph.Calls)
	}
	if len(graph.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want one missing-label warning", graph.Warnings)
	}
}
