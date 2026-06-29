package topology

import (
	"context"
	"fmt"
	"sync"

	"github.com/system-design-evaluator/backend/internal/integrations"
	"github.com/system-design-evaluator/backend/internal/integrations/otelgraph"
)

type ObservedGraphProvider interface {
	Kind() string
	FetchObservedGraph(ctx context.Context, request integrations.FetchTopologyRequest) (otelgraph.ObservedServiceGraph, error)
}

type Service struct {
	mu        sync.RWMutex
	providers map[string]ObservedGraphProvider
}

type Result struct {
	Graph      otelgraph.ObservedServiceGraph
	Normalized otelgraph.NormalizedTopology
	Projection otelgraph.StratumTopologyProjection
}

func NewService(providers ...ObservedGraphProvider) (*Service, error) {
	service := &Service{providers: make(map[string]ObservedGraphProvider)}
	for _, provider := range providers {
		if err := service.Register(provider); err != nil {
			return nil, err
		}
	}
	return service, nil
}

func (service *Service) Register(provider ObservedGraphProvider) error {
	if provider == nil {
		return integrations.ErrNilProvider
	}
	kind := provider.Kind()
	if kind == "" {
		return integrations.ErrEmptyProviderKind
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, exists := service.providers[kind]; exists {
		return fmt.Errorf("%w: %s", integrations.ErrDuplicateProvider, kind)
	}
	service.providers[kind] = provider
	return nil
}

func (service *Service) FetchProjection(ctx context.Context, request integrations.FetchTopologyRequest, policy otelgraph.ProjectionPolicy) (Result, error) {
	provider, ok := service.provider(request.Integration.Kind)
	if !ok {
		return Result{}, fmt.Errorf("observed topology provider %q is not registered", request.Integration.Kind)
	}
	graph, err := provider.FetchObservedGraph(ctx, request)
	if err != nil {
		return Result{}, err
	}
	normalized := otelgraph.Normalize(graph, request.Integration.Filters)
	if policy.Source == "" {
		policy.Source = graph.Source
	}
	if policy.IntegrationID == "" {
		policy.IntegrationID = request.Integration.ID
	}
	projection := otelgraph.Project(normalized, policy)
	return Result{
		Graph:      graph,
		Normalized: normalized,
		Projection: projection,
	}, nil
}

func (service *Service) provider(kind string) (ObservedGraphProvider, bool) {
	service.mu.RLock()
	defer service.mu.RUnlock()
	provider, ok := service.providers[kind]
	return provider, ok
}
