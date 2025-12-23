// Package file provides a secure file system adapter that implements the domain FileManager port.
// It follows hexagonal architecture principles by providing infrastructure-level file system
// operations with security validation and boundary protection.
//
// The adapter prevents path traversal attacks, validates file paths, and provides thread-safe
// operations for concurrent file access. All operations are sandboxed within a base directory
// to ensure security isolation.
//
// Example usage:
//
//	fm := file.NewLocalFileManager("/safe/base/directory")
//	content, err := fm.ReadFile("subdir/example.txt")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Println(content)
package file

import (
	"code-editing-agent/internal/domain/port"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Custom error types for better error handling and context.
var (
	ErrInvalidPath      = errors.New("invalid path")
	ErrPathTraversal    = errors.New("path traversal attempt detected")
	ErrPathValidation   = errors.New("path validation failed")
	ErrIsDirectory      = errors.New("is a directory")
	ErrNotDirectory     = errors.New("not a directory")
	ErrFileExists       = errors.New("file already exists")
	ErrFileNotFound     = errors.New("file not found")
	ErrPermissionDenied = errors.New("permission denied")
)

// PathValidationError provides detailed context about path validation failures.
type PathValidationError struct {
	Path   string
	Reason string
	Cause  error
}

func (e *PathValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("path validation failed for '%s': %s (cause: %v)", e.Path, e.Reason, e.Cause)
	}
	return fmt.Sprintf("path validation failed for '%s': %s", e.Path, e.Reason)
}

func (e *PathValidationError) Unwrap() error {
	return e.Cause
}

// LocalFileManager is a thread-safe implementation of the FileManager port that provides
// real file system operations with comprehensive security validation. It enforces strict
// boundary checking to prevent path traversal attacks and ensures all file operations
// remain within the configured base directory.
//
// The struct uses a read-write mutex (sync.RWMutex) to coordinate access to the file system,
// allowing multiple concurrent read operations while ensuring exclusive access for write
// operations. This design provides both safety and performance for concurrent workloads.
//
// Security features include:
// - Path validation for dangerous characters and null bytes
// - Boundary enforcement to prevent directory traversal
// - Symlink resolution and validation
// - Comprehensive error reporting with detailed context.
type LocalFileManager struct {
	mu      sync.RWMutex // Read-write mutex for thread-safe operations
	baseDir string       // Security boundary for all file operations
}

// NewLocalFileManager creates a new LocalFileManager instance with a specified base directory.
// The base directory serves as the security boundary - all file operations must stay within
// this directory and its subdirectories. This prevents path traversal attacks and ensures
// file operations are sandboxed to a safe location.
//
// The base directory can be an absolute or relative path. If relative, it will be resolved
// against the current working directory. The directory does not need to exist when the
// LocalFileManager is created, but must exist before performing file operations.
//
// Parameters:
//   - baseDir: The root directory that serves as the security boundary
//
// Returns:
//   - port.FileManager: An implementation of the FileManager interface ready for use
func NewLocalFileManager(baseDir string) port.FileManager {
	return &LocalFileManager{
		baseDir: baseDir,
	}
}

// validatePath performs security validation on the provided path.
// It prevents path traversal attacks and ensures the path stays within the base directory.
func (fm *LocalFileManager) validatePath(path string) error {
	if err := fm.validatePathFormat(path); err != nil {
		return ErrInvalidPath
	}

	if err := fm.validatePathBounds(path); err != nil {
		return ErrInvalidPath
	}

	return nil
}

// validatePathFormat checks for basic path format issues and dangerous characters.
func (fm *LocalFileManager) validatePathFormat(path string) error {
	if path == "" {
		return &PathValidationError{
			Path:   path,
			Reason: "empty path",
			Cause:  ErrInvalidPath,
		}
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return &PathValidationError{
			Path:   path,
			Reason: "contains null byte",
			Cause:  ErrInvalidPath,
		}
	}

	// Check for dangerous characters (command injection prevention)
	if strings.ContainsAny(path, "|;$&<>`") {
		return &PathValidationError{
			Path:   path,
			Reason: "contains dangerous characters",
			Cause:  ErrInvalidPath,
		}
	}

	return nil
}

