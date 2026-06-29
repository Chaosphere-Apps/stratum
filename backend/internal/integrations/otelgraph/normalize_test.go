package otelgraph

import (
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/integrations"
)

func TestNormalizeCreatesStableNodesAndEdges(t *testing.T) {
	requests := 42.0
	graph := ObservedServiceGraph{
		Source:      "prometheus",
		WindowStart: time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 6, 12, 10, 5, 0, 0, time.UTC),
		Services: []ObservedService{
			{Name: "Checkout Service", Namespace: "prod", Labels: map[string]string{"team": "payments"}},
			{Name: "Postgres Primary", Labels: map[string]string{"db.system": "postgresql"}},
		},
		Calls: []ObservedCall{
			{
				From:           "Checkout Service",
				To:             "Postgres Primary",
				ConnectionType: "database",
				RequestRate:    &requests,
				Labels:         map[string]string{"http.route": "/checkout", "authorization": "secret"},
			},
		},
	}
	topology := Normalize(graph, integrations.FilterConfig{LabelBlocklist: []string{"authorization"}})
	if len(topology.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2: %#v", len(topology.Nodes), topology.Nodes)
	}
	if topology.Nodes[0].ExternalKey != "checkout-service" || topology.Nodes[1].ExternalKey != "postgres-primary" {
		t.Fatalf("unexpected node order/keys: %#v", topology.Nodes)
	}
	if len(topology.Edges) != 1 {
		t.Fatalf("edges = %d, want 1: %#v", len(topology.Edges), topology.Edges)
	}
	edge := topology.Edges[0]
	if edge.ExternalKey != "checkout-service->postgres-primary:database" {
		t.Fatalf("edge key = %q", edge.ExternalKey)
	}
	if _, ok := edge.Labels["authorization"]; ok {
		t.Fatalf("blocked label was retained: %#v", edge.Labels)
	}
	if edge.RequestRate == nil || *edge.RequestRate != requests {
		t.Fatalf("request rate = %#v, want %v", edge.RequestRate, requests)
	}
}

func TestNormalizeAppliesIntegrationFilters(t *testing.T) {
	requests := 1.0
	graph := ObservedServiceGraph{
		Services: []ObservedService{
			{Name: "API"},
			{Name: "Healthcheck Service"},
			{Name: "Billing"},
		},
		Calls: []ObservedCall{
			{From: "API", To: "Billing", ConnectionType: "http", RequestRate: &requests, Labels: map[string]string{"http.route": "/api/billing"}},
			{From: "API", To: "Healthcheck Service", ConnectionType: "http", RequestRate: &requests, Labels: map[string]string{"http.route": "/healthz"}},
		},
	}
	topology := Normalize(graph, integrations.FilterConfig{
		ServiceBlocklist:  []string{"healthcheck"},
		EndpointBlocklist: []string{"/health"},
		MinimumRequests:   0.5,
	})
	if len(topology.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2: %#v", len(topology.Nodes), topology.Nodes)
	}
	if len(topology.Edges) != 1 {
		t.Fatalf("edges = %d, want 1: %#v", len(topology.Edges), topology.Edges)
	}
	if topology.Edges[0].ToExternalKey != "billing" {
		t.Fatalf("remaining edge = %#v", topology.Edges[0])
	}
}

func TestNormalizeMinimumRequestThresholdDropsUnknownTraffic(t *testing.T) {
	graph := ObservedServiceGraph{
		Calls: []ObservedCall{{From: "A", To: "B", ConnectionType: "http"}},
	}
	topology := Normalize(graph, integrations.FilterConfig{MinimumRequests: 1})
	if len(topology.Edges) != 0 {
		t.Fatalf("edges = %#v, want none", topology.Edges)
	}
	if len(topology.Nodes) != 0 {
		t.Fatalf("nodes = %#v, want none because only filtered calls referenced them", topology.Nodes)
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := map[string]string{
		" Checkout Service ":       "checkout-service",
		"payments/api_gateway:v1":  "payments-api_gateway:v1",
		"@@@":                      "",
		"Inventory.Service/Worker": "inventory.service-worker",
	}
	for input, want := range tests {
		if got := NormalizeKey(input); got != want {
			t.Fatalf("NormalizeKey(%q) = %q, want %q", input, got, want)
		}
	}
}
