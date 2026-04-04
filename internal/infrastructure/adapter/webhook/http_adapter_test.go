package webhook

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port/porttest"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/logger"
)

func TestHTTPAdapter_HealthEndpoint(t *testing.T) {
	t.Run("returns 200 OK", func(t *testing.T) {
		manager := &porttest.MockAlertSourceManager{}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("returns 200 when sources registered", func(t *testing.T) {
		manager := &porttest.MockAlertSourceManager{
			SourcesVal: []port.AlertSource{
				&porttest.MockAlertSource{NameVal: "test", SourceTypeVal: port.SourceTypeWebhook},
			},
		}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
		webhookSource := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/prometheus",
			HandleFunc: func(_ context.Context, payload []byte) ([]*entity.Alert, error) {
				receivedPayload = payload
				alert, _ := entity.NewAlert("test-id", "prometheus", "warning", "Test Alert")
				return []*entity.Alert{alert}, nil
			},
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

		req := httptest.NewRequest(http.MethodPost, "/alerts/unknown", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("returns 404 for nested unknown path", func(t *testing.T) {
		webhookSource := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/prometheus",
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus/extra", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("routes to nested path correctly", func(t *testing.T) {
		webhookSource := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus-staging", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/prometheus/staging",
			HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				return []*entity.Alert{}, nil
			},
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
		webhookSource := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/prometheus",
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

		req := httptest.NewRequest(http.MethodGet, "/alerts/prometheus", nil)
		rec := httptest.NewRecorder()

		adapter.Mux().ServeHTTP(rec, req)

		// Go 1.22+ returns 405 Method Not Allowed for method mismatch
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("POST on health endpoint returns 405", func(t *testing.T) {
		manager := &porttest.MockAlertSourceManager{}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
		webhookSource := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/prometheus",
			HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				alert1, _ := entity.NewAlert("alert-1", "prometheus", "critical", "High CPU")
				alert2, _ := entity.NewAlert("alert-2", "prometheus", "warning", "High Memory")
				return []*entity.Alert{alert1, alert2}, nil
			},
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())
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
		webhookSource := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/prometheus",
			HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "Test")
				return []*entity.Alert{alert}, nil
			},
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())
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
		webhookSource := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/prometheus",
			HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				return nil, context.DeadlineExceeded
			},
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
		manager := &porttest.MockAlertSourceManager{}
		adapter := NewHTTPAdapter(manager, config, logger.NewNop())

		if adapter.Addr() != ":9090" {
			t.Errorf("expected :9090, got %s", adapter.Addr())
		}
	})

	t.Run("DefaultConfig has sensible defaults", func(t *testing.T) {
		config := DefaultConfig()

		if config.Addr != ":8080" {
			t.Errorf("expected :8080, got %s", config.Addr)
		}
		if config.ReadTimeout != 30*time.Second {
			t.Errorf("expected 30s, got %v", config.ReadTimeout)
		}
		if config.WriteTimeout != 30*time.Second {
			t.Errorf("expected 30s, got %v", config.WriteTimeout)
		}
		if config.ShutdownTimeout != 10*time.Second {
			t.Errorf("expected 10s, got %v", config.ShutdownTimeout)
		}
	})
}

func TestHTTPAdapter_AsyncHandler_Returns202(t *testing.T) {
	webhookSource := &porttest.MockWebhookSource{
		MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
		PathVal:         "/alerts/prometheus",
		HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "High CPU")
			return []*entity.Alert{alert}, nil
		},
	}
	manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
	adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
	webhookSource := &porttest.MockWebhookSource{
		MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
		PathVal:         "/alerts/prometheus",
		HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("alert-1", "prometheus", "info", "Info Alert")
			return []*entity.Alert{alert}, nil
		},
	}
	manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
	adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
	webhookSource := &porttest.MockWebhookSource{
		MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
		PathVal:         "/alerts/prometheus",
		HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "Critical Alert")
			return []*entity.Alert{alert}, nil
		},
	}
	manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
	adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

	adapter.SetAsyncAlertHandler(
		func(_ context.Context, _ *entity.Alert) (string, error) {
			return "", context.DeadlineExceeded
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
	webhookSource := &porttest.MockWebhookSource{
		MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
		PathVal:         "/alerts/prometheus",
		HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "Critical Alert")
			return []*entity.Alert{alert}, nil
		},
	}
	manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
	adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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

// basicAuthHeader returns the Authorization header value for the given credentials.
func basicAuthHeader(username, password string) string {
	creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + creds
}

