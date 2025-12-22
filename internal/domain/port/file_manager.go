package port

import "time"

// FileInfo represents metadata about a file or directory.
type FileInfo struct {
	Name        string    `json:"name"`         // Name of the file or directory
	Path        string    `json:"path"`         // Full path to the file or directory
	Size        int64     `json:"size"`         // Size in bytes
	Modified    time.Time `json:"modified"`     // Last modification time
	IsDirectory bool      `json:"is_directory"` // True if it's a directory
	Permissions string    `json:"permissions"`  // File permissions string
}

// FileManager defines the interface for file system operations.
// This port represents the outbound dependency to file system operations and follows
// hexagonal architecture principles by abstracting file management implementations.
type FileManager interface {
	// ReadFile reads the contents of a file and returns it as a string.
	ReadFile(path string) (string, error)

	// WriteFile writes the provided content to a file.
	WriteFile(path string, content string) error

	// ListFiles lists files and directories in the given path.
	// If recursive is true, it will include subdirectories.
	ListFiles(path string, recursive bool) ([]string, error)

	// FileExists checks if a file or directory exists at the given path.
	FileExists(path string) (bool, error)

	// CreateDirectory creates a new directory at the given path.
	CreateDirectory(path string) error

	// DeleteFile deletes a file or directory at the given path.
	DeleteFile(path string) error

	// GetFileInfo returns metadata about a file or directory.
	GetFileInfo(path string) (FileInfo, error)
}
