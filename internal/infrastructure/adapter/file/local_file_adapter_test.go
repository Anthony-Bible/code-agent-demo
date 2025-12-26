package file_test

import (
	"code-editing-agent/internal/infrastructure/adapter/file"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests are intentionally written to fail during the Red Phase of TDD.
// They define the expected behavior of the LocalFileManager adapter before implementation.

func TestLocalFileManager_ReadFile(t *testing.T) {
	t.Run("read existing file successfully", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		filePath := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(filePath, []byte("hello world"), 0o644)
		require.NoError(t, err)

		content, err := fm.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "hello world", content)
	})

	t.Run("read non-existent file returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		filePath := filepath.Join(tempDir, "nonexistent.txt")
		_, err := fm.ReadFile(filePath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("attempt to read directory returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		dirPath := filepath.Join(tempDir, "testdir")
		err := os.Mkdir(dirPath, 0o755)
		require.NoError(t, err)

		_, err = fm.ReadFile(dirPath)
		require.Error(t, err)
		assert.Equal(t, "is a directory", err.Error())
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// This should be rejected because it tries to go outside tempDir
		path := filepath.Join(tempDir, "..", "etc", "passwd")
		_, err := fm.ReadFile(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal attempt detected")
	})
}

func TestLocalFileManager_WriteFile(t *testing.T) {
	t.Run("write new file successfully", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "new.txt")
		err := fm.WriteFile(path, "hello world")
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "existing.txt")
		err := os.WriteFile(path, []byte("old content"), 0o644)
		require.NoError(t, err)

		err = fm.WriteFile(path, "new content")
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "new content", string(content))
	})

	t.Run("write to directory path returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		dirPath := filepath.Join(tempDir, "subdir")
		err := os.Mkdir(dirPath, 0o755)
		require.NoError(t, err)

		err = fm.WriteFile(dirPath, "content")
		require.Error(t, err)
		assert.Equal(t, "is a directory", err.Error())
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "..", "etc", "malicious.txt")
		err := fm.WriteFile(path, "malicious content")
		require.Error(t, err)
		require.Contains(t, err.Error(), "path traversal attempt detected")
	})
}

