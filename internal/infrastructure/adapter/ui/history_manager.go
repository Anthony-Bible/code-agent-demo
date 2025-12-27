// Package ui provides user interface adapters for the CLI application.
package ui

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrEmptyEntry is returned when attempting to add an empty or whitespace-only entry.
var ErrEmptyEntry = errors.New("history: entry cannot be empty or whitespace-only")

// ErrEmbeddedNewline is returned when attempting to add an entry containing embedded newlines.
// Entries with newlines would corrupt the line-based file format used for persistence.
var ErrEmbeddedNewline = errors.New("history: entry cannot contain embedded newlines")

// ErrConsecutiveDuplicate is returned when attempting to add a duplicate of the last entry.
var ErrConsecutiveDuplicate = errors.New("history: consecutive duplicate entry not allowed")

// HistoryManager manages command history for the CLI.
type HistoryManager struct {
	filePath   string
	maxEntries int
	history    []string
	mu         sync.RWMutex
}

// NewHistoryManager creates a new HistoryManager.
// filePath: path to history file (empty string for in-memory only mode)
// maxEntries: maximum number of entries to keep (0 or negative for unlimited).
func NewHistoryManager(filePath string, maxEntries int) *HistoryManager {
	if maxEntries < 0 {
		maxEntries = 0
	}
	hm := &HistoryManager{
		filePath:   ExpandPath(filePath),
		maxEntries: maxEntries,
		history:    []string{},
	}
	hm.load()
	return hm
}

// Add adds a new entry to the history.
// The entry is trimmed of leading/trailing whitespace before storage.
// Returns ErrEmptyEntry if the entry is empty or whitespace-only.
// Returns ErrEmbeddedNewline if the entry contains embedded newlines.
// Returns ErrConsecutiveDuplicate if the entry matches the most recent entry.
func (hm *HistoryManager) Add(entry string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return ErrEmptyEntry
	}

	// Reject entries with embedded newlines (they would corrupt file format)
	if strings.Contains(trimmed, "\n") {
		return ErrEmbeddedNewline
	}

	if hm.isConsecutiveDuplicate(trimmed) {
		return ErrConsecutiveDuplicate
	}

	hm.history = append(hm.history, trimmed)

	// Check if trimming is needed
	if hm.maxEntries > 0 && len(hm.history) > hm.maxEntries {
		hm.trimToMaxEntries()
		hm.rewriteFile() // Full rewrite after trim
	} else {
		hm.appendToFile(trimmed) // Just append
	}

	return nil
}

// isConsecutiveDuplicate checks if the entry matches the most recent history entry.
// Must be called with mu held.
func (hm *HistoryManager) isConsecutiveDuplicate(entry string) bool {
	if len(hm.history) == 0 {
		return false
	}
	return hm.history[len(hm.history)-1] == entry
}

// trimToMaxEntries removes oldest entries if history exceeds maxEntries.
// A maxEntries of 0 means unlimited (no trimming).
// Must be called with mu held.
func (hm *HistoryManager) trimToMaxEntries() {
	if hm.maxEntries > 0 && len(hm.history) > hm.maxEntries {
		hm.history = hm.history[len(hm.history)-hm.maxEntries:]
	}
}

// History returns a copy of all history entries in order added (oldest first).
func (hm *HistoryManager) History() []string {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]string, len(hm.history))
	copy(result, hm.history)
	return result
}

// Size returns the number of entries in the history.
func (hm *HistoryManager) Size() int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	return len(hm.history)
}

// Clear removes all entries from the history.
func (hm *HistoryManager) Clear() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.history = []string{}
	hm.rewriteFile() // Clear the file as well
}

// Last returns the most recent entry and true, or empty string and false if history is empty.
func (hm *HistoryManager) Last() (string, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if len(hm.history) == 0 {
		return "", false
	}
	return hm.history[len(hm.history)-1], true
}

// ExpandPath expands a tilde prefix to the user's home directory.
// It handles two cases:
//   - "~" alone expands to the home directory
//   - "~/..." expands to home directory joined with the rest of the path
//
// Paths without a leading tilde, or with ~username format, are returned unchanged.
// If the home directory cannot be determined, the original path is returned.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}

	if path == "~" {
		return getHomeDir(path)
	}

	if strings.HasPrefix(path, "~/") {
		homeDir := getHomeDir(path)
		if homeDir == path {
			// getHomeDir returned original path due to error
			return path
		}
		// Use filepath.Join for cross-platform path concatenation
		return filepath.Join(homeDir, path[2:])
	}

	return path
}

// getHomeDir returns the user's home directory, or the fallback value if unavailable.
func getHomeDir(fallback string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fallback
	}
	return homeDir
}

// File permission constants for history file operations.
const (
	// filePermission is the permission mode for the history file (user read/write only).
	filePermission = 0o600
	// dirPermission is the permission mode for parent directories (user read/write/execute only).
	dirPermission = 0o700
)

// load reads history entries from the file during initialization.
// Each non-empty line in the file becomes a history entry.
// Lines are trimmed of leading/trailing whitespace before storage.
// File errors are non-fatal; in-memory history starts empty on error.
// Must be called during construction before the manager is used concurrently.
func (hm *HistoryManager) load() {
	if hm.filePath == "" {
		return
	}

	file, err := os.Open(hm.filePath)
	if err != nil {
		// File doesn't exist or can't be read - start with empty history
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			hm.history = append(hm.history, line)
		}
	}

	// Apply maxEntries limit after loading
	hm.trimToMaxEntries()
}

// ensureParentDir creates parent directories for the history file if they don't exist.
// Returns true if the directories exist or were created successfully, false otherwise.
func (hm *HistoryManager) ensureParentDir() bool {
	dir := filepath.Dir(hm.filePath)
	return os.MkdirAll(dir, dirPermission) == nil
}

// appendToFile appends a single entry to the history file.
// Creates parent directories and the file if they don't exist.
// File errors are silently ignored to allow in-memory operation when file I/O fails.
// Must be called with mu held.
func (hm *HistoryManager) appendToFile(entry string) {
	if hm.filePath == "" {
		return
	}

	if !hm.ensureParentDir() {
		return
	}

	file, err := os.OpenFile(hm.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePermission)
	if err != nil {
		return
	}
	defer file.Close()

	_, _ = file.WriteString(entry + "\n")
}

// rewriteFile rewrites the entire history file with current in-memory history.
// This is used after trimming to ensure file contents match in-memory state.
// Creates parent directories and the file if they don't exist.
// File errors are silently ignored to allow in-memory operation when file I/O fails.
// Must be called with mu held.
func (hm *HistoryManager) rewriteFile() {
	if hm.filePath == "" {
		return
	}

	if !hm.ensureParentDir() {
		return
	}

	file, err := os.OpenFile(hm.filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePermission)
	if err != nil {
		return
	}
	defer file.Close()

	for _, entry := range hm.history {
		_, _ = file.WriteString(entry + "\n")
	}
}
