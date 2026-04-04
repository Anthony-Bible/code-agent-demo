package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAlertSourcesConfig_BasicAuth(t *testing.T) {
	t.Run("loads source without basic_auth", func(t *testing.T) {
		yaml := `
addr: ":8080"
sources:
  - type: prometheus
    name: prom
    webhook_path: /alerts/prom
`
		cfg := writeAndLoad(t, yaml)

		if len(cfg.Sources) != 1 {
			t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
		}
		if cfg.Sources[0].BasicAuth != nil {
			t.Error("expected BasicAuth to be nil when not configured")
		}
	})

	t.Run("loads source with basic_auth credentials", func(t *testing.T) {
		yaml := `
addr: ":8080"
sources:
  - type: prometheus
    name: prom-secure
    webhook_path: /alerts/prom
    basic_auth:
      username: myuser
      password: mypass
`
		cfg := writeAndLoad(t, yaml)

		if len(cfg.Sources) != 1 {
			t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
		}
		src := cfg.Sources[0]
		if src.BasicAuth == nil {
			t.Fatal("expected BasicAuth to be set")
		}
		if src.BasicAuth.Username != "myuser" {
			t.Errorf("expected username 'myuser', got %q", src.BasicAuth.Username)
		}
		if src.BasicAuth.Password != "mypass" {
			t.Errorf("expected password 'mypass', got %q", src.BasicAuth.Password)
		}
	})

	t.Run("loads mixed sources: some with auth, some without", func(t *testing.T) {
		yaml := `
addr: ":8080"
sources:
  - type: prometheus
    name: prom-open
    webhook_path: /alerts/prom-open
  - type: gcp_monitoring
    name: gcp-secure
    webhook_path: /alerts/gcp
    basic_auth:
      username: gcpuser
      password: gcppass
`
		cfg := writeAndLoad(t, yaml)

		if len(cfg.Sources) != 2 {
			t.Fatalf("expected 2 sources, got %d", len(cfg.Sources))
		}

		// First source: no auth
		if cfg.Sources[0].BasicAuth != nil {
			t.Error("first source: expected BasicAuth to be nil")
		}

		// Second source: auth present
		if cfg.Sources[1].BasicAuth == nil {
			t.Fatal("second source: expected BasicAuth to be set")
		}
		if cfg.Sources[1].BasicAuth.Username != "gcpuser" {
			t.Errorf("second source username = %q, want 'gcpuser'", cfg.Sources[1].BasicAuth.Username)
		}
		if cfg.Sources[1].BasicAuth.Password != "gcppass" {
			t.Errorf("second source password = %q, want 'gcppass'", cfg.Sources[1].BasicAuth.Password)
		}
	})

	t.Run("backward compatible: existing configs without basic_auth still load", func(t *testing.T) {
		yaml := `
addr: ":9090"
sources:
  - type: prometheus
    name: legacy
    webhook_path: /alerts/prometheus
    extra:
      cluster: staging
`
		cfg := writeAndLoad(t, yaml)

		if cfg.Addr != ":9090" {
			t.Errorf("expected addr ':9090', got %q", cfg.Addr)
		}
		if len(cfg.Sources) != 1 {
			t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
		}
		if cfg.Sources[0].BasicAuth != nil {
			t.Error("expected BasicAuth nil for legacy config")
		}
		if cfg.Sources[0].Extra["cluster"] != "staging" {
			t.Error("expected extra fields to still be loaded")
		}
	})
}

// writeAndLoad writes yaml content to a temp file and calls LoadAlertSourcesConfig.
func writeAndLoad(t *testing.T, content string) *WebhookServerConfig {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "alert-sources.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	cfg, err := LoadAlertSourcesConfig(path)
	if err != nil {
		t.Fatalf("LoadAlertSourcesConfig() error = %v", err)
	}
	return cfg
}
