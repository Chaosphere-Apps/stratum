package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/system-design-evaluator/backend/internal/integrations"
	"github.com/system-design-evaluator/backend/internal/integrations/otelgraph"
)

type Provider struct {
	client *Client
}

func NewProvider(httpClient *http.Client) *Provider {
	return &Provider{client: NewClient(httpClient)}
}

func (provider *Provider) Kind() string {
	return Kind
}

func (provider *Provider) TestConnection(ctx context.Context, config integrations.IntegrationConfig) error {
	_, err := provider.client.Query(ctx, config, "up")
	return err
}

func (provider *Provider) Fetch(ctx context.Context, request integrations.FetchTopologyRequest) (integrations.RawTopologyResult, error) {
	if !request.Integration.Enabled {
		return integrations.RawTopologyResult{}, fmt.Errorf("integration is disabled")
	}
	metrics := MetricsFromIntegration(request.Integration)
	queries := BuildServiceGraphQueries(metrics)
	bundle := QueryBundle{}
	var warnings []string
	var err error
	if bundle.RequestTotal, err = provider.client.Query(ctx, request.Integration, queries.RequestTotal); err != nil {
		return integrations.RawTopologyResult{}, err
	}
	if bundle.RequestFailed, err = provider.client.Query(ctx, request.Integration, queries.RequestFailed); err != nil {
		warnings = append(warnings, "request failure metric query failed")
	}
	if bundle.ServerP95, err = provider.client.Query(ctx, request.Integration, queries.ServerP95); err != nil {
		warnings = append(warnings, "server latency metric query failed")
	}
	if bundle.ClientP95, err = provider.client.Query(ctx, request.Integration, queries.ClientP95); err != nil {
		warnings = append(warnings, "client latency metric query failed")
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		return integrations.RawTopologyResult{}, err
	}
	return integrations.RawTopologyResult{
		Source:      Kind,
		FetchedAt:   nowUTC(),
		ContentType: "application/json",
		Payload:     payload,
		Warnings:    warnings,
	}, nil
}

func (provider *Provider) FetchObservedGraph(ctx context.Context, request integrations.FetchTopologyRequest) (otelgraph.ObservedServiceGraph, error) {
	raw, err := provider.Fetch(ctx, request)
	if err != nil {
		return otelgraph.ObservedServiceGraph{}, err
	}
	graph, err := ParseRawServiceGraph(raw, request)
	if err != nil {
		return otelgraph.ObservedServiceGraph{}, err
	}
	graph.Warnings = append(raw.Warnings, graph.Warnings...)
	return graph, nil
}

func ParseRawServiceGraph(raw integrations.RawTopologyResult, request integrations.FetchTopologyRequest) (otelgraph.ObservedServiceGraph, error) {
	var bundle QueryBundle
	if err := json.Unmarshal(raw.Payload, &bundle); err != nil {
		return otelgraph.ObservedServiceGraph{}, err
	}
	return ParseServiceGraph(
		bundle,
		raw.Source,
		request.WindowStart,
		request.WindowEnd,
		request.Integration.Filters,
	)
}