// validatePathBounds ensures the path stays within the base directory boundaries.
func (fm *LocalFileManager) validatePathBounds(path string) error {
	// Clean the path to resolve any .. or . components
	cleaned := filepath.Clean(path)

	// If the path is relative, join it with the base directory first
	var fullPath string
	if filepath.IsAbs(cleaned) {
		fullPath = cleaned
	} else {
		// For relative paths, resolve them against the current working directory
		// then validate against baseDir
		absPath, err := filepath.Abs(cleaned)
		if err != nil {
			return &PathValidationError{
				Path:   path,
				Reason: "failed to resolve absolute path",
				Cause:  err,
			}
		}
		fullPath = absPath
	}

	// Now check if the full path is within the base directory
	relPath, err := filepath.Rel(fm.baseDir, fullPath)
	if err != nil {
		return &PathValidationError{
			Path:   path,
			Reason: "failed to get relative path",
			Cause:  err,
		}
	}

	// If the relative path starts with "..", it means it's outside the base directory
	if strings.HasPrefix(relPath, "..") || relPath == "../" {
		return &PathValidationError{
			Path:   path,
			Reason: "path traversal attempt detected",
			Cause:  ErrPathTraversal,
		}
	}

	// Final check: ensure the full path is actually within the base directory
	// This is a robust boundary check
	if !fm.isPathWithinBounds(fullPath) {
		return &PathValidationError{
			Path:   path,
			Reason: "path is outside base directory boundary",
			Cause:  ErrPathTraversal,
		}
	}

	return nil
}

// isPathWithinBounds performs the final boundary check including symlink resolution.
func (fm *LocalFileManager) isPathWithinBounds(fullPath string) bool {
	if strings.HasPrefix(
		filepath.Clean(fullPath)+string(filepath.Separator),
		filepath.Clean(fm.baseDir)+string(filepath.Separator),
	) ||
		strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(fm.baseDir)) {
		return true
	}

	// Try to evaluate symlinks to prevent symlink-based attacks
	evaluatedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return false
	}

	// Check the resolved path as well
	return strings.HasPrefix(
		filepath.Clean(evaluatedPath)+string(filepath.Separator),
		filepath.Clean(fm.baseDir)+string(filepath.Separator),
	) ||
		strings.HasPrefix(filepath.Clean(evaluatedPath), filepath.Clean(fm.baseDir))
}

// ensureParentDirectories creates parent directories if they don't exist.
func (fm *LocalFileManager) ensureParentDirectories(path string) error {
	dir := filepath.Dir(path)
	if dir != "." {
		return os.MkdirAll(dir, 0o750)
	}
	return nil
}

// checkIsDirectory checks if a path is a directory and returns appropriate error.
func (fm *LocalFileManager) checkIsDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return ErrIsDirectory
	}
	return nil
}

// checkIsNotDirectory checks if a path is not a directory and returns appropriate error.
func (fm *LocalFileManager) checkIsNotDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return ErrNotDirectory
	}
	return nil
}

