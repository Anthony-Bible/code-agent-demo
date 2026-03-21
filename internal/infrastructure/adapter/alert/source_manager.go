package alert

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// Manager errors for source registration and lookup.
var (
	errSourceAlreadyRegistered = errors.New("source already registered")
	errSourceNotFound          = errors.New("source not found")
)

// LocalAlertSourceManager implements port.AlertSourceManager for local/in-process alert handling.
// It maintains a thread-safe registry of alert sources and manages their lifecycle.
type LocalAlertSourceManager struct {
	mu           sync.RWMutex
	sources      map[string]port.AlertSource
	paths        map[string]port.WebhookAlertSource
	alertHandler port.AlertHandler
	started      bool
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewLocalAlertSourceManager creates a new local alert source manager.
func NewLocalAlertSourceManager() *LocalAlertSourceManager {
	return &LocalAlertSourceManager{
		sources: make(map[string]port.AlertSource),
		paths:   make(map[string]port.WebhookAlertSource),
	}
}

// RegisterSource adds an alert source to the manager's registry.
// Returns an error if a source with the same name is already registered.
func (m *LocalAlertSourceManager) RegisterSource(source port.AlertSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := source.Name()
	if _, exists := m.sources[name]; exists {
		return fmt.Errorf("%w: %s", errSourceAlreadyRegistered, name)
	}

	m.sources[name] = source

	// If it's a webhook source, register its path
	if webhookSrc, ok := source.(port.WebhookAlertSource); ok {
		m.paths[webhookSrc.WebhookPath()] = webhookSrc
	}

	return nil
}

// UnregisterSource removes an alert source from the registry and closes it.
// Returns an error if the source is not found or if closing fails.
func (m *LocalAlertSourceManager) UnregisterSource(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	source, exists := m.sources[name]
	if !exists {
		return fmt.Errorf("%w: %s", errSourceNotFound, name)
	}

	delete(m.sources, name)

	// Remove from paths map if it was a webhook source
	if webhookSrc, ok := source.(port.WebhookAlertSource); ok {
		delete(m.paths, webhookSrc.WebhookPath())
	}

	if err := source.Close(); err != nil {
		return err
	}

	return nil
}

// GetSource retrieves a registered source by name.
// Returns an error if the source is not found.
func (m *LocalAlertSourceManager) GetSource(name string) (port.AlertSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	source, exists := m.sources[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", errSourceNotFound, name)
	}

	return source, nil
}

// ListSources returns a slice of all registered alert sources.
// The returned slice is a copy; modifications do not affect the registry.
func (m *LocalAlertSourceManager) ListSources() []port.AlertSource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sources := make([]port.AlertSource, 0, len(m.sources))
	for _, source := range m.sources {
		sources = append(sources, source)
	}
	return sources
}

// GetWebhookSourceByPath retrieves a registered webhook source by its HTTP path.
// Returns nil if no matching webhook source is found.
func (m *LocalAlertSourceManager) GetWebhookSourceByPath(path string) port.WebhookAlertSource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.paths[path]
}

// SetAlertHandler sets the callback function for processing incoming alerts.
// The handler is called for each alert received from any registered source.
func (m *LocalAlertSourceManager) SetAlertHandler(handler port.AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandler = handler
}

// Start initializes the manager with the given context.
// The context is used to control the lifecycle of background operations.
func (m *LocalAlertSourceManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.started = true

	return nil
}

// Shutdown stops the manager and closes all registered sources.
// Returns an error if any source fails to close.
func (m *LocalAlertSourceManager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	var closeErrors []error
	for name, source := range m.sources {
		if err := source.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("%s: %w", name, err))
		}
	}

	m.started = false

	if len(closeErrors) > 0 {
		return errors.Join(closeErrors...)
	}

	return nil
}
