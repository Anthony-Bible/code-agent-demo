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

// TestConfig_ExtendedThinkingDefaults verifies that Defaults() includes proper extended thinking configuration.
func TestConfig_ExtendedThinkingDefaults(t *testing.T) {
	t.Run("ExtendedThinking defaults to false", func(t *testing.T) {
		cfg := Defaults()

		assert.False(t, cfg.ExtendedThinking,
			"ExtendedThinking should default to false")
	})

	t.Run("ThinkingBudget defaults to 10000", func(t *testing.T) {
		cfg := Defaults()

		assert.Equal(t, int64(10000), cfg.ThinkingBudget,
			"ThinkingBudget should default to 10000")
	})

	t.Run("ShowThinking defaults to false", func(t *testing.T) {
		cfg := Defaults()

		assert.False(t, cfg.ShowThinking,
			"ShowThinking should default to false")
	})

	t.Run("MaxTokens defaults to 20000", func(t *testing.T) {
		cfg := Defaults()

		assert.Equal(t, int64(20000), cfg.MaxTokens,
			"MaxTokens should default to 20000 (not hardcoded 4096)")
	})
}

// TestConfig_ExtendedThinkingEnvironmentVariables verifies environment variable overrides for extended thinking.
func TestConfig_ExtendedThinkingEnvironmentVariables(t *testing.T) {
	resetViper := func() {
		viper.Reset()
	}

	t.Run("AGENT_THINKING_ENABLED overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_THINKING_ENABLED", "true")

		cfg := LoadConfig()

		assert.True(t, cfg.ExtendedThinking,
			"AGENT_THINKING_ENABLED should override the default extended thinking setting")
	})

	t.Run("AGENT_THINKING_BUDGET overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_THINKING_BUDGET", "15000")

		cfg := LoadConfig()

		assert.Equal(t, int64(15000), cfg.ThinkingBudget,
			"AGENT_THINKING_BUDGET should override the default thinking budget")
	})

	t.Run("AGENT_SHOW_THINKING overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_SHOW_THINKING", "true")

		cfg := LoadConfig()

		assert.True(t, cfg.ShowThinking,
			"AGENT_SHOW_THINKING should override the default show thinking setting")
	})

	t.Run("AGENT_MAX_TOKENS overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_MAX_TOKENS", "30000")

		cfg := LoadConfig()

		assert.Equal(t, int64(30000), cfg.MaxTokens,
			"AGENT_MAX_TOKENS should override the default max tokens")
	})

	t.Run("all extended thinking environment variables can be set together", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_THINKING_ENABLED", "true")
		t.Setenv("AGENT_THINKING_BUDGET", "25000")
		t.Setenv("AGENT_SHOW_THINKING", "true")
		t.Setenv("AGENT_MAX_TOKENS", "50000")

		cfg := LoadConfig()

		assert.True(t, cfg.ExtendedThinking,
			"ExtendedThinking should be set from environment variable")
		assert.Equal(t, int64(25000), cfg.ThinkingBudget,
			"ThinkingBudget should be set from environment variable")
		assert.True(t, cfg.ShowThinking,
			"ShowThinking should be set from environment variable")
		assert.Equal(t, int64(50000), cfg.MaxTokens,
			"MaxTokens should be set from environment variable")
	})
}

// TestConfig_ThinkingBudgetValidation verifies validation of thinking budget values.
func TestConfig_ThinkingBudgetValidation(t *testing.T) {
	resetViper := func() {
		viper.Reset()
	}

	tests := []struct {
		name           string
		budgetValue    string
		expectedBudget int64
		description    string
	}{
		{
			name:           "budget below 1024 is capped at 1024",
			budgetValue:    "512",
			expectedBudget: 1024,
			description:    "thinking budget below 1024 should be capped at minimum 1024",
		},
		{
			name:           "budget of 500 is capped at 1024",
			budgetValue:    "500",
			expectedBudget: 1024,
			description:    "thinking budget of 500 should be capped at minimum 1024",
		},
		{
			name:           "budget of exactly 1024 is preserved",
			budgetValue:    "1024",
			expectedBudget: 1024,
			description:    "thinking budget of exactly 1024 should be preserved",
		},
		{
			name:           "budget above 1024 is preserved",
			budgetValue:    "5000",
			expectedBudget: 5000,
			description:    "thinking budget above 1024 should be preserved",
		},
		{
			name:           "zero budget uses default of 10000",
			budgetValue:    "0",
			expectedBudget: 10000,
			description:    "zero thinking budget should fall back to default of 10000",
		},
		{
			name:           "negative budget uses default of 10000",
			budgetValue:    "-100",
			expectedBudget: 10000,
			description:    "negative thinking budget should fall back to default of 10000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			defer resetViper()

			t.Setenv("AGENT_THINKING_BUDGET", tt.budgetValue)

			cfg := LoadConfig()

			assert.Equal(t, tt.expectedBudget, cfg.ThinkingBudget, tt.description)
		})
	}
}

// TestConfig_ExtendedThinkingFieldsExist verifies that Config struct has required extended thinking fields.
func TestConfig_ExtendedThinkingFieldsExist(t *testing.T) {
	t.Run("Config has ExtendedThinking field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if ExtendedThinking field doesn't exist
		cfg.ExtendedThinking = true
		require.NotNil(t, cfg)
	})

	t.Run("Config has ThinkingBudget field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if ThinkingBudget field doesn't exist
		cfg.ThinkingBudget = int64(5000)
		require.NotNil(t, cfg)
	})

	t.Run("Config has ShowThinking field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if ShowThinking field doesn't exist
		cfg.ShowThinking = true
		require.NotNil(t, cfg)
	})

	t.Run("Config has MaxTokens as int64", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if MaxTokens field doesn't exist or isn't int64
		cfg.MaxTokens = int64(20000)
		require.NotNil(t, cfg)
	})
}
