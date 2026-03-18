package porttest

import (
	"context"
	"errors"
	"sync"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

var ErrSourceNotFound = errors.New("source not found")

// MockAlertSource is a shared test double for port.AlertSource.
type MockAlertSource struct {
	NameVal       string
	SourceTypeVal port.SourceType
	ClosedVal     bool
	CloseErr      error
	mu            sync.RWMutex
}

func NewMockAlertSource(name string, sourceType port.SourceType) *MockAlertSource {
	return &MockAlertSource{
		NameVal:       name,
		SourceTypeVal: sourceType,
	}
}

func (m *MockAlertSource) Name() string {
	return m.NameVal
}

func (m *MockAlertSource) Type() port.SourceType {
	return m.SourceTypeVal
}

func (m *MockAlertSource) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ClosedVal = true
	return m.CloseErr
}

func (m *MockAlertSource) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ClosedVal
}

// MockWebhookSource is a shared test double for port.WebhookAlertSource.
type MockWebhookSource struct {
	*MockAlertSource

	PathVal      string
	HandleFunc   func(ctx context.Context, payload []byte) ([]*entity.Alert, error)
	HandledCalls int
	LastPayload  []byte
	mu           sync.RWMutex
}

func NewMockWebhookSource(name, path string) *MockWebhookSource {
	return &MockWebhookSource{
		MockAlertSource: NewMockAlertSource(name, port.SourceTypeWebhook),
		PathVal:         path,
	}
}

func (m *MockWebhookSource) WebhookPath() string {
	return m.PathVal
}

func (m *MockWebhookSource) HandleWebhook(ctx context.Context, payload []byte) ([]*entity.Alert, error) {
	m.mu.Lock()
	m.HandledCalls++
	m.LastPayload = payload
	m.mu.Unlock()

	if m.HandleFunc != nil {
		return m.HandleFunc(ctx, payload)
	}
	return nil, nil
}

// MockAlertSourceManager is a shared test double for port.AlertSourceManager.
type MockAlertSourceManager struct {
	SourcesVal   []port.AlertSource
	AlertHandler port.AlertHandler
	mu           sync.RWMutex
}

func (m *MockAlertSourceManager) RegisterSource(source port.AlertSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SourcesVal = append(m.SourcesVal, source)
	return nil
}

func (m *MockAlertSourceManager) UnregisterSource(_ string) error { return nil }

func (m *MockAlertSourceManager) GetSource(name string) (port.AlertSource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.SourcesVal {
		if s.Name() == name {
			return s, nil
		}
	}
	return nil, ErrSourceNotFound
}

func (m *MockAlertSourceManager) ListSources() []port.AlertSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.SourcesVal
}

func (m *MockAlertSourceManager) SetAlertHandler(handler port.AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AlertHandler = handler
}

func (m *MockAlertSourceManager) GetWebhookSourceByPath(path string) port.WebhookAlertSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.SourcesVal {
		if ws, ok := s.(port.WebhookAlertSource); ok {
			if ws.WebhookPath() == path {
				return ws
			}
		}
	}
	return nil
}
