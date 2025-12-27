package ui_test

import (
	"code-editing-agent/internal/infrastructure/adapter/ui"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TDD Red Phase Tests for HistoryManager
// These tests define the expected behavior for command history persistence.
// All tests should FAIL initially until the implementation is complete.
// This is Cycle 1 focusing on core logic (no file I/O yet - that's Cycle 2).
// =============================================================================

// =============================================================================
// Test: NewHistoryManager
// =============================================================================

func TestNewHistoryManager(t *testing.T) {
	t.Run("with empty file path creates in-memory only manager", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		require.NotNil(t, hm, "should create a history manager with empty file path")

		// Verify it works in memory mode
		err := hm.Add("test entry")
		require.NoError(t, err, "should allow adding entries in memory-only mode")

		history := hm.History()
		require.Len(t, history, 1, "should have one entry")
		assert.Equal(t, "test entry", history[0], "should contain the added entry")
	})

	t.Run("with max entries zero creates unlimited history", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 0)

		require.NotNil(t, hm, "should create a history manager with maxEntries=0")

		// Add many unique entries to verify no limit is enforced
		for i := range 1000 {
			entry := "entry_" + string(rune('A'+i/26%26)) + string(rune('a'+i%26))
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		history := hm.History()
		assert.Len(t, history, 1000, "unlimited mode should keep all 1000 entries")
	})

	t.Run("with max entries greater than zero creates limited history", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 50)

		require.NotNil(t, hm, "should create a history manager with maxEntries=50")

		// Add more entries than the limit
		for i := range 100 {
			err := hm.Add("entry" + string(rune('0'+i%10)))
			require.NoError(t, err)
		}

		history := hm.History()
		assert.Len(t, history, 50, "should limit history to maxEntries=50")
	})

	t.Run("with valid file path stores the path", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, ".history")

		hm := ui.NewHistoryManager(filePath, 100)

		require.NotNil(t, hm, "should create a history manager with file path")

		// The manager should be created successfully even if file doesn't exist yet
		// (file creation/loading is Cycle 2)
	})

	t.Run("with negative max entries treats as zero (unlimited)", func(t *testing.T) {
		hm := ui.NewHistoryManager("", -1)

		require.NotNil(t, hm, "should create a history manager with negative maxEntries")

		// Add many unique entries to verify no limit is enforced (treated as unlimited)
		for i := range 500 {
			entry := "entry_" + string(rune('A'+i/26%26)) + string(rune('a'+i%26))
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		history := hm.History()
		assert.Len(t, history, 500, "negative maxEntries should be treated as unlimited")
	})
}

// =============================================================================
// Test: HistoryManager.Add
// =============================================================================

func TestHistoryManager_Add(t *testing.T) {
	t.Run("adding a normal entry increases history length", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		initialLen := len(hm.History())
		assert.Equal(t, 0, initialLen, "initial history should be empty")

		err := hm.Add("first command")
		require.NoError(t, err, "adding normal entry should succeed")

		history := hm.History()
		require.Len(t, history, 1, "history should have one entry after add")
		assert.Equal(t, "first command", history[0], "entry should match what was added")
	})

	t.Run("adding multiple entries increases history length", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		entries := []string{"command1", "command2", "command3"}
		for _, entry := range entries {
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		history := hm.History()
		assert.Len(t, history, 3, "history should have three entries")
		assert.Equal(t, entries, history, "entries should match in order")
	})

	t.Run("empty string is rejected and not added", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		// Add a valid entry first
		err := hm.Add("valid entry")
		require.NoError(t, err)

		// Try to add empty string
		err = hm.Add("")
		require.Error(t, err, "adding empty string should return error")
		assert.Contains(t, err.Error(), "empty", "error should mention empty input")

		history := hm.History()
		assert.Len(t, history, 1, "empty string should not be added to history")
	})

	t.Run("whitespace-only string is rejected and not added", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		whitespaceInputs := []string{" ", "  ", "\t", "\n", " \t\n ", "   \t   "}

		for _, ws := range whitespaceInputs {
			err := hm.Add(ws)
			require.Error(t, err, "adding whitespace-only string should return error: %q", ws)
		}

		history := hm.History()
		assert.Empty(t, history, "whitespace-only strings should not be added to history")
	})

	t.Run("consecutive duplicate is rejected and not added", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		// Add first entry
		err := hm.Add("same command")
		require.NoError(t, err)

		// Try to add the same entry immediately after
		err = hm.Add("same command")
		require.Error(t, err, "adding consecutive duplicate should return error")
		assert.Contains(t, err.Error(), "duplicate", "error should mention duplicate")

		history := hm.History()
		assert.Len(t, history, 1, "consecutive duplicate should not be added")
	})

	t.Run("non-consecutive duplicate is allowed", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		// Add entries with a repeat
		err := hm.Add("command A")
		require.NoError(t, err)

		err = hm.Add("command B")
		require.NoError(t, err)

		// Add command A again (non-consecutive)
		err = hm.Add("command A")
		require.NoError(t, err, "non-consecutive duplicate should be allowed")

		history := hm.History()
		assert.Len(t, history, 3, "non-consecutive duplicate should be added")
		assert.Equal(t, []string{"command A", "command B", "command A"}, history)
	})

	t.Run("entry with leading and trailing whitespace is trimmed before comparison", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("  command  ")
		require.NoError(t, err)

		history := hm.History()
		require.Len(t, history, 1)
		assert.Equal(t, "command", history[0], "entry should be trimmed")

		// Adding the same command without whitespace should be rejected as duplicate
		err = hm.Add("command")
		require.Error(t, err, "trimmed duplicate should be rejected")

		history = hm.History()
		assert.Len(t, history, 1, "trimmed duplicate should not be added")
	})

	t.Run("entry with internal whitespace is preserved", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("command with   internal   spaces")
		require.NoError(t, err)

		history := hm.History()
		require.Len(t, history, 1)
		assert.Equal(t, "command with   internal   spaces", history[0],
			"internal whitespace should be preserved")
	})
}

