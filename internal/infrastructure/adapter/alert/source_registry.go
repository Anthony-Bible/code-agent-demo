// Package alert provides adapters for various alert sources.
// This file implements a factory registry pattern for extensible source creation.
package alert

import (
	"code-editing-agent/internal/domain/port"
	"fmt"
	"sort"
	"sync"
)

// AlertSourceFactory creates a source from configuration.
// Implementations should validate config and return an error if invalid.
type AlertSourceFactory func(cfg SourceConfig) (port.AlertSource, error)

// SourceRegistry manages alert source factories using a thread-safe registry.
// It provides a central place to register and instantiate alert sources.
type SourceRegistry struct {
	mu        sync.RWMutex
	factories map[string]AlertSourceFactory
}

// NewSourceRegistry creates an empty source registry.
func NewSourceRegistry() *SourceRegistry {
	return &SourceRegistry{
		factories: make(map[string]AlertSourceFactory),
	}
}

// RegisterFactory adds a factory for the specified source type.
// If a factory for the type already exists, it will be replaced.
func (r *SourceRegistry) RegisterFactory(sourceType string, factory AlertSourceFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[sourceType] = factory
}

// CreateSource instantiates an alert source from configuration.
// Returns an error if the source type is unknown or if the factory fails.
func (r *SourceRegistry) CreateSource(cfg SourceConfig) (port.AlertSource, error) {
	r.mu.RLock()
	factory, ok := r.factories[cfg.Type]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown source type: %s (supported types: %v)", cfg.Type, r.SupportedTypes())
	}

	return factory(cfg)
}

// SupportedTypes returns a sorted list of registered source types.
// Useful for error messages and documentation.
func (r *SourceRegistry) SupportedTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.factories))
	for sourceType := range r.factories {
		types = append(types, sourceType)
	}
	sort.Strings(types)
	return types
}

// RegisterBuiltinFactories registers all built-in alert source factories.
// This includes prometheus and gcp_monitoring sources.
func (r *SourceRegistry) RegisterBuiltinFactories() {
	r.RegisterFactory("prometheus", NewPrometheusSource)
	r.RegisterFactory("gcp_monitoring", NewGCPMonitoringSource)
}
