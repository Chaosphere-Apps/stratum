package topology

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/system-design-evaluator/backend/internal/integrations"
	"github.com/system-design-evaluator/backend/internal/integrations/otelgraph"
)

type fakeObservedProvider struct {
	kind  string
	graph otelgraph.ObservedServiceGraph
	err   error
}

func (provider fakeObservedProvider) Kind() string {
	return provider.kind
}

func (provider fakeObservedProvider) FetchObservedGraph(context.Context, integrations.FetchTopologyRequest) (otelgraph.ObservedServiceGraph, error) {
	return provider.graph, provider.err
}

func TestServiceFetchProjection(t *testing.T) {
	requests := 300.0
	service, err := NewService(fakeObservedProvider{
		kind: "test",
		graph: otelgraph.ObservedServiceGraph{
			Source:      "test",
			WindowStart: time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
			WindowEnd:   time.Date(2026, 6, 18, 12, 5, 0, 0, time.UTC),
			Calls: []otelgraph.ObservedCall{
				{From: "API", To: "Postgres", ConnectionType: "database", RequestRate: &requests, Labels: map[string]string{"db.system": "postgresql"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	result, err := service.FetchProjection(context.Background(), integrations.FetchTopologyRequest{
		Integration: integrations.IntegrationConfig{ID: "integration_1", Kind: "test", Filters: integrations.FilterConfig{}},
	}, otelgraph.ProjectionPolicy{})
	if err != nil {
		t.Fatalf("FetchProjection returned error: %v", err)
	}
	if len(result.Normalized.Edges) != 1 {
		t.Fatalf("normalized edges = %#v, want one", result.Normalized.Edges)
	}
	if len(result.Projection.Components) != 2 {
		t.Fatalf("projected components = %#v, want two", result.Projection.Components)
	}
	if result.Projection.Components[0].Metadata["observed"] == nil {
		t.Fatalf("missing observed provenance metadata: %#v", result.Projection.Components[0].Metadata)
	}
}

func TestServiceFetchProjectionRequiresProvider(t *testing.T) {
	service, err := NewService()
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	_, err = service.FetchProjection(context.Background(), integrations.FetchTopologyRequest{
		Integration: integrations.IntegrationConfig{Kind: "missing"},
	}, otelgraph.ProjectionPolicy{})
	if err == nil {
		t.Fatal("expected missing provider error")
	}
}

func TestServicePropagatesProviderError(t *testing.T) {
	providerErr := errors.New("provider failed")
	service, err := NewService(fakeObservedProvider{kind: "test", err: providerErr})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	_, err = service.FetchProjection(context.Background(), integrations.FetchTopologyRequest{
		Integration: integrations.IntegrationConfig{Kind: "test"},
	}, otelgraph.ProjectionPolicy{})
	if !errors.Is(err, providerErr) {
		t.Fatalf("error = %v, want provider error", err)
	}
}