// =============================================================================
// Test: HistoryManager.Add - Table-Driven
// =============================================================================

func TestHistoryManager_Add_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		entries       []string
		wantLen       int
		wantErr       bool
		lastEntryWant string
	}{
		{
			name:          "single valid entry",
			entries:       []string{"ls -la"},
			wantLen:       1,
			wantErr:       false,
			lastEntryWant: "ls -la",
		},
		{
			name:          "multiple unique entries",
			entries:       []string{"cd /tmp", "ls", "pwd"},
			wantLen:       3,
			wantErr:       false,
			lastEntryWant: "pwd",
		},
		{
			name:          "entry with special characters",
			entries:       []string{"echo 'hello world' | grep -E \"[a-z]+\""},
			wantLen:       1,
			wantErr:       false,
			lastEntryWant: "echo 'hello world' | grep -E \"[a-z]+\"",
		},
		{
			name:          "entry with unicode",
			entries:       []string{"echo Hello World"},
			wantLen:       1,
			wantErr:       false,
			lastEntryWant: "echo Hello World",
		},
		{
			name:          "very long entry",
			entries:       []string{"cat " + string(make([]byte, 1000))},
			wantLen:       1,
			wantErr:       false,
			lastEntryWant: "cat " + string(make([]byte, 1000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := ui.NewHistoryManager("", 100)

			var lastErr error
			for _, entry := range tt.entries {
				lastErr = hm.Add(entry)
			}

			history := hm.History()

			if tt.wantErr {
				require.Error(t, lastErr)
			} else {
				require.NoError(t, lastErr)
			}

			require.Len(t, history, tt.wantLen)

			if tt.wantLen > 0 {
				assert.Equal(t, tt.lastEntryWant, history[len(history)-1])
			}
		})
	}
}

// =============================================================================
// Test: HistoryManager.History
// =============================================================================

func TestHistoryManager_History(t *testing.T) {
	t.Run("returns empty slice when no entries", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		history := hm.History()

		require.NotNil(t, history, "History() should never return nil")
		assert.Empty(t, history, "should return empty slice when no entries")
	})

	t.Run("returns copy of history not internal state", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("entry1")
		require.NoError(t, err)
		err = hm.Add("entry2")
		require.NoError(t, err)

		// Get history and modify the returned slice
		history1 := hm.History()
		require.Len(t, history1, 2)

		// Modify the returned slice
		history1[0] = "modified"
		_ = append(history1, "extra")

		// Get history again and verify it wasn't affected
		history2 := hm.History()
		assert.Len(t, history2, 2, "internal state should not be affected by modifying returned slice")
		assert.Equal(t, "entry1", history2[0], "first entry should be unchanged")
		assert.Equal(t, "entry2", history2[1], "second entry should be unchanged")
	})

	t.Run("returns entries in order added", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		entries := []string{"first", "second", "third", "fourth", "fifth"}
		for _, entry := range entries {
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		history := hm.History()
		assert.Equal(t, entries, history, "entries should be in order added (oldest first)")
	})

	t.Run("returns entries after some were trimmed by max limit", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 3)

		entries := []string{"one", "two", "three", "four", "five"}
		for _, entry := range entries {
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		history := hm.History()
		assert.Len(t, history, 3, "should be limited to maxEntries")
		// Oldest entries should be trimmed, keeping most recent
		assert.Equal(t, []string{"three", "four", "five"}, history,
			"should keep most recent entries after trimming")
	})

	t.Run("multiple calls return consistent results", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("consistent")
		require.NoError(t, err)

		history1 := hm.History()
		history2 := hm.History()
		history3 := hm.History()

		assert.Equal(t, history1, history2, "multiple calls should return same content")
		assert.Equal(t, history2, history3, "multiple calls should return same content")
	})
}

// =============================================================================
// Test: ExpandPath
// =============================================================================

func TestExpandPath(t *testing.T) {
	t.Run("tilde alone is expanded to home directory", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err, "should be able to get home directory")

		result := ui.ExpandPath("~")

		assert.Equal(t, homeDir, result, "~ should expand to home directory")
	})

	t.Run("tilde with path is expanded to home plus path", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err, "should be able to get home directory")

		result := ui.ExpandPath("~/foo")

		expected := filepath.Join(homeDir, "foo")
		assert.Equal(t, expected, result, "~/foo should expand to home/foo")
	})

	t.Run("tilde with nested path is expanded correctly", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		result := ui.ExpandPath("~/foo/bar/baz")

		expected := filepath.Join(homeDir, "foo", "bar", "baz")
		assert.Equal(t, expected, result, "~/foo/bar/baz should expand correctly")
	})

	t.Run("regular path without tilde is unchanged", func(t *testing.T) {
		paths := []string{
			"/usr/local/bin",
			"/home/user/file.txt",
			"./relative/path",
			"relative/path",
			"/",
			".",
		}

		for _, path := range paths {
			result := ui.ExpandPath(path)
			assert.Equal(t, path, result, "path without ~ should be unchanged: %s", path)
		}
	})

	t.Run("empty string returns empty string", func(t *testing.T) {
		result := ui.ExpandPath("")

		assert.Empty(t, result, "empty string should return empty string")
	})

	t.Run("tilde in middle of path is not expanded", func(t *testing.T) {
		input := "/path/to/~file"

		result := ui.ExpandPath(input)

		assert.Equal(t, input, result, "tilde in middle of path should not be expanded")
	})

	t.Run("tilde followed by username is not expanded", func(t *testing.T) {
		// This tests that ~username format is NOT expanded
		// (only ~ and ~/ are expanded)
		input := "~otheruser/files"

		result := ui.ExpandPath(input)

		// Implementation may choose to expand or not expand ~user format
		// For simplicity, we expect it NOT to expand (only ~ and ~/ are handled)
		assert.Equal(t, input, result, "~username format should not be expanded")
	})
}