func TestLocalFileManager_ListFiles(t *testing.T) {
	t.Run("list files non-recursive", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// Create test files and directories
		os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0o644)
		os.WriteFile(filepath.Join(tempDir, "file2.go"), []byte("package main"), 0o644)
		os.Mkdir(filepath.Join(tempDir, "subdir1"), 0o755)
		os.Mkdir(filepath.Join(tempDir, "subdir2"), 0o755)

		files, err := fm.ListFiles(tempDir, false, false)
		require.NoError(t, err)
		assert.Len(t, files, 4)

		fileMap := make(map[string]bool)
		for _, f := range files {
			fileMap[f] = true
		}
		assert.True(t, fileMap["file1.txt"])
		assert.True(t, fileMap["file2.go"])
		assert.True(t, fileMap["subdir1"])
		assert.True(t, fileMap["subdir2"])
	})

	t.Run("list files recursive", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// Create test files and directories
		os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0o644)
		os.WriteFile(filepath.Join(tempDir, "file2.go"), []byte("package main"), 0o644)
		os.Mkdir(filepath.Join(tempDir, "subdir1"), 0o755)
		os.WriteFile(filepath.Join(tempDir, "subdir1", "nested.txt"), []byte("nested"), 0o644)
		os.Mkdir(filepath.Join(tempDir, "subdir2"), 0o755)
		os.WriteFile(filepath.Join(tempDir, "subdir2", "file.md"), []byte("# README"), 0o644)

		files, err := fm.ListFiles(tempDir, true, false)
		require.NoError(t, err)
		assert.Len(t, files, 6)

		fileMap := make(map[string]bool)
		for _, f := range files {
			fileMap[f] = true
		}
		assert.True(t, fileMap["file1.txt"])
		assert.True(t, fileMap["file2.go"])
		assert.True(t, fileMap["subdir1/nested.txt"])
		assert.True(t, fileMap["subdir2/file.md"])
		assert.True(t, fileMap["subdir1"])
		assert.True(t, fileMap["subdir2"])
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "..", "etc")
		_, err := fm.ListFiles(path, false, false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "path traversal attempt detected")
	})

	t.Run("excludes .git directory by default (non-recursive)", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// Create .git directory and other files
		os.Mkdir(filepath.Join(tempDir, ".git"), 0o755)
		os.WriteFile(filepath.Join(tempDir, ".git", "config"), []byte("[core]\n"), 0o644)
		os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0o644)

		files, err := fm.ListFiles(tempDir, false, false)
		require.NoError(t, err)
		assert.Len(t, files, 1)
		assert.Equal(t, "file1.txt", files[0])
	})

	t.Run("includes .git directory when explicitly requested (non-recursive)", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// Create .git directory and other files
		os.Mkdir(filepath.Join(tempDir, ".git"), 0o755)
		os.WriteFile(filepath.Join(tempDir, ".git", "config"), []byte("[core]\n"), 0o644)
		os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0o644)

		files, err := fm.ListFiles(tempDir, false, true)
		require.NoError(t, err)
		assert.Len(t, files, 2)

		fileMap := make(map[string]bool)
		for _, f := range files {
			fileMap[f] = true
		}
		assert.True(t, fileMap[".git"])
		assert.True(t, fileMap["file1.txt"])
	})

	t.Run("excludes .git directory recursively", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// Create .git directory with content
		gitDir := filepath.Join(tempDir, ".git")
		os.Mkdir(gitDir, 0o755)
		os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]\n"), 0o644)
		os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
		os.Mkdir(filepath.Join(gitDir, "objects"), 0o755)
		os.Mkdir(filepath.Join(gitDir, "refs"), 0o755)
		os.WriteFile(filepath.Join(gitDir, "refs", "heads"), []byte(""), 0o644)

		// Create other files at root
		os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0o644)
		os.WriteFile(filepath.Join(tempDir, "file2.go"), []byte("package main"), 0o644)

		// Create subdirectory with files
		subdir := filepath.Join(tempDir, "subdir")
		os.Mkdir(subdir, 0o755)
		os.WriteFile(filepath.Join(subdir, "nested.txt"), []byte("nested content"), 0o644)

		files, err := fm.ListFiles(tempDir, true, false)
		require.NoError(t, err)
		assert.Len(t, files, 4)

		fileMap := make(map[string]bool)
		for _, f := range files {
			assert.NotContains(t, f, ".git", "Expected .git directory to be excluded")
			fileMap[f] = true
		}
		// Verify non-git files are included
		assert.True(t, fileMap["file1.txt"])
		assert.True(t, fileMap["file2.go"])
		assert.True(t, fileMap["subdir"])
		assert.True(t, fileMap["subdir/nested.txt"])
	})

	t.Run("includes .git directory when explicitly requested (recursive)", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// Create .git directory with content
		gitDir := filepath.Join(tempDir, ".git")
		os.Mkdir(gitDir, 0o755)
		os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]\n"), 0o644)
		os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
		os.Mkdir(filepath.Join(gitDir, "objects"), 0o755)
		os.Mkdir(filepath.Join(gitDir, "refs"), 0o755)

		// Create other files
		os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0o644)

		files, err := fm.ListFiles(tempDir, true, true)
		require.NoError(t, err)

		fileMap := make(map[string]bool)
		for _, f := range files {
			fileMap[f] = true
		}
		// Verify .git and its contents are included
		assert.True(t, fileMap[".git"])
		assert.True(t, fileMap[".git/config"])
		assert.True(t, fileMap[".git/HEAD"])
		assert.True(t, fileMap[".git/objects"])
		assert.True(t, fileMap[".git/refs"])
		// Verify other files are still included
		assert.True(t, fileMap["file1.txt"])
	})

	t.Run("excludes nested .git directories recursively", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// Create nested structure with .git in subdirectories
		git1 := filepath.Join(tempDir, "module1", ".git")
		git2 := filepath.Join(tempDir, "module2", "nested", ".git")
		os.MkdirAll(git1, 0o755)
		os.MkdirAll(git2, 0o755)
		os.WriteFile(filepath.Join(git1, "config"), []byte("[core]\n"), 0o644)
		os.WriteFile(filepath.Join(git2, "config"), []byte("[core]\n"), 0o644)

		// Create non-git files
		os.WriteFile(filepath.Join(tempDir, "module1", "main.go"), []byte("package main"), 0o644)
		os.WriteFile(filepath.Join(tempDir, "module2", "nested", "app.go"), []byte("package app"), 0o644)

		files, err := fm.ListFiles(tempDir, true, false)
		require.NoError(t, err)

		// Verify no .git paths are included
		for _, f := range files {
			assert.NotContains(t, f, ".git", "Expected no .git paths, got: "+f)
		}
		// Verify non-git files are included
		fileMap := make(map[string]bool)
		for _, f := range files {
			fileMap[f] = true
		}
		assert.True(t, fileMap["module1/main.go"])
		assert.True(t, fileMap["module2/nested/app.go"])
	})
}

func TestLocalFileManager_FileExists(t *testing.T) {
	t.Run("existing file returns true", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		filePath := filepath.Join(tempDir, "exists.txt")
		err := os.WriteFile(filePath, []byte("content"), 0o644)
		require.NoError(t, err)

		exists, err := fm.FileExists(filePath)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("non-existent file returns false", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		filePath := filepath.Join(tempDir, "nonexistent.txt")
		exists, err := fm.FileExists(filePath)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("path traversal prevention returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "..", "etc", "passwd")
		_, err := fm.FileExists(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "path traversal attempt detected")
	})
}

func TestLocalFileManager_CreateDirectory(t *testing.T) {
	t.Run("create single directory", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "newdir")
		err := fm.CreateDirectory(path)
		require.NoError(t, err)

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("create nested directories", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "a", "b", "c", "d")
		err := fm.CreateDirectory(path)
		require.NoError(t, err)

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "..", "etc", "malicious")
		err := fm.CreateDirectory(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "path traversal attempt detected")
	})
}

