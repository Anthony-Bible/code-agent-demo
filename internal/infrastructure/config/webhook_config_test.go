package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAlertSourcesConfig_BasicAuth(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, cfg *WebhookServerConfig)
	}{
		{
			name: "loads source without basic_auth",
			yaml: `
addr: ":8080"
sources:
  - type: prometheus
    name: prom
    webhook_path: /alerts/prom
`,
			validate: func(t *testing.T, cfg *WebhookServerConfig) {
				if len(cfg.Sources) != 1 {
					t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
				}
				if cfg.Sources[0].BasicAuth != nil {
					t.Error("expected BasicAuth to be nil when not configured")
				}
			},
		},
		{
			name: "loads source with basic_auth credentials",
			yaml: `
addr: ":8080"
sources:
  - type: prometheus
    name: prom-secure
    webhook_path: /alerts/prom
    basic_auth:
      username: myuser
      password: mypass
`,
			validate: func(t *testing.T, cfg *WebhookServerConfig) {
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
			},
		},
		{
			name: "loads mixed sources: some with auth, some without",
			yaml: `
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
`,
			validate: func(t *testing.T, cfg *WebhookServerConfig) {
				if len(cfg.Sources) != 2 {
					t.Fatalf("expected 2 sources, got %d", len(cfg.Sources))
				}
				if cfg.Sources[0].BasicAuth != nil {
					t.Error("first source: expected BasicAuth to be nil")
				}
				if cfg.Sources[1].BasicAuth == nil {
					t.Fatal("second source: expected BasicAuth to be set")
				}
				if cfg.Sources[1].BasicAuth.Username != "gcpuser" {
					t.Errorf("second source username = %q, want 'gcpuser'", cfg.Sources[1].BasicAuth.Username)
				}
				if cfg.Sources[1].BasicAuth.Password != "gcppass" {
					t.Errorf("second source password = %q, want 'gcppass'", cfg.Sources[1].BasicAuth.Password)
				}
			},
		},
		{
			name: "backward compatible: existing configs without basic_auth still load",
			yaml: `
addr: ":9090"
sources:
  - type: prometheus
    name: legacy
    webhook_path: /alerts/prometheus
    extra:
      cluster: staging
`,
			validate: func(t *testing.T, cfg *WebhookServerConfig) {
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
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := writeAndLoad(t, tt.yaml)
			tt.validate(t, cfg)
		})
	}
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