// =============================================================================
// Test: ExpandPath - Table-Driven
// =============================================================================

func TestExpandPath_TableDriven(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde alone",
			input:    "~",
			expected: homeDir,
		},
		{
			name:     "tilde with simple path",
			input:    "~/Documents",
			expected: filepath.Join(homeDir, "Documents"),
		},
		{
			name:     "tilde with deep path",
			input:    "~/a/b/c/d/e",
			expected: filepath.Join(homeDir, "a", "b", "c", "d", "e"),
		},
		{
			name:     "absolute path",
			input:    "/etc/hosts",
			expected: "/etc/hosts",
		},
		{
			name:     "relative path",
			input:    "relative/path",
			expected: "relative/path",
		},
		{
			name:     "current directory",
			input:    ".",
			expected: ".",
		},
		{
			name:     "parent directory",
			input:    "..",
			expected: "..",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "path with tilde in filename",
			input:    "/path/to/file~backup",
			expected: "/path/to/file~backup",
		},
		{
			name:     "tilde with file extension",
			input:    "~/.bashrc",
			expected: filepath.Join(homeDir, ".bashrc"),
		},
		{
			name:     "tilde with hidden directory",
			input:    "~/.config/app",
			expected: filepath.Join(homeDir, ".config", "app"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.ExpandPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Test: HistoryManager MaxEntries Behavior
// =============================================================================

func TestHistoryManager_MaxEntries(t *testing.T) {
	t.Run("when maxEntries greater than zero old entries are trimmed when limit exceeded", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 5)

		// Add exactly maxEntries
		for i := 1; i <= 5; i++ {
			err := hm.Add("entry" + string(rune('0'+i)))
			require.NoError(t, err)
		}

		history := hm.History()
		assert.Len(t, history, 5, "should have exactly 5 entries at limit")

		// Add one more to exceed limit
		err := hm.Add("entry6")
		require.NoError(t, err)

		history = hm.History()
		require.Len(t, history, 5, "should still have 5 entries after exceeding limit")
		assert.Equal(t, "entry2", history[0], "oldest entry should be removed")
		assert.Equal(t, "entry6", history[4], "newest entry should be present")
	})

	t.Run("when maxEntries is zero no trimming occurs (unlimited)", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 0)

		// Add many entries
		numEntries := 10000
		for i := range numEntries {
			err := hm.Add("entry" + string(rune(i%256)))
			// Note: some may be duplicates, but we're testing unlimited storage
			_ = err
		}

		history := hm.History()
		assert.Greater(t, len(history), 5000,
			"unlimited mode should allow storing many entries")
	})

	t.Run("trimming removes oldest entries first", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 3)

		entries := []string{"oldest", "middle", "newest"}
		for _, entry := range entries {
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		// Add one more
		err := hm.Add("newest2")
		require.NoError(t, err)

		history := hm.History()
		assert.Equal(t, []string{"middle", "newest", "newest2"}, history,
			"oldest entry should be removed first")
	})

	t.Run("adding many entries at once trims correctly", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 3)

		// Add 10 entries
		for i := 1; i <= 10; i++ {
			err := hm.Add("cmd" + string(rune('0'+i%10)))
			require.NoError(t, err)
		}

		history := hm.History()
		assert.Len(t, history, 3, "should maintain maxEntries limit")
		// Last 3 unique entries should be present (7, 8, 9 -> but accounting for mod 10: 8, 9, 0)
	})

	t.Run("maxEntries of 1 keeps only the most recent entry", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 1)

		err := hm.Add("first")
		require.NoError(t, err)

		err = hm.Add("second")
		require.NoError(t, err)

		err = hm.Add("third")
		require.NoError(t, err)

		history := hm.History()
		require.Len(t, history, 1, "should only keep 1 entry")
		assert.Equal(t, "third", history[0], "should keep the most recent entry")
	})
}

// =============================================================================
// Test: HistoryManager MaxEntries - Table-Driven
// =============================================================================

