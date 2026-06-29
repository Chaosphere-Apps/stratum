package integrations

import (
	"fmt"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	providers map[string]TopologyProvider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]TopologyProvider)}
}

func (registry *Registry) Register(provider TopologyProvider) error {
	if provider == nil {
		return ErrNilProvider
	}
	kind := provider.Kind()
	if kind == "" {
		return ErrEmptyProviderKind
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.providers[kind]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateProvider, kind)
	}
	registry.providers[kind] = provider
	return nil
}

func (registry *Registry) Get(kind string) (TopologyProvider, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	provider, ok := registry.providers[kind]
	return provider, ok
}

func (registry *Registry) Kinds() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	kinds := make([]string, 0, len(registry.providers))
	for kind := range registry.providers {
		kinds = append(kinds, kind)
	}
	return kinds
}
