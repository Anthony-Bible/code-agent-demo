// Package config provides configuration loading and dependency injection for the application.
// This file handles loading alert source configurations from YAML files.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AlertSourceConfig represents the configuration for a single alert source.
type AlertSourceConfig struct {
	// Type is the alert source type (e.g., "prometheus", "grafana").
	Type string `yaml:"type"`
	// Name is the unique identifier for this source instance.
	Name string `yaml:"name"`
	// WebhookPath is the HTTP path for receiving webhooks.
	WebhookPath string `yaml:"webhook_path"`
	// Extra contains additional source-specific configuration options.
	Extra map[string]string `yaml:"extra,omitempty"`
}

// WebhookServerConfig represents the webhook server configuration.
type WebhookServerConfig struct {
	// Addr is the address to listen on (e.g., ":8080").
	Addr string `yaml:"addr"`
	// Sources is the list of alert sources to register.
	Sources []AlertSourceConfig `yaml:"sources"`
}

// LoadAlertSourcesConfig loads the webhook server configuration from a YAML file.
// Returns an error if the file cannot be read or parsed.
func LoadAlertSourcesConfig(path string) (*WebhookServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config WebhookServerConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Addr == "" {
		config.Addr = ":8080"
	}

	return &config, nil
}

// LoadAlertSourcesConfigWithDefaults loads config from a file, falling back to defaults if the file doesn't exist.
func LoadAlertSourcesConfigWithDefaults(path string) (*WebhookServerConfig, error) {
	config, err := LoadAlertSourcesConfig(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return &WebhookServerConfig{
				Addr:    ":8080",
				Sources: []AlertSourceConfig{},
			}, nil
		}
		return nil, err
	}
	return config, nil
}
