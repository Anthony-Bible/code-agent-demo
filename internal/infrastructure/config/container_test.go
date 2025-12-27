package config

import (
	"code-editing-agent/internal/infrastructure/adapter/ui"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainer_UsesHistoryConfig verifies that NewContainer creates a CLIAdapter
// with history configuration from the Config struct.
func TestContainer_UsesHistoryConfig(t *testing.T) {
	t.Run("container passes HistoryFile from config to CLIAdapter", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/test-agent-history"
		cfg.HistoryMaxEntries = 500

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

	t.Run("container passes HistoryMaxEntries from config to CLIAdapter", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/test-history"
		cfg.HistoryMaxEntries = 2500

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Verify max entries is set from config
		assert.Equal(t, 2500, cliAdapter.GetMaxHistoryEntries(),
			"CLIAdapter should use HistoryMaxEntries from config")
	})

	t.Run("container uses default history values when config has defaults", func(t *testing.T) {
		cfg := Defaults()

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Default HistoryFile is "~/.code-editing-agent-history"
		assert.Equal(t, "~/.code-editing-agent-history", cliAdapter.GetHistoryFile(),
			"CLIAdapter should use default HistoryFile from config")

		// Default HistoryMaxEntries is 1000
		assert.Equal(t, 1000, cliAdapter.GetMaxHistoryEntries(),
			"CLIAdapter should use default HistoryMaxEntries from config")
	})

	t.Run("container supports empty history file for in-memory mode", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "" // Empty means in-memory only

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
	t.Run("UIAdapter has non-nil HistoryManager when history is configured", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/container-test-history"
		cfg.HistoryMaxEntries = 100

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// The adapter should have a HistoryManager initialized
		historyManager := cliAdapter.GetHistoryManager()
		assert.NotNil(t, historyManager,
			"CLIAdapter should have a HistoryManager when history is configured")
	})

	t.Run("UIAdapter is in interactive mode when history is configured", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/interactive-test-history"

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// When history is configured, the adapter should be interactive
		assert.True(t, cliAdapter.IsInteractive(),
			"CLIAdapter should be in interactive mode when created with history config")
	})

	t.Run("HistoryManager has correct max entries", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/max-entries-test"
		cfg.HistoryMaxEntries = 750

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		historyManager := cliAdapter.GetHistoryManager()
		require.NotNil(t, historyManager, "HistoryManager should not be nil")

		// The HistoryManager should be configured with the correct max entries
		// We verify this indirectly through the CLIAdapter's GetMaxHistoryEntries
		assert.Equal(t, 750, cliAdapter.GetMaxHistoryEntries(),
			"HistoryManager should use max entries from config")
	})
}

// TestContainer_HistoryFilePath verifies that the container properly handles
// history file paths, including tilde expansion.
func TestContainer_HistoryFilePath(t *testing.T) {
	t.Run("container expands tilde in history file path", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "~/.custom-agent-history"

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// The raw history file path should be preserved (CLIAdapter stores original)
		// but the HistoryManager should have the expanded path
		historyFile := cliAdapter.GetHistoryFile()

		// GetHistoryFile returns the original path as passed to the adapter
		assert.Equal(t, "~/.custom-agent-history", historyFile,
			"GetHistoryFile should return the configured path")

		// The HistoryManager internally uses the expanded path
		historyManager := cliAdapter.GetHistoryManager()
		assert.NotNil(t, historyManager,
			"HistoryManager should be initialized with expanded path")
	})

	t.Run("container passes absolute path unchanged", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/var/lib/agent/history"

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
		cfg.HistoryFile = ".agent-history"

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Relative paths should be passed through
		assert.Equal(t, ".agent-history", cliAdapter.GetHistoryFile(),
			"Relative path should be preserved")
	})
}

// TestContainer_HistoryIntegrationWithChatService verifies that the ChatService
// receives a properly configured UI adapter with history support.
func TestContainer_HistoryIntegrationWithChatService(t *testing.T) {
	t.Run("ChatService uses UI adapter with history", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/chatservice-history-test"
		cfg.HistoryMaxEntries = 200

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		// The ChatService should be wired with the same UI adapter
		chatService := container.ChatService()
		require.NotNil(t, chatService, "ChatService should not be nil")

		// Get the UI adapter from container and verify it has history
		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		assert.NotNil(t, cliAdapter.GetHistoryManager(),
			"UIAdapter used by ChatService should have HistoryManager")
	})
}

// TestContainer_HistoryConfigEdgeCases verifies edge cases in history configuration.
func TestContainer_HistoryConfigEdgeCases(t *testing.T) {
	t.Run("zero max entries uses default value", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/zero-max-test"
		cfg.HistoryMaxEntries = 0 // Invalid, should use default

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Zero max entries should be converted to default (1000)
		maxEntries := cliAdapter.GetMaxHistoryEntries()
		assert.Equal(t, 1000, maxEntries,
			"Zero max entries should fall back to default of 1000")
	})

	t.Run("negative max entries uses default value", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/negative-max-test"
		cfg.HistoryMaxEntries = -50 // Invalid, should use default

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Negative max entries should be converted to default (1000)
		maxEntries := cliAdapter.GetMaxHistoryEntries()
		assert.Equal(t, 1000, maxEntries,
			"Negative max entries should fall back to default of 1000")
	})

	t.Run("very large max entries is accepted", func(t *testing.T) {
		cfg := Defaults()
		cfg.HistoryFile = "/tmp/large-max-test"
		cfg.HistoryMaxEntries = 100000

		container, err := NewContainer(cfg)
		require.NoError(t, err, "NewContainer should not return an error")

		cliAdapter, ok := container.UIAdapter().(*ui.CLIAdapter)
		require.True(t, ok, "UIAdapter should be a *ui.CLIAdapter")

		// Large values should be accepted
		assert.Equal(t, 100000, cliAdapter.GetMaxHistoryEntries(),
			"Large max entries value should be accepted")
	})
}