func TestLocalFileManager_DeleteFile(t *testing.T) {
	t.Run("delete existing file", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		filePath := filepath.Join(tempDir, "to-delete.txt")
		err := os.WriteFile(filePath, []byte("content"), 0o644)
		require.NoError(t, err)

		err = fm.DeleteFile(filePath)
		require.NoError(t, err)

		_, err = os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("delete empty directory", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		dirPath := filepath.Join(tempDir, "empty-dir")
		err := os.Mkdir(dirPath, 0o755)
		require.NoError(t, err)

		err = fm.DeleteFile(dirPath)
		require.NoError(t, err)

		_, err = os.Stat(dirPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "..", "etc", "passwd")
		err := fm.DeleteFile(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "path traversal attempt detected")
	})
}

func TestLocalFileManager_GetFileInfo(t *testing.T) {
	t.Run("get info for regular file", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		filePath := filepath.Join(tempDir, "test.txt")
		content := "hello world"
		err := os.WriteFile(filePath, []byte(content), 0o644)
		require.NoError(t, err)

		info, err := fm.GetFileInfo(filePath)
		require.NoError(t, err)
		assert.Equal(t, "test.txt", info.Name)
		assert.Equal(t, filePath, info.Path)
		assert.Equal(t, int64(len(content)), info.Size)
		assert.False(t, info.IsDirectory)
		assert.NotEmpty(t, info.Permissions)
		assert.False(t, info.Modified.IsZero())
	})

	t.Run("get info for directory", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		dirPath := filepath.Join(tempDir, "testdir")
		err := os.Mkdir(dirPath, 0o755)
		require.NoError(t, err)

		info, err := fm.GetFileInfo(dirPath)
		require.NoError(t, err)
		assert.Equal(t, "testdir", info.Name)
		assert.Equal(t, dirPath, info.Path)
		assert.True(t, info.IsDirectory)
		assert.True(t, strings.HasPrefix(info.Permissions, "d"))
		assert.False(t, info.Modified.IsZero())
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		path := filepath.Join(tempDir, "..", "etc", "passwd")
		_, err := fm.GetFileInfo(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "path traversal attempt detected")
	})
}

func TestLocalFileManager_ConcurrentOperations(t *testing.T) {
	t.Run("concurrent file reads and writes", func(t *testing.T) {
		tempDir := t.TempDir()
		fm := file.NewLocalFileManager(tempDir)

		// Create initial file
		filePath := filepath.Join(tempDir, "concurrent.txt")
		err := fm.WriteFile(filePath, "initial content")
		require.NoError(t, err)

		// Run multiple goroutines
		done := make(chan bool, 10)
		errors := make(chan error, 10)

		for i := range 5 {
			go func(_ int) {
				_, err := fm.ReadFile(filePath)
				if err != nil {
					errors <- err
					return
				}
				done <- true
			}(i)
		}

		for i := range 5 {
			go func(id int) {
				err := fm.WriteFile(filePath, fmt.Sprintf("content from goroutine %d", id))
				if err != nil {
					errors <- err
					return
				}
				done <- true
			}(i)
		}

		// Wait for all operations to complete
		for range 10 {
			select {
			case <-done:
			case err := <-errors:
				t.Errorf("Concurrent operation failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for concurrent operations")
			}
		}

		// Final read should succeed
		content, err := fm.ReadFile(filePath)
		require.NoError(t, err)
		assert.NotEmpty(t, content)
	})
}

func TestLocalFileManager_SecurityValidation(t *testing.T) {
	securityTests := []struct {
		name          string
		path          string
		expectedValid bool
	}{
		{"normal file path", "file.txt", true},
		{"subdirectory file", filepath.Join("subdir", "file.txt"), true},
		{"absolute path", "/etc/passwd", false},
		{"relative path with ..", filepath.Join("..", "etc", "passwd"), false},
		{"path with null byte", "file\x00.txt", false},
		{"empty path", "", false},
		{"path with pipe", "|ls", false},
	}

	for _, tt := range securityTests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			fm := file.NewLocalFileManager(tempDir)

			// Test that security validation works as expected
			if !tt.expectedValid {
				// ReadFile should fail
				_, err := fm.ReadFile(tt.path)
				require.Error(t, err)

				// WriteFile should fail
				err = fm.WriteFile(tt.path, "content")
				require.Error(t, err)
			}
		})
	}
}
