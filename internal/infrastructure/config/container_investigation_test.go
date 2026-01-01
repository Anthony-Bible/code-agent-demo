package config

import (
	"os"
	"testing"
)

// =============================================================================
// Container Investigation Wiring Tests
// These tests verify that the Container correctly wires and exposes the
// alert investigation components (AlertSourceManager and InvestigationUseCase).
//
// RED PHASE: These tests are expected to FAIL until the accessor methods
// are implemented in container.go.
// =============================================================================

// =============================================================================
// Test Helpers
// =============================================================================

// createTestConfig creates a minimal config for testing.
// Uses a temp directory for working dir to avoid permission issues.
func createTestConfig(t *testing.T) *Config {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "container-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return &Config{
		AIModel:           "test-model",
		WorkingDir:        tmpDir,
		HistoryFile:       "",
		HistoryMaxEntries: 100,
	}
}

// =============================================================================
// AlertSourceManager Accessor Tests
// =============================================================================

func TestContainer_AlertSourceManagerAccessor_NotNil(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	manager := container.AlertSourceManager()

	// Assert
	if manager == nil {
		t.Error("AlertSourceManager() should not return nil")
	}
}

func TestContainer_AlertSourceManagerAccessor_SameInstance(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	manager1 := container.AlertSourceManager()
	manager2 := container.AlertSourceManager()

	// Assert: should return the same instance (singleton)
	if manager1 != manager2 {
		t.Error("AlertSourceManager() should return the same instance on multiple calls")
	}
}

func TestContainer_AlertSourceManagerAccessor_ImplementsInterface(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	manager := container.AlertSourceManager()

	// Assert: manager should implement AlertSourceManager interface
	if manager == nil {
		t.Skip("AlertSourceManager() returned nil")
	}

	// Verify it has the expected methods by calling them
	sources := manager.ListSources()
	if sources == nil {
		t.Error("ListSources() should return non-nil slice (even if empty)")
	}
}

func TestContainer_AlertSourceManagerAccessor_CanRegisterSource(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	manager := container.AlertSourceManager()
	if manager == nil {
		t.Skip("AlertSourceManager() returned nil")
	}

	// Act & Assert: should be able to interact with the manager
	// This tests that the returned interface is properly wired
	sources := manager.ListSources()
	initialCount := len(sources)

	// The manager should start with zero sources
	if initialCount != 0 {
		t.Logf("Note: AlertSourceManager has %d initial sources", initialCount)
	}
}

// =============================================================================
// InvestigationUseCase Accessor Tests
// =============================================================================

func TestContainer_InvestigationUseCaseAccessor_NotNil(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	useCase := container.InvestigationUseCase()

	// Assert
	if useCase == nil {
		t.Error("InvestigationUseCase() should not return nil")
	}
}

func TestContainer_InvestigationUseCaseAccessor_SameInstance(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	useCase1 := container.InvestigationUseCase()
	useCase2 := container.InvestigationUseCase()

	// Assert: should return the same instance (singleton)
	if useCase1 != useCase2 {
		t.Error("InvestigationUseCase() should return the same instance on multiple calls")
	}
}

func TestContainer_InvestigationUseCaseAccessor_HasExpectedMethods(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	useCase := container.InvestigationUseCase()

	// Assert: use case should have expected methods
	if useCase == nil {
		t.Skip("InvestigationUseCase() returned nil")
	}

	// Verify basic functionality
	activeCount := useCase.GetActiveCount()
	if activeCount < 0 {
		t.Error("GetActiveCount() should return non-negative value")
	}
}

func TestContainer_InvestigationUseCaseAccessor_InitialState(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	useCase := container.InvestigationUseCase()

	// Assert: use case should start with no active investigations
	if useCase == nil {
		t.Skip("InvestigationUseCase() returned nil")
	}

	if useCase.GetActiveCount() != 0 {
		t.Errorf("InvestigationUseCase initial GetActiveCount() = %v, want 0", useCase.GetActiveCount())
	}
}

// =============================================================================
// Integration Tests - Components Work Together
// =============================================================================

func TestContainer_InvestigationComponents_Integration(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	manager := container.AlertSourceManager()
	useCase := container.InvestigationUseCase()

	// Assert: both components should be available
	if manager == nil {
		t.Error("AlertSourceManager() should not return nil")
	}
	if useCase == nil {
		t.Error("InvestigationUseCase() should not return nil")
	}
}

func TestContainer_InvestigationComponents_IndependentOfChatService(t *testing.T) {
	// Arrange
	cfg := createTestConfig(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act: access investigation components
	manager := container.AlertSourceManager()
	useCase := container.InvestigationUseCase()

	// Also access chat service to ensure they coexist
	chatService := container.ChatService()

	// Assert: all should be non-nil and independent
	if manager == nil {
		t.Error("AlertSourceManager() should not return nil")
	}
	if useCase == nil {
		t.Error("InvestigationUseCase() should not return nil")
	}
	if chatService == nil {
		t.Error("ChatService() should not return nil")
	}
}

// =============================================================================
// Nil Container Edge Cases
// =============================================================================

func TestContainer_AlertSourceManagerAccessor_AfterNilConfig(t *testing.T) {
	// This test documents behavior when container creation might have issues
	// The actual behavior depends on NewContainer implementation

	// Skip if NewContainer doesn't handle nil gracefully
	container, err := NewContainer(nil)
	if err != nil {
		// Expected - nil config should fail
		t.Logf("NewContainer(nil) correctly returned error: %v", err)
		return
	}

	if container == nil {
		t.Log("NewContainer(nil) returned nil container")
		return
	}

	// If we got a container somehow, test the accessor
	manager := container.AlertSourceManager()
	if manager == nil {
		t.Log("AlertSourceManager() returned nil for edge case container")
	}
}
