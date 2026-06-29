package otelgraph

import (
	"testing"
	"time"
)

func TestProjectMapsObservedTopologyToStratumComponents(t *testing.T) {
	requestRate := 90.0
	errorRate := 0.02
	latency := 44.0
	topology := NormalizedTopology{
		Source:      "prometheus",
		WindowStart: time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 6, 12, 10, 5, 0, 0, time.UTC),
		Nodes: []NormalizedNode{
			{ExternalKey: "checkout-service", Name: "Checkout Service"},
			{ExternalKey: "postgres-primary", Name: "Postgres Primary", Labels: map[string]string{"db.system": "postgresql"}},
			{ExternalKey: "redis-cache", Name: "Redis Cache", Labels: map[string]string{"db.system": "redis"}},
			{ExternalKey: "events-topic", Name: "Events Topic", Labels: map[string]string{"messaging.system": "kafka"}},
		},
		Edges: []NormalizedEdge{
			{
				ExternalKey:     "checkout-service->postgres-primary:database",
				FromExternalKey: "checkout-service",
				ToExternalKey:   "postgres-primary",
				ConnectionType:  "database",
				RequestRate:     &requestRate,
				ErrorRate:       &errorRate,
				P95LatencyMs:    &latency,
				Labels:          map[string]string{"rpc.system": "grpc"},
			},
			{
				ExternalKey:     "checkout-service->events-topic:messaging_system",
				FromExternalKey: "checkout-service",
				ToExternalKey:   "events-topic",
				ConnectionType:  "messaging_system",
				Labels:          map[string]string{"messaging.system": "kafka"},
			},
		},
	}
	projection := Project(topology, ProjectionPolicy{IntegrationID: "integration_1"})
	if len(projection.Components) != 4 {
		t.Fatalf("components = %d, want 4", len(projection.Components))
	}
	assertComponentType(t, projection.Components, "checkout-service", ComponentService)
	assertComponentType(t, projection.Components, "postgres-primary", ComponentSQLDatabase)
	assertComponentType(t, projection.Components, "redis-cache", ComponentRedis)
	assertComponentType(t, projection.Components, "events-topic", ComponentQueue)
	if len(projection.Connectors) != 2 {
		t.Fatalf("connectors = %d, want 2", len(projection.Connectors))
	}
	if projection.Connectors[0].Metrics["requestRate"] != requestRate {
		t.Fatalf("request rate metric = %#v", projection.Connectors[0].Metrics)
	}
	if projection.Connectors[1].Type != ConnectorAsyncEvent {
		t.Fatalf("async connector type = %q", projection.Connectors[1].Type)
	}
}

func TestProjectMatchesEnterpriseCatalogCandidates(t *testing.T) {
	topology := NormalizedTopology{
		Nodes: []NormalizedNode{{ExternalKey: "purchase-service", Name: "Purchase Service"}},
	}
	projection := Project(topology, ProjectionPolicy{
		CatalogCandidates: []CatalogCandidate{
			{
				ID:             "asset_1",
				Name:           "Purchase Service",
				NormalizedName: "purchase-service",
				Type:           ComponentService,
			},
		},
	})
	if len(projection.Matches) != 1 {
		t.Fatalf("matches = %#v, want one", projection.Matches)
	}
	if projection.Components[0].CatalogAssetID == nil || *projection.Components[0].CatalogAssetID != "asset_1" {
		t.Fatalf("catalog asset id = %#v", projection.Components[0].CatalogAssetID)
	}
}

func assertComponentType(t *testing.T, components []ProjectedComponent, externalKey string, want string) {
	t.Helper()
	for _, component := range components {
		if component.ExternalKey == externalKey {
			if component.Type != want {
				t.Fatalf("component %s type = %q, want %q", externalKey, component.Type, want)
			}
			return
		}
	}
	t.Fatalf("component %s not found in %#v", externalKey, components)
}
