package webhook

import (
	"bytes"
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

var errSourceNotFound = errors.New("source not found")

// mockAlertSource implements port.AlertSource for testing.
type mockAlertSource struct {
	name       string
	sourceType port.SourceType
}

func (m *mockAlertSource) Name() string          { return m.name }
func (m *mockAlertSource) Type() port.SourceType { return m.sourceType }
func (m *mockAlertSource) Close() error          { return nil }

// mockWebhookSource implements port.WebhookAlertSource for testing.
type mockWebhookSource struct {
	mockAlertSource

	webhookPath string
	handleFunc  func(ctx context.Context, payload []byte) ([]*entity.Alert, error)
}

func (m *mockWebhookSource) WebhookPath() string { return m.webhookPath }
func (m *mockWebhookSource) HandleWebhook(ctx context.Context, payload []byte) ([]*entity.Alert, error) {
	if m.handleFunc != nil {
		return m.handleFunc(ctx, payload)
	}
	return nil, nil
}

// mockSourceManager implements port.AlertSourceManager for testing.
type mockSourceManager struct {
	sources      []port.AlertSource
	alertHandler port.AlertHandler
}

func (m *mockSourceManager) RegisterSource(source port.AlertSource) error {
	m.sources = append(m.sources, source)
	return nil
}

func (m *mockSourceManager) UnregisterSource(_ string) error { return nil }
func (m *mockSourceManager) GetSource(name string) (port.AlertSource, error) {
	for _, s := range m.sources {
		if s.Name() == name {
			return s, nil
		}
	}
	return nil, errSourceNotFound
}

func (m *mockSourceManager) ListSources() []port.AlertSource {
	return m.sources
}

func (m *mockSourceManager) SetAlertHandler(handler port.AlertHandler) {
	m.alertHandler = handler
}

func TestHTTPAdapter_HealthEndpoint(t *testing.T) {
	t.Run("returns 200 OK", func(t *testing.T) {
		manager := &mockSourceManager{}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["status"] != "ok" {
			t.Errorf("expected status 'ok', got %q", resp["status"])
		}
	})
}

func TestHTTPAdapter_ReadyEndpoint(t *testing.T) {
	t.Run("returns 503 when no sources", func(t *testing.T) {
		manager := &mockSourceManager{sources: []port.AlertSource{}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("returns 200 when sources registered", func(t *testing.T) {
		manager := &mockSourceManager{
			sources: []port.AlertSource{
				&mockAlertSource{name: "test", sourceType: port.SourceTypeWebhook},
			},
		}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["sources"] != float64(1) {
			t.Errorf("expected sources=1, got %v", resp["sources"])
		}
	})
}

func TestHTTPAdapter_WebhookRouting(t *testing.T) {
	t.Run("routes to correct source by path", func(t *testing.T) {
		var receivedPayload []byte
		webhookSource := &mockWebhookSource{
			mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
			webhookPath:     "/alerts/prometheus",
			handleFunc: func(_ context.Context, payload []byte) ([]*entity.Alert, error) {
				receivedPayload = payload
				alert, _ := entity.NewAlert("test-id", "prometheus", "warning", "Test Alert")
				return []*entity.Alert{alert}, nil
			},
		}
		manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		payload := `{"alerts":[]}`
		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		if string(receivedPayload) != payload {
			t.Errorf("expected payload %q, got %q", payload, string(receivedPayload))
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["received"] != float64(1) {
			t.Errorf("expected received=1, got %v", resp["received"])
		}
	})

	t.Run("returns 404 for unknown path", func(t *testing.T) {
		manager := &mockSourceManager{sources: []port.AlertSource{}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodPost, "/alerts/unknown", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("returns 404 for nested unknown path", func(t *testing.T) {
		webhookSource := &mockWebhookSource{
			mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
			webhookPath:     "/alerts/prometheus",
		}
		manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus/extra", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("routes to nested path correctly", func(t *testing.T) {
		webhookSource := &mockWebhookSource{
			mockAlertSource: mockAlertSource{name: "prometheus-staging", sourceType: port.SourceTypeWebhook},
			webhookPath:     "/alerts/prometheus/staging",
			handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				return []*entity.Alert{}, nil
			},
		}
		manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus/staging", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestHTTPAdapter_MethodRouting(t *testing.T) {
	t.Run("GET on webhook path returns 405", func(t *testing.T) {
		webhookSource := &mockWebhookSource{
			mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
			webhookPath:     "/alerts/prometheus",
		}
		manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodGet, "/alerts/prometheus", nil)
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		// Go 1.22+ returns 405 Method Not Allowed for method mismatch
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("POST on health endpoint returns 405", func(t *testing.T) {
		manager := &mockSourceManager{}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodPost, "/health", nil)
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})
}

func TestHTTPAdapter_AlertHandlerIntegration(t *testing.T) {
	t.Run("dispatches alerts to handler", func(t *testing.T) {
		var handledAlerts []*entity.Alert
		webhookSource := &mockWebhookSource{
			mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
			webhookPath:     "/alerts/prometheus",
			handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				alert1, _ := entity.NewAlert("alert-1", "prometheus", "critical", "High CPU")
				alert2, _ := entity.NewAlert("alert-2", "prometheus", "warning", "High Memory")
				return []*entity.Alert{alert1, alert2}, nil
			},
		}
		manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())
		adapter.SetAlertHandler(func(_ context.Context, alert *entity.Alert) error {
			handledAlerts = append(handledAlerts, alert)
			return nil
		})

		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}

		if len(handledAlerts) != 2 {
			t.Errorf("expected 2 alerts handled, got %d", len(handledAlerts))
		}
	})

	t.Run("counts handler errors", func(t *testing.T) {
		webhookSource := &mockWebhookSource{
			mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
			webhookPath:     "/alerts/prometheus",
			handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "Test")
				return []*entity.Alert{alert}, nil
			},
		}
		manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())
		adapter.SetAlertHandler(func(_ context.Context, _ *entity.Alert) error {
			return context.DeadlineExceeded // Simulate error
		})

		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp["errors"] != float64(1) {
			t.Errorf("expected errors=1, got %v", resp["errors"])
		}
	})
}

func TestHTTPAdapter_ErrorHandling(t *testing.T) {
	t.Run("returns 400 for invalid payload", func(t *testing.T) {
		webhookSource := &mockWebhookSource{
			mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
			webhookPath:     "/alerts/prometheus",
			handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				return nil, context.DeadlineExceeded
			},
		}
		manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("invalid"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})
}

func TestHTTPAdapter_Config(t *testing.T) {
	t.Run("Addr returns configured address", func(t *testing.T) {
		config := HTTPAdapterConfig{Addr: ":9090"}
		manager := &mockSourceManager{}
		adapter := NewHTTPAdapter(manager, config)

		if adapter.Addr() != ":9090" {
			t.Errorf("expected :9090, got %s", adapter.Addr())
		}
	})

	t.Run("DefaultConfig has sensible defaults", func(t *testing.T) {
		config := DefaultConfig()

		if config.Addr != ":8080" {
			t.Errorf("expected :8080, got %s", config.Addr)
		}
		if config.ReadTimeout != 30*1e9 {
			t.Errorf("expected 30s, got %v", config.ReadTimeout)
		}
		if config.WriteTimeout != 30*1e9 {
			t.Errorf("expected 30s, got %v", config.WriteTimeout)
		}
		if config.ShutdownTimeout != 10*1e9 {
			t.Errorf("expected 10s, got %v", config.ShutdownTimeout)
		}
	})
}

func TestHTTPAdapter_AsyncHandler_Returns202(t *testing.T) {
	webhookSource := &mockWebhookSource{
		mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
		webhookPath:     "/alerts/prometheus",
		handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "High CPU")
			return []*entity.Alert{alert}, nil
		},
	}
	manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
	adapter := NewHTTPAdapter(manager, DefaultConfig())

	var runnerCalled bool
	var runnerWg sync.WaitGroup
	runnerWg.Add(1)

	adapter.SetAsyncAlertHandler(
		func(_ context.Context, _ *entity.Alert) (string, error) {
			return "inv-12345", nil
		},
		func(_ context.Context, _ *entity.Alert, _ string) error {
			runnerCalled = true
			runnerWg.Done()
			return nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
	rec := httptest.NewRecorder()

	adapter.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "accepted" {
		t.Errorf("expected status 'accepted', got %q", resp["status"])
	}
	if resp["investigation_id"] != "inv-12345" {
		t.Errorf("expected investigation_id 'inv-12345', got %q", resp["investigation_id"])
	}

	runnerWg.Wait()
	if !runnerCalled {
		t.Error("expected runner to be called")
	}
}

func TestHTTPAdapter_AsyncHandler_FilteredAlerts(t *testing.T) {
	webhookSource := &mockWebhookSource{
		mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
		webhookPath:     "/alerts/prometheus",
		handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("alert-1", "prometheus", "info", "Info Alert")
			return []*entity.Alert{alert}, nil
		},
	}
	manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
	adapter := NewHTTPAdapter(manager, DefaultConfig())

	adapter.SetAsyncAlertHandler(
		func(_ context.Context, _ *entity.Alert) (string, error) {
			return "", nil // Empty ID means filtered out
		},
		func(_ context.Context, _ *entity.Alert, _ string) error {
			t.Error("runner should not be called for filtered alerts")
			return nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
	rec := httptest.NewRecorder()

	adapter.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["message"] != "no investigations started (alerts filtered)" {
		t.Errorf("expected filtered message, got %q", resp["message"])
	}
}

func TestHTTPAdapter_AsyncHandler_StartError(t *testing.T) {
	webhookSource := &mockWebhookSource{
		mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
		webhookPath:     "/alerts/prometheus",
		handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "Critical Alert")
			return []*entity.Alert{alert}, nil
		},
	}
	manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
	adapter := NewHTTPAdapter(manager, DefaultConfig())

	adapter.SetAsyncAlertHandler(
		func(_ context.Context, _ *entity.Alert) (string, error) {
			return "", errors.New("start failed")
		},
		func(_ context.Context, _ *entity.Alert, _ string) error {
			t.Error("runner should not be called when start fails")
			return nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
	rec := httptest.NewRecorder()

	adapter.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPAdapter_AsyncHandler_ShutdownWaits(t *testing.T) {
	webhookSource := &mockWebhookSource{
		mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
		webhookPath:     "/alerts/prometheus",
		handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "Critical Alert")
			return []*entity.Alert{alert}, nil
		},
	}
	manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
	adapter := NewHTTPAdapter(manager, DefaultConfig())

	runnerStarted := make(chan struct{})
	runnerComplete := make(chan struct{})

	adapter.SetAsyncAlertHandler(
		func(_ context.Context, _ *entity.Alert) (string, error) {
			return "inv-shutdown-test", nil
		},
		func(_ context.Context, _ *entity.Alert, _ string) error {
			close(runnerStarted)
			<-runnerComplete
			return nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
	rec := httptest.NewRecorder()
	adapter.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}

	<-runnerStarted

	shutdownComplete := make(chan struct{})
	go func() {
		_ = adapter.Shutdown()
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		t.Error("shutdown completed before investigation finished")
	case <-time.After(50 * time.Millisecond):
		// Expected - shutdown should be blocked
	}

	close(runnerComplete)

	select {
	case <-shutdownComplete:
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("shutdown did not complete after investigation finished")
	}
}

func TestHTTPAdapter_AsyncAndSyncHandlerPrecedence(t *testing.T) {
	t.Run("async handler takes precedence over sync handler", func(t *testing.T) {
		webhookSource := &mockWebhookSource{
			mockAlertSource: mockAlertSource{name: "prometheus", sourceType: port.SourceTypeWebhook},
			webhookPath:     "/alerts/prometheus",
			handleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "High CPU")
				return []*entity.Alert{alert}, nil
			},
		}
		manager := &mockSourceManager{sources: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig())

		// Set both sync and async handlers
		syncCalled := false
		adapter.SetAlertHandler(func(_ context.Context, _ *entity.Alert) error {
			syncCalled = true
			return nil
		})

		asyncCalled := false
		var runnerWg sync.WaitGroup
		runnerWg.Add(1)
		adapter.SetAsyncAlertHandler(
			func(_ context.Context, _ *entity.Alert) (string, error) {
				asyncCalled = true
				return "inv-async", nil
			},
			func(_ context.Context, _ *entity.Alert, _ string) error {
				runnerWg.Done()
				return nil
			},
		)

		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		// Should return 202 (async) not 200 (sync)
		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}

		runnerWg.Wait()

		if !asyncCalled {
			t.Error("expected async handler to be called")
		}
		if syncCalled {
			t.Error("sync handler should not be called when async is set")
		}
	})
}
