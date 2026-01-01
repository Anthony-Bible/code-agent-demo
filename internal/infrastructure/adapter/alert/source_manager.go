package alert

import (
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"sync"
)

// ErrorCallback is called when a source reports an error.
type ErrorCallback func(source string, err error)

// LocalAlertSourceManager manages alert sources locally.
type LocalAlertSourceManager struct {
	mu            sync.RWMutex
	sources       map[string]port.AlertSource
	alertHandler  port.AlertHandler
	errorCallback ErrorCallback
	started       bool
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewLocalAlertSourceManager creates a new local alert source manager.
func NewLocalAlertSourceManager() *LocalAlertSourceManager {
	return &LocalAlertSourceManager{
		sources: make(map[string]port.AlertSource),
	}
}

// RegisterSource registers an alert source.
func (m *LocalAlertSourceManager) RegisterSource(source port.AlertSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := source.Name()
	if _, exists := m.sources[name]; exists {
		return errors.New("source already registered: " + name)
	}

	m.sources[name] = source
	return nil
}

// UnregisterSource unregisters and closes an alert source.
func (m *LocalAlertSourceManager) UnregisterSource(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	source, exists := m.sources[name]
	if !exists {
		return errors.New("source not found: " + name)
	}

	delete(m.sources, name)

	if err := source.Close(); err != nil {
		return err
	}

	return nil
}

// GetSource returns a registered source by name.
func (m *LocalAlertSourceManager) GetSource(name string) (port.AlertSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	source, exists := m.sources[name]
	if !exists {
		return nil, errors.New("source not found: " + name)
	}

	return source, nil
}

// ListSources returns all registered sources.
func (m *LocalAlertSourceManager) ListSources() []port.AlertSource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sources := make([]port.AlertSource, 0, len(m.sources))
	for _, source := range m.sources {
		sources = append(sources, source)
	}
	return sources
}

// SetAlertHandler sets the handler for incoming alerts.
func (m *LocalAlertSourceManager) SetAlertHandler(handler port.AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandler = handler
}

// SetErrorCallback sets the callback for source errors.
func (m *LocalAlertSourceManager) SetErrorCallback(callback ErrorCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCallback = callback
}

// Start starts the manager.
func (m *LocalAlertSourceManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.started = true

	return nil
}

// Shutdown stops the manager and closes all sources.
func (m *LocalAlertSourceManager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	var errs []error
	for name, source := range m.sources {
		if err := source.Close(); err != nil {
			errs = append(errs, errors.New(name+": "+err.Error()))
		}
	}

	m.started = false

	if len(errs) > 0 {
		return errors.New("failed to close sources")
	}

	return nil
}
