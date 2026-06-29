package prometheus

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/integrations"
)

func TestParseRawServiceGraph(t *testing.T) {
	start := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	bundle := QueryBundle{
		RequestTotal: []byte(`{
			"status": "success",
			"data": {"resultType": "vector", "result": [
				{"metric": {"client": "API", "server": "Worker", "connection_type": "http"}, "value": [1770000000, "120"]}
			]}
		}`),
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	graph, err := ParseRawServiceGraph(integrations.RawTopologyResult{
		Source:  Kind,
		Payload: payload,
	}, integrations.FetchTopologyRequest{
		Integration: integrations.IntegrationConfig{Filters: integrations.FilterConfig{}},
		WindowStart: start,
		WindowEnd:   start.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ParseRawServiceGraph returned error: %v", err)
	}
	if len(graph.Calls) != 1 {
		t.Fatalf("calls = %#v, want one", graph.Calls)
	}
	if graph.Calls[0].RequestRate == nil || *graph.Calls[0].RequestRate != 1 {
		t.Fatalf("request rate = %#v, want 1 rps", graph.Calls[0].RequestRate)
	}
}
