package port

import (
	"testing"
)

// TestFileManagerInterface_Contract validates that FileManager interface exists with expected methods.
func TestFileManagerInterface_Contract(t *testing.T) {
	// Verify that FileManager interface exists
	var _ FileManager = (*mockFileManager)(nil)
}

// mockFileManager is a minimal implementation to validate interface contract
type mockFileManager struct{}

func (m *mockFileManager) ReadFile(path string) (string, error) {
	return "", nil
}

func (m *mockFileManager) WriteFile(path string, content string) error {
	return nil
}

func (m *mockFileManager) ListFiles(path string, recursive bool, includeGit bool) ([]string, error) {
	return nil, nil
}

func (m *mockFileManager) FileExists(path string) (bool, error) {
	return false, nil
}

func (m *mockFileManager) CreateDirectory(path string) error {
	return nil
}

func (m *mockFileManager) DeleteFile(path string) error {
	return nil
}

func (m *mockFileManager) GetFileInfo(path string) (FileInfo, error) {
	return FileInfo{}, nil
}

// TestFileManagerReadFile_Exists validates ReadFile method exists.
func TestFileManagerReadFile_Exists(t *testing.T) {
	var manager FileManager = (*mockFileManager)(nil)

	// This will fail to compile if ReadFile method doesn't exist with correct signature
	_ = manager.ReadFile
}

// TestFileManagerWriteFile_Exists validates WriteFile method exists.
func TestFileManagerWriteFile_Exists(t *testing.T) {
	var manager FileManager = (*mockFileManager)(nil)

	// This will fail to compile if WriteFile method doesn't exist with correct signature
	_ = manager.WriteFile
}

// TestFileManagerListFiles_Exists validates ListFiles method exists.
func TestFileManagerListFiles_Exists(t *testing.T) {
	var manager FileManager = (*mockFileManager)(nil)

	// This will fail to compile if ListFiles method doesn't exist with correct signature
	_ = manager.ListFiles
}

// TestFileManagerFileExists_Exists validates FileExists method exists.
func TestFileManagerFileExists_Exists(t *testing.T) {
	var manager FileManager = (*mockFileManager)(nil)

	// This will fail to compile if FileExists method doesn't exist with correct signature
	_ = manager.FileExists
}

// TestFileManagerCreateDirectory_Exists validates CreateDirectory method exists.
func TestFileManagerCreateDirectory_Exists(t *testing.T) {
	var manager FileManager = (*mockFileManager)(nil)

	// This will fail to compile if CreateDirectory method doesn't exist with correct signature
	_ = manager.CreateDirectory
}

// TestFileManagerDeleteFile_Exists validates DeleteFile method exists.
func TestFileManagerDeleteFile_Exists(t *testing.T) {
	var manager FileManager = (*mockFileManager)(nil)

	// This will fail to compile if DeleteFile method doesn't exist with correct signature
	_ = manager.DeleteFile
}

// TestFileManagerGetFileInfo_Exists validates GetFileInfo method exists.
func TestFileManagerGetFileInfo_Exists(t *testing.T) {
	var manager FileManager = (*mockFileManager)(nil)

	// This will fail to compile if GetFileInfo method doesn't exist with correct signature
	_ = manager.GetFileInfo
}
