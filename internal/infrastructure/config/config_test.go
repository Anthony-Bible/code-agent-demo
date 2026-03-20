package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_HistoryDefaults verifies that Defaults() includes proper history configuration.
func TestConfig_HistoryDefaults(t *testing.T) {
	t.Run("History.File has default value", func(t *testing.T) {
		cfg := Defaults()

		// History.File should default to ~/.code-agent-demo-history
		assert.Equal(t, "~/.code-agent-demo-history", cfg.History.File,
			"History.File should default to ~/.code-agent-demo-history")
	})

	t.Run("History.MaxEntries has default value of 1000", func(t *testing.T) {
		cfg := Defaults()

		// History.MaxEntries should default to 1000
		assert.Equal(t, 1000, cfg.History.MaxEntries,
			"History.MaxEntries should default to 1000")
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

		assert.Equal(t, customPath, cfg.History.File,
			"AGENT_HISTORY_FILE should override the default history file path")
	})

	t.Run("AGENT_HISTORY_MAX_ENTRIES overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_HISTORY_MAX_ENTRIES", "5000")

		cfg := LoadConfig()

		assert.Equal(t, 5000, cfg.History.MaxEntries,
			"AGENT_HISTORY_MAX_ENTRIES should override the default max entries")
	})

	t.Run("both history environment variables can be set together", func(t *testing.T) {
		resetViper()
		defer resetViper()

		customPath := "/tmp/agent-history"
		t.Setenv("AGENT_HISTORY_FILE", customPath)
		t.Setenv("AGENT_HISTORY_MAX_ENTRIES", "250")

		cfg := LoadConfig()

		assert.Equal(t, customPath, cfg.History.File,
			"History.File should be set from environment variable")
		assert.Equal(t, 250, cfg.History.MaxEntries,
			"History.MaxEntries should be set from environment variable")
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

		assert.Equal(t, 1000, cfg.History.MaxEntries,
			"zero max entries should fall back to default of 1000")
	})

	t.Run("negative max entries uses default of 1000", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_HISTORY_MAX_ENTRIES", "-100")

		cfg := LoadConfig()

		assert.Equal(t, 1000, cfg.History.MaxEntries,
			"negative max entries should fall back to default of 1000")
	})

	t.Run("empty history file is valid for in-memory only mode", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_HISTORY_FILE", "")

		cfg := LoadConfig()

		assert.Empty(t, cfg.History.File,
			"empty history file should be allowed for in-memory only mode")
	})
}

// TestConfig_HistoryFieldsExist verifies that Config struct has required history fields.
func TestConfig_HistoryFieldsExist(t *testing.T) {
	t.Run("Config has History.File field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if History.File field doesn't exist
		cfg.History.File = "/some/path"
		require.NotNil(t, cfg)
	})

	t.Run("Config has History.MaxEntries field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if History.MaxEntries field doesn't exist
		cfg.History.MaxEntries = 500
		require.NotNil(t, cfg)
	})
}

// TestConfig_ExtendedThinkingDefaults verifies that Defaults() includes proper extended thinking configuration.
func TestConfig_ExtendedThinkingDefaults(t *testing.T) {
	t.Run("Thinking.Enabled defaults to false", func(t *testing.T) {
		cfg := Defaults()

		assert.False(t, cfg.Thinking.Enabled,
			"Thinking.Enabled should default to false")
	})

	t.Run("Thinking.Budget defaults to 10000", func(t *testing.T) {
		cfg := Defaults()

		assert.Equal(t, int64(10000), cfg.Thinking.Budget,
			"Thinking.Budget should default to 10000")
	})

	t.Run("Thinking.Show defaults to false", func(t *testing.T) {
		cfg := Defaults()

		assert.False(t, cfg.Thinking.Show,
			"Thinking.Show should default to false")
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

		assert.True(t, cfg.Thinking.Enabled,
			"AGENT_THINKING_ENABLED should override the default extended thinking setting")
	})

	t.Run("AGENT_THINKING_BUDGET overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_THINKING_BUDGET", "15000")

		cfg := LoadConfig()

		assert.Equal(t, int64(15000), cfg.Thinking.Budget,
			"AGENT_THINKING_BUDGET should override the default thinking budget")
	})

	t.Run("AGENT_THINKING_SHOW overrides default", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_THINKING_SHOW", "true")

		cfg := LoadConfig()

		assert.True(t, cfg.Thinking.Show,
			"AGENT_THINKING_SHOW should override the default show thinking setting")
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
		t.Setenv("AGENT_THINKING_SHOW", "true")
		t.Setenv("AGENT_MAX_TOKENS", "50000")

		cfg := LoadConfig()

		assert.True(t, cfg.Thinking.Enabled,
			"Thinking.Enabled should be set from environment variable")
		assert.Equal(t, int64(25000), cfg.Thinking.Budget,
			"Thinking.Budget should be set from environment variable")
		assert.True(t, cfg.Thinking.Show,
			"Thinking.Show should be set from environment variable")
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

			assert.Equal(t, tt.expectedBudget, cfg.Thinking.Budget, tt.description)
		})
	}
}

// TestConfig_ExtendedThinkingFieldsExist verifies that Config struct has required extended thinking fields.
func TestConfig_ExtendedThinkingFieldsExist(t *testing.T) {
	t.Run("Config has Thinking.Enabled field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if Thinking.Enabled field doesn't exist
		cfg.Thinking.Enabled = true
		require.NotNil(t, cfg)
	})

	t.Run("Config has Thinking.Budget field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if Thinking.Budget field doesn't exist
		cfg.Thinking.Budget = int64(5000)
		require.NotNil(t, cfg)
	})

	t.Run("Config has Thinking.Show field", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if Thinking.Show field doesn't exist
		cfg.Thinking.Show = true
		require.NotNil(t, cfg)
	})

	t.Run("Config has MaxTokens as int64", func(t *testing.T) {
		cfg := &Config{}

		// This will fail to compile if MaxTokens field doesn't exist or isn't int64
		cfg.MaxTokens = int64(20000)
		require.NotNil(t, cfg)
	})
}

// TestConfig_CommandWhitelistOverride verifies CommandWhitelistOverride default and env var loading.
func TestConfig_CommandWhitelistOverride(t *testing.T) {
	resetViper := func() {
		viper.Reset()
	}

	t.Run("defaults to false", func(t *testing.T) {
		cfg := Defaults()
		assert.False(t, cfg.Safety.CommandWhitelistOverride,
			"CommandWhitelistOverride should default to false")
	})

	t.Run("AGENT_SAFETY_COMMAND_WHITELIST_OVERRIDE=true sets to true", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_SAFETY_COMMAND_WHITELIST_OVERRIDE", "true")

		cfg := LoadConfig()

		assert.True(t, cfg.Safety.CommandWhitelistOverride,
			"AGENT_SAFETY_COMMAND_WHITELIST_OVERRIDE=true should set CommandWhitelistOverride to true")
	})

	t.Run("AGENT_SAFETY_COMMAND_WHITELIST_OVERRIDE=false keeps false", func(t *testing.T) {
		resetViper()
		defer resetViper()

		t.Setenv("AGENT_SAFETY_COMMAND_WHITELIST_OVERRIDE", "false")

		cfg := LoadConfig()

		assert.False(t, cfg.Safety.CommandWhitelistOverride,
			"AGENT_SAFETY_COMMAND_WHITELIST_OVERRIDE=false should set CommandWhitelistOverride to false")
	})
}