func TestHTTPAdapter_BasicAuth(t *testing.T) {
	makeSource := func(path string) *porttest.MockWebhookSource {
		return &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         path,
			HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				return []*entity.Alert{}, nil
			},
		}
	}

	tests := []struct {
		name       string
		authUser   string
		authPass   string
		reqUser    string
		reqPass    string
		wantStatus int
	}{
		{
			name:       "allows request when no auth configured",
			wantStatus: http.StatusOK,
		},
		{
			name:       "rejects request with no credentials when auth is required",
			authUser:   "user",
			authPass:   "pass",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "rejects request with wrong password",
			authUser:   "user",
			authPass:   "correct-pass",
			reqUser:    "user",
			reqPass:    "wrong-pass",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "rejects request with wrong username",
			authUser:   "correct-user",
			authPass:   "pass",
			reqUser:    "wrong-user",
			reqPass:    "pass",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "allows request with correct credentials",
			authUser:   "myuser",
			authPass:   "mypass",
			reqUser:    "myuser",
			reqPass:    "mypass",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := makeSource("/alerts/prometheus")
			manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{source}}
			adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

			if tt.authUser != "" {
				adapter.SetBasicAuth("/alerts/prometheus", tt.authUser, tt.authPass)
			}

			req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
			if tt.reqUser != "" {
				req.Header.Set("Authorization", basicAuthHeader(tt.reqUser, tt.reqPass))
			}
			rec := httptest.NewRecorder()
			adapter.Mux().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
			if tt.wantStatus == http.StatusUnauthorized && tt.reqUser == "" {
				if rec.Header().Get("WWW-Authenticate") == "" {
					t.Error("expected WWW-Authenticate header to be set")
				}
			}
		})
	}

	t.Run("auth is scoped per path: authenticated path does not affect unauthenticated path", func(t *testing.T) {
		sourceA := makeSource("/alerts/prometheus")
		sourceB := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "gcp", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/gcp",
			HandleFunc:      func(_ context.Context, _ []byte) ([]*entity.Alert, error) { return nil, nil },
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{sourceA, sourceB}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())
		adapter.SetBasicAuth("/alerts/prometheus", "user", "pass")

		// /alerts/gcp has no auth configured — should be accessible without credentials
		req := httptest.NewRequest(http.MethodPost, "/alerts/gcp", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()
		adapter.Mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for unauthenticated path, got %d", rec.Code)
		}
	})

	t.Run("removes credentials when called with empty strings", func(t *testing.T) {
		source := makeSource("/alerts/prometheus")
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{source}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())
		adapter.SetBasicAuth("/alerts/prometheus", "user", "pass")

		// Confirm auth is enforced
		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()
		adapter.Mux().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 before removal, got %d", rec.Code)
		}

		// Remove credentials
		adapter.SetBasicAuth("/alerts/prometheus", "", "")

		// Confirm auth is no longer enforced
		req2 := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
		rec2 := httptest.NewRecorder()
		adapter.Mux().ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Errorf("expected 200 after credential removal, got %d", rec2.Code)
		}
	})

	t.Run("response body does not contain credential information on auth failure", func(t *testing.T) {
		source := makeSource("/alerts/prometheus")
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{source}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())
		adapter.SetBasicAuth("/alerts/prometheus", "secretuser", "secretpass")

		req := httptest.NewRequest(http.MethodPost, "/alerts/prometheus", bytes.NewBufferString("{}"))
		req.Header.Set("Authorization", basicAuthHeader("secretuser", "wrongpass"))
		rec := httptest.NewRecorder()
		adapter.Mux().ServeHTTP(rec, req)

		body := rec.Body.String()
		if strings.Contains(body, "secretuser") || strings.Contains(body, "secretpass") || strings.Contains(body, "wrongpass") {
			t.Errorf("response body must not contain credential values, got: %s", body)
		}
	})
}

func TestHTTPAdapter_AsyncAndSyncHandlerPrecedence(t *testing.T) {
	t.Run("async handler takes precedence over sync handler", func(t *testing.T) {
		webhookSource := &porttest.MockWebhookSource{
			MockAlertSource: &porttest.MockAlertSource{NameVal: "prometheus", SourceTypeVal: port.SourceTypeWebhook},
			PathVal:         "/alerts/prometheus",
			HandleFunc: func(_ context.Context, _ []byte) ([]*entity.Alert, error) {
				alert, _ := entity.NewAlert("alert-1", "prometheus", "critical", "High CPU")
				return []*entity.Alert{alert}, nil
			},
		}
		manager := &porttest.MockAlertSourceManager{SourcesVal: []port.AlertSource{webhookSource}}
		adapter := NewHTTPAdapter(manager, DefaultConfig(), logger.NewNop())

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