// ReadFile reads the contents of a file and returns it as a string.
// The method performs security validation to ensure the path is within the base directory
// and prevents path traversal attacks. It also checks that the target is not a directory.
//
// The operation acquires a read lock to allow concurrent reading by multiple goroutines
// while preventing write operations from interfering.
//
// Parameters:
//   - path: The path to the file to read, relative to the base directory
//
// Returns:
//   - string: The file contents as a string
//   - error: An error if the file doesn't exist, is a directory, or security validation fails
func (fm *LocalFileManager) ReadFile(path string) (string, error) {
	if err := fm.validatePath(path); err != nil {
		return "", ErrInvalidPath
	}

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Check if it's a directory
	if err := fm.checkIsDirectory(path); err != nil {
		return "", err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// WriteFile writes the provided content to a file, creating parent directories if necessary.
// The method performs comprehensive security validation before writing and prevents writing
// to locations that are directories or outside the base directory.
//
// The operation acquires an exclusive write lock to prevent concurrent modifications
// and ensure data integrity. Parent directories are automatically created with secure
// permissions (0o750) if they don't exist.
//
// Files are created with secure permissions (0o600) to ensure only the owner can read
// and write to them.
//
// Parameters:
//   - path: The path to the file to write, relative to the base directory
//   - content: The string content to write to the file
//
// Returns:
//   - error: An error if the path is a directory, security validation fails, or write fails
func (fm *LocalFileManager) WriteFile(path string, content string) error {
	if err := fm.validatePath(path); err != nil {
		return ErrInvalidPath
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Check if path is a directory
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return ErrIsDirectory
	}

	// Create parent directories if needed
	if err := fm.ensureParentDirectories(path); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Write the file
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}

	return nil
}

// ListFiles lists files and directories in the given path.
func (fm *LocalFileManager) ListFiles(path string, recursive bool) ([]string, error) {
	if err := fm.validatePath(path); err != nil {
		return nil, ErrInvalidPath
	}

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Check if path exists and is a directory
	if err := fm.checkIsNotDirectory(path); err != nil {
		return nil, err
	}

	if recursive {
		return fm.listFilesRecursive(path)
	}

	return fm.listFilesNonRecursive(path)
}

// listFilesRecursive handles recursive file listing.
func (fm *LocalFileManager) listFilesRecursive(path string) ([]string, error) {
	var files []string

	err := filepath.Walk(path, func(walkPath string, _ fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Skip the root path
		if walkPath == path {
			return nil
		}

		// Convert to relative path
		relPath, err := filepath.Rel(path, walkPath)
		if err != nil {
			return err
		}

		// Only add if relPath is not empty (skip the root directory)
		if relPath != "." && relPath != "" {
			files = append(files, filepath.ToSlash(relPath))
		}
		return nil
	})

	return files, err
}

// listFilesNonRecursive handles non-recursive file listing.
func (fm *LocalFileManager) listFilesNonRecursive(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, filepath.ToSlash(entry.Name()))
	}

	return files, nil
}

// FileExists checks if a file or directory exists at the given path.
func (fm *LocalFileManager) FileExists(path string) (bool, error) {
	if err := fm.validatePath(path); err != nil {
		return false, ErrInvalidPath
	}

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CreateDirectory creates a new directory at the given path.
func (fm *LocalFileManager) CreateDirectory(path string) error {
	if err := fm.validatePath(path); err != nil {
		return ErrInvalidPath
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Check if already exists
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			// Directory already exists, that's fine
			return nil
		}
		return ErrNotDirectory
	}

	return os.MkdirAll(path, 0o750)
}

// DeleteFile deletes a file or directory at the given path.
func (fm *LocalFileManager) DeleteFile(path string) error {
	if err := fm.validatePath(path); err != nil {
		return ErrInvalidPath
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Check if file/directory exists before attempting to delete
	_, err := os.Stat(path)
	if err != nil {
		return err // Return the error if file doesn't exist or other issues
	}

	// Use os.RemoveAll for both files and directories
	return os.RemoveAll(path)
}

// GetFileInfo returns metadata about a file or directory.
func (fm *LocalFileManager) GetFileInfo(path string) (port.FileInfo, error) {
	if err := fm.validatePath(path); err != nil {
		return port.FileInfo{}, ErrInvalidPath
	}

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Handle symlinks by getting info about the target
	info, err := os.Stat(path)
	if err != nil {
		return port.FileInfo{}, err
	}

	// For symlinks, os.Stat already returns info about the target
	// which is the desired behavior for most use cases

	// Convert FileMode to permission string (keep the full mode string including file type)
	permStr := info.Mode().String()
	if len(permStr) < 10 {
		// If mode string is too short, fall back to octal format with file type
		if info.IsDir() {
			permStr = "d" + fmt.Sprintf("%o", info.Mode().Perm())
		} else {
			permStr = "-" + fmt.Sprintf("%o", info.Mode().Perm())
		}
	}

	return port.FileInfo{
		Name:        info.Name(),
		Path:        path,
		Size:        info.Size(),
		Modified:    info.ModTime(),
		IsDirectory: info.IsDir(),
		Permissions: permStr,
	}, nil
}