func TestHistoryManager_MaxEntries_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		maxEntries   int
		entriesToAdd int
		expectedLen  int
	}{
		{
			name:         "maxEntries 10 with 5 entries",
			maxEntries:   10,
			entriesToAdd: 5,
			expectedLen:  5,
		},
		{
			name:         "maxEntries 10 with 10 entries",
			maxEntries:   10,
			entriesToAdd: 10,
			expectedLen:  10,
		},
		{
			name:         "maxEntries 10 with 15 entries",
			maxEntries:   10,
			entriesToAdd: 15,
			expectedLen:  10,
		},
		{
			name:         "maxEntries 5 with 100 entries",
			maxEntries:   5,
			entriesToAdd: 100,
			expectedLen:  5,
		},
		{
			name:         "maxEntries 0 (unlimited) with 500 entries",
			maxEntries:   0,
			entriesToAdd: 500,
			expectedLen:  500,
		},
		{
			name:         "maxEntries 1 with 50 entries",
			maxEntries:   1,
			entriesToAdd: 50,
			expectedLen:  1,
		},
		{
			name:         "maxEntries 100 with 1 entry",
			maxEntries:   100,
			entriesToAdd: 1,
			expectedLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := ui.NewHistoryManager("", tt.maxEntries)

			for i := range tt.entriesToAdd {
				// Use unique entries to avoid duplicate rejection
				entry := "unique_entry_" + string(
					rune(i/1000+'0'),
				) + string(
					rune((i/100)%10+'0'),
				) + string(
					rune((i/10)%10+'0'),
				) + string(
					rune(i%10+'0'),
				)
				err := hm.Add(entry)
				require.NoError(t, err, "adding entry %d should succeed", i)
			}

			history := hm.History()
			assert.Len(t, history, tt.expectedLen, "history length should match expected")
		})
	}
}

// =============================================================================
// Test: HistoryManager Edge Cases
// =============================================================================

func TestHistoryManager_EdgeCases(t *testing.T) {
	t.Run("handles concurrent add operations safely", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 1000)

		done := make(chan bool, 100)

		// Spawn multiple goroutines adding entries
		for i := range 100 {
			go func(id int) {
				entry := "concurrent_entry_" + string(rune('A'+id%26))
				_ = hm.Add(entry)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for range 100 {
			<-done
		}

		history := hm.History()
		assert.NotEmpty(t, history, "should have some entries after concurrent adds")
		assert.LessOrEqual(t, len(history), 1000, "should not exceed maxEntries")
	})

	t.Run("handles concurrent history reads safely", func(_ *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		// Add some entries first
		for i := range 50 {
			_ = hm.Add("entry" + string(rune('0'+i%10)))
		}

		done := make(chan bool, 100)

		// Spawn multiple goroutines reading history
		for range 100 {
			go func() {
				history := hm.History()
				_ = len(history) // Use the result
				done <- true
			}()
		}

		// Wait for all goroutines
		for range 100 {
			<-done
		}

		// Should complete without race condition or panic
	})

	t.Run("entry with only newlines is rejected", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("\n\n\n")
		require.Error(t, err, "entry with only newlines should be rejected")

		history := hm.History()
		assert.Empty(t, history)
	})

	t.Run("entry with mixed whitespace types is rejected if all whitespace", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add(" \t\n \r\n ")
		require.Error(t, err, "entry with only mixed whitespace should be rejected")

		history := hm.History()
		assert.Empty(t, history)
	})

	t.Run("duplicate detection is case-sensitive", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("Command")
		require.NoError(t, err)

		err = hm.Add("command")
		require.NoError(t, err, "different case should not be considered duplicate")

		err = hm.Add("COMMAND")
		require.NoError(t, err, "different case should not be considered duplicate")

		history := hm.History()
		assert.Len(t, history, 3, "all three case variants should be stored")
	})
}

// =============================================================================
// Test: HistoryManager Size Method (convenience)
// =============================================================================

func TestHistoryManager_Size(t *testing.T) {
	t.Run("returns zero for empty history", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		size := hm.Size()
		assert.Equal(t, 0, size, "size should be 0 for empty history")
	})

	t.Run("returns correct count after adding entries", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		for i := range 5 {
			err := hm.Add("entry" + string(rune('0'+i)))
			require.NoError(t, err)
		}

		size := hm.Size()
		assert.Equal(t, 5, size, "size should match number of entries added")
	})

	t.Run("returns capped size when at maxEntries limit", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 3)

		for i := range 10 {
			err := hm.Add("entry" + string(rune('0'+i)))
			require.NoError(t, err)
		}

		size := hm.Size()
		assert.Equal(t, 3, size, "size should be capped at maxEntries")
	})
}

// =============================================================================
// Test: HistoryManager Clear Method
// =============================================================================

func TestHistoryManager_Clear(t *testing.T) {
	t.Run("clears all entries from history", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		for i := range 10 {
			err := hm.Add("entry" + string(rune('0'+i)))
			require.NoError(t, err)
		}

		assert.Equal(t, 10, hm.Size(), "should have 10 entries before clear")

		hm.Clear()

		assert.Equal(t, 0, hm.Size(), "should have 0 entries after clear")
		history := hm.History()
		assert.Empty(t, history, "history should be empty after clear")
	})

	t.Run("allows adding entries after clear", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("before clear")
		require.NoError(t, err)

		hm.Clear()

		err = hm.Add("after clear")
		require.NoError(t, err)

		history := hm.History()
		require.Len(t, history, 1)
		assert.Equal(t, "after clear", history[0])
	})

	t.Run("clear on empty history is safe", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		// Should not panic
		hm.Clear()

		assert.Equal(t, 0, hm.Size())
	})
}

// =============================================================================
// Test: HistoryManager Last Method
// =============================================================================

