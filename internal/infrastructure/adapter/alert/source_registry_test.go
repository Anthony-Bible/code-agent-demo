package alert

import (
	"code-editing-agent/internal/domain/port"
	"sync"
	"testing"
)

func TestNewSourceRegistry(t *testing.T) {
	registry := NewSourceRegistry()
	if registry == nil {
		t.Fatal("NewSourceRegistry() returned nil")
	}
	if registry.factories == nil {
		t.Error("NewSourceRegistry() did not initialize factories map")
	}
}

func TestSourceRegistry_RegisterFactory(t *testing.T) {
	registry := NewSourceRegistry()

	registry.RegisterFactory("test", NewPrometheusSource)

	// Verify the factory was registered
	types := registry.SupportedTypes()
	if len(types) != 1 {
		t.Errorf("SupportedTypes() = %v, want 1 type", types)
	}
	if types[0] != "test" {
		t.Errorf("SupportedTypes()[0] = %v, want 'test'", types[0])
	}
}

func TestSourceRegistry_CreateSource_UnknownType(t *testing.T) {
	registry := NewSourceRegistry()

	_, err := registry.CreateSource(SourceConfig{
		Type:        "unknown",
		Name:        "test",
		WebhookPath: "/test",
	})

	if err == nil {
		t.Error("CreateSource() with unknown type should return error")
	}
	if !containsIgnoreCase(err.Error(), "unknown source type") {
		t.Errorf("CreateSource() error = %v, should contain 'unknown source type'", err)
	}
}

func TestSourceRegistry_CreateSource_ValidPrometheus(t *testing.T) {
	registry := NewSourceRegistry()
	registry.RegisterFactory("prometheus", NewPrometheusSource)

	source, err := registry.CreateSource(SourceConfig{
		Type:        "prometheus",
		Name:        "test-prom",
		WebhookPath: "/alerts/prom",
	})
	if err != nil {
		t.Errorf("CreateSource() error = %v, want nil", err)
	}
	if source == nil {
		t.Error("CreateSource() returned nil source without error")
	}
}

func TestSourceRegistry_CreateSource_ValidGCP(t *testing.T) {
	registry := NewSourceRegistry()
	registry.RegisterFactory("gcp_monitoring", NewGCPMonitoringSource)

	source, err := registry.CreateSource(SourceConfig{
		Type:        "gcp_monitoring",
		Name:        "test-gcp",
		WebhookPath: "/alerts/gcp",
	})
	if err != nil {
		t.Errorf("CreateSource() error = %v, want nil", err)
	}
	if source == nil {
		t.Error("CreateSource() returned nil source without error")
	}
}

func TestSourceRegistry_CreateSource_FactoryError(t *testing.T) {
	registry := NewSourceRegistry()
	registry.RegisterFactory("prometheus", NewPrometheusSource)

	source, err := registry.CreateSource(SourceConfig{
		Type:        "prometheus",
		Name:        "", // Invalid: empty name
		WebhookPath: "/alerts/prom",
	})

	if err == nil {
		t.Error("CreateSource() with invalid config should return error")
	}
	if source != nil {
		t.Error("CreateSource() returned non-nil source on error")
	}
}

func TestSourceRegistry_SupportedTypes(t *testing.T) {
	registry := NewSourceRegistry()

	// Initially empty
	types := registry.SupportedTypes()
	if len(types) != 0 {
		t.Errorf("SupportedTypes() = %v, want empty slice", types)
	}

	// Register multiple factories
	registry.RegisterFactory("prometheus", NewPrometheusSource)
	registry.RegisterFactory("gcp_monitoring", NewGCPMonitoringSource)
	registry.RegisterFactory("zebra", NewPrometheusSource) // To test sorting

	types = registry.SupportedTypes()
	if len(types) != 3 {
		t.Errorf("SupportedTypes() length = %d, want 3", len(types))
	}

	// Verify sorted order
	expected := []string{"gcp_monitoring", "prometheus", "zebra"}
	for i, expectedType := range expected {
		if types[i] != expectedType {
			t.Errorf("SupportedTypes()[%d] = %v, want %v", i, types[i], expectedType)
		}
	}
}

func TestSourceRegistry_RegisterBuiltinFactories(t *testing.T) {
	registry := NewSourceRegistry()
	registry.RegisterBuiltinFactories()

	types := registry.SupportedTypes()
	if len(types) < 2 {
		t.Errorf("RegisterBuiltinFactories() registered %d types, want at least 2", len(types))
	}

	// Verify prometheus is registered
	hasPrometheus := false
	hasGCP := false
	for _, typ := range types {
		if typ == "prometheus" {
			hasPrometheus = true
		}
		if typ == "gcp_monitoring" {
			hasGCP = true
		}
	}

	if !hasPrometheus {
		t.Error("RegisterBuiltinFactories() did not register 'prometheus'")
	}
	if !hasGCP {
		t.Error("RegisterBuiltinFactories() did not register 'gcp_monitoring'")
	}
}

func TestSourceRegistry_ThreadSafety(_ *testing.T) {
	registry := NewSourceRegistry()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrently register factories
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(_ int) {
			defer wg.Done()
			registry.RegisterFactory("prometheus", NewPrometheusSource)
			_ = registry.SupportedTypes()
		}(i)
	}

	// Concurrently create sources
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(_ int) {
			defer wg.Done()
			_, _ = registry.CreateSource(SourceConfig{
				Type:        "prometheus",
				Name:        "test",
				WebhookPath: "/test",
			})
		}(i)
	}

	wg.Wait()
}

func TestSourceRegistry_FactoryReplacement(t *testing.T) {
	registry := NewSourceRegistry()

	// Register initial factory
	called1 := false
	factory1 := func(cfg SourceConfig) (port.AlertSource, error) {
		called1 = true
		return NewPrometheusSource(cfg)
	}
	registry.RegisterFactory("test", factory1)

	// Replace with second factory
	called2 := false
	factory2 := func(cfg SourceConfig) (port.AlertSource, error) {
		called2 = true
		return NewPrometheusSource(cfg)
	}
	registry.RegisterFactory("test", factory2)

	// Create a source - should use factory2
	_, _ = registry.CreateSource(SourceConfig{
		Type:        "test",
		Name:        "test",
		WebhookPath: "/test",
	})

	if called1 {
		t.Error("CreateSource() called replaced factory")
	}
	if !called2 {
		t.Error("CreateSource() did not call current factory")
	}
}
