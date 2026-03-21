package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/safety"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/ui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainer_UsesHistoryConfig verifies that NewContainer creates a CLIAdapter
// with history configuration from the Config struct.
func TestContainer_UsesHistoryConfig(t *testing.T) {
	t.Run("container passes HistoryFile from config to CLIAdapter", func(t *testing.T) {
		cfg := Defaults()
		cfg.History.File = "/tmp/test-agent-history"
		cfg.History.MaxEntries = 500

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")
		require.NotNil(t, container, "container should not be nil")

		// Get the UI adapter and assert it's a CLIAdapter
		uiAdapter := container.UIAdapter()
		require.NotNil(t, uiAdapter, "UIAdapter should not be nil")

		cliAdapter, ok := uiAdapter.(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Verify history file is set from config
		assert.Equal(t, "/tmp/test-agent-history", cliAdapter.GetHistoryFile(),
			"CLIAdapter should use HistoryFile from config")
	})

	t.Run("container uses default history values when config has defaults", func(t *testing.T) {
		cfg := Defaults()

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Default HistoryFile gets expanded from "~/.code-agent-demo-history"
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err, "should be able to get home directory")
		expectedPath := filepath.Join(homeDir, ".code-agent-demo-history")
		assert.Equal(t, expectedPath, cliAdapter.GetHistoryFile(),
			"CLIAdapter should expand ~ in HistoryFile from config")
	})

	t.Run("container supports empty history file for in-memory mode", func(t *testing.T) {
		cfg := Defaults()
		cfg.History.File = "" // Empty means in-memory only

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error with empty history file")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Empty history file should be preserved (in-memory only mode)
		assert.Empty(t, cliAdapter.GetHistoryFile(),
			"CLIAdapter should accept empty HistoryFile for in-memory mode")
	})
}

// TestContainer_UIAdapterHasHistory verifies that the UI adapter returned by
// container is configured with a HistoryManager for interactive use.
func TestContainer_UIAdapterHasHistory(t *testing.T) {
	t.Run("UIAdapter is in interactive mode when history is configured", func(t *testing.T) {
		cfg := Defaults()
		cfg.History.File = "/tmp/interactive-test-history"

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// When history is configured, the adapter should be interactive
		assert.True(t, cliAdapter.IsInteractive(),
			"CLIAdapter should be in interactive mode when created with history config")
	})
}

// TestContainer_HistoryFilePath verifies that the container properly handles
// history file paths, including tilde expansion.
func TestContainer_HistoryFilePath(t *testing.T) {
	t.Run("container passes absolute path unchanged", func(t *testing.T) {
		cfg := Defaults()
		cfg.History.File = "/var/lib/agent/history"

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Absolute paths should be passed through unchanged
		assert.Equal(t, "/var/lib/agent/history", cliAdapter.GetHistoryFile(),
			"Absolute path should be preserved")
	})

	t.Run("container handles relative path", func(t *testing.T) {
		cfg := Defaults()
		cfg.History.File = ".agent-history"

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Relative paths should be passed through
		assert.Equal(t, ".agent-history", cliAdapter.GetHistoryFile(),
			"Relative path should be preserved")
	})
}

// TestContainer_HistoryIntegrationWithUIAdapter verifies that the UI adapter
// is properly configured with history support.
func TestContainer_HistoryIntegrationWithUIAdapter(t *testing.T) {
	t.Run("UIAdapter has history configured", func(t *testing.T) {
		cfg := Defaults()
		cfg.History.File = "/tmp/uiadapter-history-test"
		cfg.History.MaxEntries = 200

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		// Get the UI adapter from container and verify it's configured
		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Verify history file is configured
		assert.Equal(t, "/tmp/uiadapter-history-test", cliAdapter.GetHistoryFile(),
			"UIAdapter should have history file configured")
	})
}

// TestBuildWhitelistPatterns verifies that buildWhitelistPatterns respects
// the CommandWhitelistOverride setting.
func TestBuildWhitelistPatterns(t *testing.T) {
	t.Run("override=false extends defaults with custom patterns", func(t *testing.T) {
		cfg := Defaults()
		cfg.Safety.CommandValidationMode = "whitelist"
		cfg.Safety.CommandWhitelistOverride = false
		cfg.Safety.CommandWhitelistJSON = `[{"pattern": "^mycustom(\\s|$)", "description": "custom"}]`

		patterns, err := buildWhitelistPatterns(cfg)
		require.NoError(t, err)

		// Should have defaults + custom pattern
		defaultCount := len(safety.DefaultWhitelistPatterns())
		assert.Greater(t, len(patterns), defaultCount,
			"should have more patterns than defaults")

		// Last pattern should be our custom one
		lastPattern := patterns[len(patterns)-1]
		assert.Equal(t, "custom", lastPattern.Description)
	})

	t.Run("override=true uses only custom patterns", func(t *testing.T) {
		cfg := Defaults()
		cfg.Safety.CommandValidationMode = "whitelist"
		cfg.Safety.CommandWhitelistOverride = true
		cfg.Safety.CommandWhitelistJSON = `[{"pattern": "^mycustom(\\s|$)", "description": "custom"}]`

		patterns, err := buildWhitelistPatterns(cfg)
		require.NoError(t, err)

		// Should have only our custom pattern
		assert.Len(t, patterns, 1, "should have only custom pattern")
		assert.Equal(t, "custom", patterns[0].Description)
	})

	t.Run("override=true with no JSON returns empty patterns", func(t *testing.T) {
		cfg := Defaults()
		cfg.Safety.CommandValidationMode = "whitelist"
		cfg.Safety.CommandWhitelistOverride = true
		cfg.Safety.CommandWhitelistJSON = ""

		patterns, err := buildWhitelistPatterns(cfg)
		require.NoError(t, err)

		// Should have no patterns (blocks all commands)
		assert.Empty(t, patterns, "should have no patterns")
	})

	t.Run("override=false with no JSON uses defaults", func(t *testing.T) {
		cfg := Defaults()
		cfg.Safety.CommandValidationMode = "whitelist"
		cfg.Safety.CommandWhitelistOverride = false
		cfg.Safety.CommandWhitelistJSON = ""

		patterns, err := buildWhitelistPatterns(cfg)
		require.NoError(t, err)

		// Should have exactly the defaults
		assert.Len(t, patterns, len(safety.DefaultWhitelistPatterns()),
			"should have exactly default patterns")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		cfg := Defaults()
		cfg.Safety.CommandValidationMode = "whitelist"
		cfg.Safety.CommandWhitelistJSON = `[{"pattern": "invalid json`

		_, err := buildWhitelistPatterns(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AGENT_SAFETY_COMMAND_WHITELIST_JSON")
	})
}