func TestHistoryManager_Last(t *testing.T) {
	t.Run("returns empty string and false when history is empty", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		last, ok := hm.Last()

		assert.False(t, ok, "ok should be false for empty history")
		assert.Empty(t, last, "last should be empty string for empty history")
	})

	t.Run("returns the most recent entry", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("first")
		require.NoError(t, err)
		err = hm.Add("second")
		require.NoError(t, err)
		err = hm.Add("third")
		require.NoError(t, err)

		last, ok := hm.Last()

		assert.True(t, ok, "ok should be true when history has entries")
		assert.Equal(t, "third", last, "should return the most recent entry")
	})

	t.Run("returns correct entry after trimming", func(t *testing.T) {
		hm := ui.NewHistoryManager("", 2)

		err := hm.Add("will be removed")
		require.NoError(t, err)
		err = hm.Add("second")
		require.NoError(t, err)
		err = hm.Add("third")
		require.NoError(t, err)

		last, ok := hm.Last()

		assert.True(t, ok)
		assert.Equal(t, "third", last, "should return most recent after trim")
	})
}

// =============================================================================
// TDD Red Phase Tests for HistoryManager - Cycle 2: File Persistence
// These tests define the expected behavior for file I/O operations.
// All tests should FAIL initially until the file persistence is implemented.
// =============================================================================

// =============================================================================
// Test: HistoryManager_LoadFromFile
// =============================================================================

func TestHistoryManager_LoadFromFile(t *testing.T) {
	t.Run("loads entries from existing file with multiple lines", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// Pre-create a history file with entries
		content := "first command\nsecond command\nthird command\n"
		err := os.WriteFile(historyFile, []byte(content), 0o600)
		require.NoError(t, err, "setup: should create history file")

		// Create HistoryManager - constructor should load from file
		hm := ui.NewHistoryManager(historyFile, 100)

		history := hm.History()
		require.Len(t, history, 3, "should load 3 entries from file")
		assert.Equal(t, "first command", history[0])
		assert.Equal(t, "second command", history[1])
		assert.Equal(t, "third command", history[2])
	})

	t.Run("loads entries from file without trailing newline", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// File without trailing newline
		content := "command one\ncommand two"
		err := os.WriteFile(historyFile, []byte(content), 0o600)
		require.NoError(t, err)

		hm := ui.NewHistoryManager(historyFile, 100)

		history := hm.History()
		require.Len(t, history, 2, "should load entries from file without trailing newline")
		assert.Equal(t, "command one", history[0])
		assert.Equal(t, "command two", history[1])
	})

	t.Run("loads from empty file results in empty history", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// Create empty file
		err := os.WriteFile(historyFile, []byte(""), 0o600)
		require.NoError(t, err)

		hm := ui.NewHistoryManager(historyFile, 100)

		history := hm.History()
		assert.Empty(t, history, "empty file should result in empty history")
	})

	t.Run("non-existent file does not error and starts with empty history", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "nonexistent_history")

		// File does not exist - should not error
		hm := ui.NewHistoryManager(historyFile, 100)

		require.NotNil(t, hm, "should create manager even when file does not exist")
		history := hm.History()
		assert.Empty(t, history, "should start with empty history when file does not exist")
	})

	t.Run("skips empty lines when loading", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// File with empty lines
		content := "command one\n\ncommand two\n\n\ncommand three\n"
		err := os.WriteFile(historyFile, []byte(content), 0o600)
		require.NoError(t, err)

		hm := ui.NewHistoryManager(historyFile, 100)

		history := hm.History()
		require.Len(t, history, 3, "should skip empty lines")
		assert.Equal(t, []string{"command one", "command two", "command three"}, history)
	})

	t.Run("trims whitespace from loaded entries", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// File with entries that have leading/trailing whitespace
		content := "  command one  \n\tcommand two\t\n command three \n"
		err := os.WriteFile(historyFile, []byte(content), 0o600)
		require.NoError(t, err)

		hm := ui.NewHistoryManager(historyFile, 100)

		history := hm.History()
		require.Len(t, history, 3, "should load 3 trimmed entries")
		assert.Equal(t, "command one", history[0], "entry should be trimmed")
		assert.Equal(t, "command two", history[1], "entry should be trimmed")
		assert.Equal(t, "command three", history[2], "entry should be trimmed")
	})

	t.Run("respects maxEntries when loading from file", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// File with more entries than maxEntries
		content := "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
		err := os.WriteFile(historyFile, []byte(content), 0o600)
		require.NoError(t, err)

		hm := ui.NewHistoryManager(historyFile, 5)

		history := hm.History()
		require.Len(t, history, 5, "should respect maxEntries limit when loading")
		// Should keep the most recent entries
		assert.Equal(t, []string{"six", "seven", "eight", "nine", "ten"}, history,
			"should keep most recent entries when loading exceeds maxEntries")
	})

	t.Run("handles file with only whitespace lines gracefully", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// File with only whitespace lines
		content := "   \n\t\t\n  \t  \n"
		err := os.WriteFile(historyFile, []byte(content), 0o600)
		require.NoError(t, err)

		hm := ui.NewHistoryManager(historyFile, 100)

		history := hm.History()
		assert.Empty(t, history, "file with only whitespace should result in empty history")
	})

	t.Run("handles file with carriage return line endings", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// Windows-style CRLF line endings
		content := "command one\r\ncommand two\r\ncommand three\r\n"
		err := os.WriteFile(historyFile, []byte(content), 0o600)
		require.NoError(t, err)

		hm := ui.NewHistoryManager(historyFile, 100)

		history := hm.History()
		require.Len(t, history, 3, "should handle CRLF line endings")
		assert.Equal(t, "command one", history[0])
		assert.Equal(t, "command two", history[1])
		assert.Equal(t, "command three", history[2])
	})
}

// =============================================================================
// Test: HistoryManager_SaveToFile
// =============================================================================

