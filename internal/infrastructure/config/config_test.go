package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_HistoryDefaults verifies that Defaults() includes proper history configuration.
func TestConfig_HistoryDefaults(t *testing.T) {
	t.Run("HistoryFile has default value", func(t *testing.T) {
		cfg := Defaults()

		// HistoryFile should default to ~/.code-editing-agent-history
		assert.Equal(t, "~/.code-editing-agent-history", cfg.HistoryFile,
			"HistoryFile should default to ~/.code-editing-agent-history")
	})

	t.Run("HistoryMaxEntries has default value of 1000", func(t *testing.T) {
		cfg := Defaults()

		// HistoryMaxEntries should default to 1000
		assert.Equal(t, 1000, cfg.HistoryMaxEntries,
			"HistoryMaxEntries should default to 1000")
	})
}

// TestConfig_HistoryEnvironmentVariables verifies environment variable overrides.
func TestConfig_HistoryEnvironmentVariables(t *testing.T) {
	// Helper to reset viper between tests
	resetViper := func() {
		viper.Reset()
	}

	t.Run("AGENT_HISTORY_FILE overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		customPath := "/custom/path/to/history"
		t.Setenv("AGENT_HISTORY_FILE", customPath)

		cfg := LoadConfig()

		assert.Equal(t, customPath, cfg.HistoryFile,
			"AGENT_HISTORY_FILE should override the default history file path")
	})

	t.Run("AGENT_HISTORY_MAX_ENTRIES overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_HISTORY_MAX_ENTRIES", "5000")

		cfg := LoadConfig()

		assert.Equal(t, 5000, cfg.HistoryMaxEntries,
			"AGENT_HISTORY_MAX_ENTRIES should override the default max entries")
	})

	t.Run("both history environment variables can be set together", func(t *testing.T) {
		resetViper()
		defer resetViper()

		customPath := "/tmp/agent-history"
		t.Setenv("AGENT_HISTORY_FILE", customPath)
		t.Setenv("AGENT_HISTORY_MAX_ENTRIES", "250")

		cfg := LoadConfig()

		assert.Equal(t, customPath, cfg.HistoryFile,
			"HistoryFile should be set from environment variable")
		assert.Equal(t, 250, cfg.HistoryMaxEntries,
			"HistoryMaxEntries should be set from environment variable")
	})
}

// TestConfig_HistoryValidation verifies validation of history configuration values.
func TestConfig_HistoryValidation(t *testing.T) {
	resetViper := func() {
		viper.Reset()
	}

	t.Run("zero max entries uses default of 1000", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_HISTORY_MAX_ENTRIES", "0")

		cfg := LoadConfig()

		assert.Equal(t, 1000, cfg.HistoryMaxEntries,
			"zero max entries should fall back to default of 1000")
	})

	t.Run("negative max entries uses default of 1000", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_HISTORY_MAX_ENTRIES", "-100")

		cfg := LoadConfig()

		assert.Equal(t, 1000, cfg.HistoryMaxEntries,
			"negative max entries should fall back to default of 1000")
	})

	t.Run("empty history file is valid for in-memory only mode", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_HISTORY_FILE", "")

		cfg := LoadConfig()

		assert.Empty(t, cfg.HistoryFile,
			"empty history file should be allowed for in-memory only mode")
	})
}

// TestConfig_HistoryFieldsExist verifies that Config struct has required history fields.
func TestConfig_HistoryFieldsExist(t *testing.T) {
	t.Run("Config has HistoryFile field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if HistoryFile field doesn't exist
		cfg.HistoryFile = "/some/path"
		require.NotNil(t, cfg)
	})

	t.Run("Config has HistoryMaxEntries field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if HistoryMaxEntries field doesn't exist
		cfg.HistoryMaxEntries = 500
		require.NotNil(t, cfg)
	})
}
