// Package alert provides adapters for various alert sources.
// This file implements a factory registry pattern for extensible source creation.
package alert

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// Configuration errors for alert sources.
var (
	errSourceNameRequired   = errors.New("source name is required")
	errWebhookPathRequired  = errors.New("webhook path is required")
	errWebhookPathNoSlash   = errors.New("webhook path must start with a leading slash")
	errWebhookPathTraversal = errors.New("webhook path contains path traversal")
	errEmptyPayload         = errors.New("empty payload")
)

// AlertSourceFactory creates a source from configuration.
// Implementations should validate config and return an error if invalid.
type AlertSourceFactory func(cfg SourceConfig) (port.AlertSource, error)

// SourceConfig contains configuration for creating an alert source.
// It provides a unified configuration structure for all alert source types.
type SourceConfig struct {
	// Type specifies the alert source type (e.g., "prometheus", "grafana").
	Type string
	// Name is the unique identifier for this source instance.
	Name string
	// WebhookPath is the HTTP path for receiving webhooks (required for webhook sources).
	WebhookPath string
	// Extra contains additional source-specific configuration options.
	Extra map[string]string
}

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

// validateSourceConfig performs common validation for alert source configurations.
// Returns an error if the name or webhook path is invalid.
func validateSourceConfig(config SourceConfig) error {
	if strings.TrimSpace(config.Name) == "" {
		return errSourceNameRequired
	}
	if strings.TrimSpace(config.WebhookPath) == "" {
		return errWebhookPathRequired
	}
	if !strings.HasPrefix(config.WebhookPath, "/") {
		return errWebhookPathNoSlash
	}
	if strings.Contains(config.WebhookPath, "..") {
		return errWebhookPathTraversal
	}
	return nil
}