func TestHistoryManager_SaveToFile(t *testing.T) {
	t.Run("save creates file if it does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "new_history")

		hm := ui.NewHistoryManager(historyFile, 100)

		err := hm.Add("first command")
		require.NoError(t, err)

		// Verify file was created
		_, err = os.Stat(historyFile)
		require.NoError(t, err, "file should be created after Add")

		content, err := os.ReadFile(historyFile)
		require.NoError(t, err)
		assert.Equal(t, "first command\n", string(content), "file should contain the entry")
	})

	t.Run("save writes all entries one per line", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		entries := []string{"first", "second", "third", "fourth", "fifth"}
		for _, entry := range entries {
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		content, err := os.ReadFile(historyFile)
		require.NoError(t, err)

		expected := "first\nsecond\nthird\nfourth\nfifth\n"
		assert.Equal(t, expected, string(content), "file should contain all entries one per line")
	})

	t.Run("save overwrites existing file content", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// Create initial history
		hm1 := ui.NewHistoryManager(historyFile, 100)
		err := hm1.Add("old entry one")
		require.NoError(t, err)
		err = hm1.Add("old entry two")
		require.NoError(t, err)

		// Create new history manager and clear/add new entries
		hm2 := ui.NewHistoryManager(historyFile, 100)
		hm2.Clear()
		err = hm2.Add("new entry")
		require.NoError(t, err)

		// Force a save (if Clear triggers save, or Add triggers save)
		// The file should reflect current state

		content, err := os.ReadFile(historyFile)
		require.NoError(t, err)

		assert.Equal(t, "new entry\n", string(content),
			"file should be overwritten with new content after Clear and Add")
	})

	t.Run("save with empty history creates empty file or deletes file", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		// Create with some entries first
		hm := ui.NewHistoryManager(historyFile, 100)
		err := hm.Add("temporary entry")
		require.NoError(t, err)

		// Clear the history
		hm.Clear()

		// File should either be empty or not exist
		content, err := os.ReadFile(historyFile)
		if err == nil {
			assert.Empty(t, string(content), "cleared history should result in empty file")
		}
		// Alternative: file could be deleted, which is also acceptable
	})

	t.Run("save creates parent directories if they do not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "subdir", "nested", "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		err := hm.Add("entry in nested dir")
		require.NoError(t, err)

		// Verify file was created
		content, err := os.ReadFile(historyFile)
		require.NoError(t, err, "file should be created including parent directories")
		assert.Equal(t, "entry in nested dir\n", string(content))
	})
}

// =============================================================================
// Test: HistoryManager_AppendToFile
// =============================================================================

func TestHistoryManager_AppendToFile(t *testing.T) {
	t.Run("Add appends entry to file immediately", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		err := hm.Add("first entry")
		require.NoError(t, err)

		// Read file immediately after Add
		content, err := os.ReadFile(historyFile)
		require.NoError(t, err)
		assert.Equal(t, "first entry\n", string(content),
			"entry should be written to file immediately after Add")
	})

	t.Run("multiple Add calls append multiple lines", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		err := hm.Add("line one")
		require.NoError(t, err)

		content1, _ := os.ReadFile(historyFile)
		assert.Equal(t, "line one\n", string(content1))

		err = hm.Add("line two")
		require.NoError(t, err)

		content2, _ := os.ReadFile(historyFile)
		assert.Equal(t, "line one\nline two\n", string(content2))

		err = hm.Add("line three")
		require.NoError(t, err)

		content3, _ := os.ReadFile(historyFile)
		assert.Equal(t, "line one\nline two\nline three\n", string(content3),
			"each Add should append to file")
	})

	t.Run("file content matches in-memory history after multiple operations", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		entries := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
		for _, entry := range entries {
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		// Read file content
		content, err := os.ReadFile(historyFile)
		require.NoError(t, err)

		// Parse file content
		lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")

		// Compare with in-memory history
		history := hm.History()
		assert.Equal(t, history, lines, "file content should match in-memory history")
	})

	t.Run("rejected entries are not written to file", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		err := hm.Add("valid entry")
		require.NoError(t, err)

		// Try to add invalid entries
		_ = hm.Add("")            // Empty - should be rejected
		_ = hm.Add("   ")         // Whitespace only - should be rejected
		_ = hm.Add("valid entry") // Duplicate - should be rejected

		content, err := os.ReadFile(historyFile)
		require.NoError(t, err)

		assert.Equal(t, "valid entry\n", string(content),
			"rejected entries should not be written to file")
	})
}

// =============================================================================
// Test: HistoryManager_FilePermissions
// =============================================================================

func TestHistoryManager_FilePermissions(t *testing.T) {
	t.Run("created file has permission 0600", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		err := hm.Add("test entry")
		require.NoError(t, err)

		info, err := os.Stat(historyFile)
		require.NoError(t, err, "file should exist")

		// Check file permissions (0600 = user read/write only)
		perm := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0o600), perm,
			"history file should have 0600 permissions for security")
	})

	t.Run("file permissions are preserved after multiple writes", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		// Add multiple entries
		for i := range 10 {
			err := hm.Add("entry" + string(rune('0'+i)))
			require.NoError(t, err)
		}

		info, err := os.Stat(historyFile)
		require.NoError(t, err)

		perm := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0o600), perm,
			"file should maintain 0600 permissions after multiple writes")
	})
}

// =============================================================================
// Test: HistoryManager_TrimRewritesFile
// =============================================================================

func TestHistoryManager_TrimRewritesFile(t *testing.T) {
	t.Run("file is rewritten when trimming occurs", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 3)

		// Add entries up to limit
		err := hm.Add("first")
		require.NoError(t, err)
		err = hm.Add("second")
		require.NoError(t, err)
		err = hm.Add("third")
		require.NoError(t, err)

		content, _ := os.ReadFile(historyFile)
		assert.Equal(t, "first\nsecond\nthird\n", string(content), "file should have 3 entries")

		// Add one more to trigger trim
		err = hm.Add("fourth")
		require.NoError(t, err)

		content, err = os.ReadFile(historyFile)
		require.NoError(t, err)

		// File should be rewritten to only contain the trimmed entries
		assert.Equal(t, "second\nthird\nfourth\n", string(content),
			"file should be rewritten after trim to match in-memory state")
	})

	t.Run("file content matches in-memory after trim", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 5)

		// Add more entries than maxEntries
		for i := 1; i <= 10; i++ {
			err := hm.Add("entry" + string(rune('0'+i%10)))
			require.NoError(t, err)
		}

		// Read file content
		content, err := os.ReadFile(historyFile)
		require.NoError(t, err)

		// Parse file content
		fileLines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")

		// Compare with in-memory history
		history := hm.History()
		assert.Equal(t, history, fileLines,
			"file content should match in-memory history after trimming")
	})

	t.Run("multiple trims keep file in sync with memory", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 2)

		// Add entries that will cause multiple trims
		entries := []string{"a", "b", "c", "d", "e", "f", "g"}
		for _, entry := range entries {
			err := hm.Add(entry)
			require.NoError(t, err)

			// Verify file matches memory after each add
			content, err := os.ReadFile(historyFile)
			require.NoError(t, err)

			fileLines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
			history := hm.History()

			assert.Equal(t, history, fileLines,
				"file should match memory after adding %q", entry)
		}

		// Final check
		content, _ := os.ReadFile(historyFile)
		assert.Equal(t, "f\ng\n", string(content),
			"file should contain only the last 2 entries")
	})
}

// =============================================================================
// Test: HistoryManager_FilePath
// =============================================================================

func TestHistoryManager_FilePath(t *testing.T) {
	t.Run("empty filePath means no file operations occur", func(t *testing.T) {
		// Create with empty path (in-memory only)
		hm := ui.NewHistoryManager("", 100)

		err := hm.Add("in memory only")
		require.NoError(t, err)

		history := hm.History()
		require.Len(t, history, 1)
		assert.Equal(t, "in memory only", history[0])

		// No file should be created anywhere - this is the expected behavior
		// for in-memory mode. The implementation should not write to any file
		// when filePath is empty.
	})

	t.Run("tilde expansion works for file paths", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		// Use a path that would be in temp but starts with ~
		// We need to simulate this carefully
		tempDir := t.TempDir()

		// Create a relative path from home to temp for testing
		// Instead, we'll test that tilde expansion is applied to the filePath
		// by checking if the file is created in the correct location

		// For testing purposes, we'll verify the ExpandPath function works
		// with file paths by creating a file in a known location
		historyFile := filepath.Join(tempDir, "tilde_test_history")

		// Create the test file to ensure directory exists
		hm := ui.NewHistoryManager(historyFile, 100)
		err = hm.Add("test")
		require.NoError(t, err)

		// Verify file was created in the expected location
		_, err = os.Stat(historyFile)
		require.NoError(t, err, "file should be created at the specified path")

		// Now test that ExpandPath works correctly (separate from HistoryManager)
		expandedPath := ui.ExpandPath("~/.test_history")
		expectedPath := filepath.Join(homeDir, ".test_history")
		assert.Equal(t, expectedPath, expandedPath,
			"ExpandPath should expand ~ to home directory")
	})

	t.Run("absolute path is used without modification", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "absolute_path_history")

		hm := ui.NewHistoryManager(historyFile, 100)

		err := hm.Add("test entry")
		require.NoError(t, err)

		// Verify file was created at the exact path
		_, err = os.Stat(historyFile)
		require.NoError(t, err, "file should be created at absolute path")

		content, err := os.ReadFile(historyFile)
		require.NoError(t, err)
		assert.Equal(t, "test entry\n", string(content))
	})
}

// =============================================================================
// Test: HistoryManager_FilePersistence_Integration
// =============================================================================

func TestHistoryManager_FilePersistence_Integration(t *testing.T) {
	t.Run("history persists across manager instances", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "persistent_history")

		// First instance - add entries
		hm1 := ui.NewHistoryManager(historyFile, 100)
		err := hm1.Add("persistent one")
		require.NoError(t, err)
		err = hm1.Add("persistent two")
		require.NoError(t, err)
		err = hm1.Add("persistent three")
		require.NoError(t, err)

		// Second instance - should load previous entries
		hm2 := ui.NewHistoryManager(historyFile, 100)

		history := hm2.History()
		require.Len(t, history, 3, "should load entries from previous instance")
		assert.Equal(t, []string{"persistent one", "persistent two", "persistent three"}, history)
	})

	t.Run("new entries are appended after loading existing file", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "append_history")

		// First instance
		hm1 := ui.NewHistoryManager(historyFile, 100)
		err := hm1.Add("original")
		require.NoError(t, err)

		// Second instance - load and add more
		hm2 := ui.NewHistoryManager(historyFile, 100)
		err = hm2.Add("appended")
		require.NoError(t, err)

		// Third instance - verify all entries
		hm3 := ui.NewHistoryManager(historyFile, 100)
		history := hm3.History()

		require.Len(t, history, 2)
		assert.Equal(t, []string{"original", "appended"}, history)
	})

	t.Run("cleared history persists across instances", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "clear_persist_history")

		// First instance - add then clear
		hm1 := ui.NewHistoryManager(historyFile, 100)
		err := hm1.Add("to be cleared")
		require.NoError(t, err)
		hm1.Clear()

		// Second instance - should start empty
		hm2 := ui.NewHistoryManager(historyFile, 100)
		history := hm2.History()

		assert.Empty(t, history, "cleared history should persist as empty")
	})

	t.Run("maxEntries limit is applied when loading large file", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "large_history")

		// First instance with high limit - add many entries
		hm1 := ui.NewHistoryManager(historyFile, 1000)
		for i := range 100 {
			entry := "entry_" + string(rune('0'+i/100)) + string(rune('0'+(i/10)%10)) + string(rune('0'+i%10))
			err := hm1.Add(entry)
			require.NoError(t, err)
		}

		// Second instance with lower limit - should trim on load
		hm2 := ui.NewHistoryManager(historyFile, 10)

		history := hm2.History()
		require.Len(t, history, 10, "should respect new maxEntries when loading")

		// Should have the most recent entries
		assert.Equal(t, "entry_090", history[0])
		assert.Equal(t, "entry_099", history[9])
	})
}

// =============================================================================
// Test: HistoryManager_FileErrors
// =============================================================================

func TestHistoryManager_FileErrors(t *testing.T) {
	t.Run("handles read-only directory gracefully", func(t *testing.T) {
		// Skip on systems where we cannot test permissions effectively
		if os.Getuid() == 0 {
			t.Skip("skipping permission test when running as root")
		}

		tempDir := t.TempDir()
		readOnlyDir := filepath.Join(tempDir, "readonly")
		err := os.Mkdir(readOnlyDir, 0o500) // Read and execute only
		require.NoError(t, err)

		historyFile := filepath.Join(readOnlyDir, "history")

		// Creating manager should not panic
		hm := ui.NewHistoryManager(historyFile, 100)
		require.NotNil(t, hm, "should create manager even with read-only directory")

		// Add should either return error or handle gracefully
		_ = hm.Add("test entry")
		// We expect this to fail since directory is read-only
		// The implementation should handle this gracefully

		// Restore permissions for cleanup
		_ = os.Chmod(readOnlyDir, 0o700)
	})

	t.Run("handles directory as file path gracefully", func(t *testing.T) {
		tempDir := t.TempDir()
		dirAsFile := filepath.Join(tempDir, "is_a_directory")
		err := os.Mkdir(dirAsFile, 0o700)
		require.NoError(t, err)

		// Try to use directory as history file
		hm := ui.NewHistoryManager(dirAsFile, 100)
		require.NotNil(t, hm)

		// Add should handle this gracefully (return error or in-memory fallback)
		_ = hm.Add("test entry")
		// Implementation decides behavior - should not panic

		// In-memory should still work
		history := hm.History()
		// Either empty (if Add failed) or contains entry (if in-memory fallback)
		_ = history
	})
}

// =============================================================================
// Test: HistoryManager_SpecialCharacters
// =============================================================================

func TestHistoryManager_SpecialCharacters(t *testing.T) {
	t.Run("entries with newlines are handled correctly", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		// Entry containing newline - should either be rejected or escaped
		entry := "line1\nline2"
		err := hm.Add(entry)

		// Two possible valid behaviors:
		// 1. Reject entries with embedded newlines (return error)
		// 2. Escape/encode newlines and store correctly
		// We check that either behavior is consistent with file persistence

		if err != nil {
			// If rejected, history should be empty
			history := hm.History()
			assert.Empty(t, history, "rejected entry should not be in history")
		} else {
			// If accepted, file should correctly represent the entry
			// and loading should restore it correctly
			hm2 := ui.NewHistoryManager(historyFile, 100)
			history := hm2.History()
			require.Len(t, history, 1)
			assert.Equal(t, entry, history[0],
				"entry with newline should be preserved correctly after reload")
		}
	})

	t.Run("entries with unicode characters persist correctly", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		unicodeEntries := []string{
			"echo Hello World",
			"git commit -m 'fix: update feature'",
			"ls path/to/file",
			"cat README.md",
		}

		for _, entry := range unicodeEntries {
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		// Reload and verify
		hm2 := ui.NewHistoryManager(historyFile, 100)
		history := hm2.History()

		assert.Equal(t, unicodeEntries, history,
			"unicode entries should persist correctly")
	})

	t.Run("entries with special shell characters persist correctly", func(t *testing.T) {
		tempDir := t.TempDir()
		historyFile := filepath.Join(tempDir, "history")

		hm := ui.NewHistoryManager(historyFile, 100)

		specialEntries := []string{
			"echo $HOME",
			"ls *.go",
			"cat file | grep 'pattern' > output.txt",
			"cmd1 && cmd2 || cmd3",
			"echo \"quoted string with 'single quotes'\"",
			"ls `pwd`",
			"echo ${VAR:-default}",
		}

		for _, entry := range specialEntries {
			err := hm.Add(entry)
			require.NoError(t, err)
		}

		// Reload and verify
		hm2 := ui.NewHistoryManager(historyFile, 100)
		history := hm2.History()

		assert.Equal(t, specialEntries, history,
			"special shell characters should persist correctly")
	})
}
