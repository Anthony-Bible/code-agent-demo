package config

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Container Subagent Wiring Tests (Phase 7 RED)
// =============================================================================
//
// These tests verify that the Container correctly wires and exposes the
// subagent system components following the same pattern as the investigation
// and skill system wiring.
//
// The subagent system consists of three main components:
//   1. SubagentManager - Discovers and loads subagent metadata from:
//      - ./agents (project root, highest priority)
//      - ./.claude/agents (project .claude directory)
//      - ~/.claude/agents (user global, lowest priority)
//   2. SubagentRunner - Executes subagent tasks in isolated conversation contexts
//   3. SubagentUseCase - High-level orchestration of subagent spawning
//
// Additionally, the ToolExecutor must have SetSubagentUseCase() called to enable
// the "task" tool for delegating work to subagents.
//
// RED PHASE: These tests are EXPECTED TO FAIL until the container wiring
// is implemented in container.go following the same pattern as SkillManager
// and InvestigationUseCase.
//
// =============================================================================

// =============================================================================
// Test Helpers
// =============================================================================

// createTestConfigForSubagent creates a minimal config with a temp directory
// that includes the agents directory structure for testing.
func createTestConfigForSubagent(t *testing.T) *Config {
	t.Helper()

	tmpDir := t.TempDir()

	// Create agents directories to prevent errors
	agentsDir := filepath.Join(tmpDir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}

	claudeAgentsDir := filepath.Join(tmpDir, ".claude", "agents")
	if err := os.MkdirAll(claudeAgentsDir, 0o755); err != nil {
		t.Fatalf("Failed to create .claude/agents dir: %v", err)
	}

	return &Config{
		AIModel:           "test-model",
		WorkingDir:        tmpDir,
		HistoryFile:       "",
		HistoryMaxEntries: 100,
	}
}

// createTestSubagent creates a minimal test subagent in the given directory.
func createTestSubagent(t *testing.T, baseDir, agentName string) {
	t.Helper()

	agentDir := filepath.Join(baseDir, "agents", agentName)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("Failed to create agent dir: %v", err)
	}

	agentMDPath := filepath.Join(agentDir, "AGENT.md")
	content := `---
name: ` + agentName + `
description: Test agent for container wiring
allowed-tools: bash read_file list_files
model: inherit
---

# Test Agent

This is a test agent for verifying container wiring.
`

	if err := os.WriteFile(agentMDPath, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create AGENT.md: %v", err)
	}
}

// =============================================================================
// SubagentManager Accessor Tests
// =============================================================================

// TestContainer_SubagentManagerAccessor_NotNil verifies that the container
// provides access to a non-nil SubagentManager instance.
func TestContainer_SubagentManagerAccessor_NotNil(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	manager := container.SubagentManager()

	// Assert
	if manager == nil {
		t.Error("SubagentManager() should not return nil")
	}
}

// TestContainer_SubagentManagerAccessor_SameInstance verifies that multiple
// calls to SubagentManager() return the same instance (singleton pattern).
func TestContainer_SubagentManagerAccessor_SameInstance(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	manager1 := container.SubagentManager()
	manager2 := container.SubagentManager()

	// Assert: should return the same instance (singleton)
	if manager1 != manager2 {
		t.Error("SubagentManager() should return the same instance on multiple calls")
	}
}

// TestContainer_SubagentManagerAccessor_ImplementsInterface verifies that
// the returned SubagentManager properly implements the port.SubagentManager interface.
func TestContainer_SubagentManagerAccessor_ImplementsInterface(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	manager := container.SubagentManager()

	// Assert: manager should implement SubagentManager interface
	if manager == nil {
		t.Skip("SubagentManager() returned nil")
	}

	// Verify it has the expected methods by calling them
	ctx := context.Background()
	result, err := manager.DiscoverAgents(ctx)
	if err != nil {
		t.Errorf("DiscoverAgents() error = %v", err)
	}
	if result == nil {
		t.Error("DiscoverAgents() should return non-nil result")
	}
}

// TestContainer_SubagentManagerAccessor_CanDiscoverAgents verifies that
// the SubagentManager can successfully discover subagents from the configured directories.
func TestContainer_SubagentManagerAccessor_CanDiscoverAgents(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	createTestSubagent(t, cfg.WorkingDir, "test-agent")

	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	manager := container.SubagentManager()
	if manager == nil {
		t.Skip("SubagentManager() returned nil")
	}

	// Act
	ctx := context.Background()
	result, err := manager.DiscoverAgents(ctx)
	// Assert
	if err != nil {
		t.Errorf("DiscoverAgents() error = %v", err)
	}

	if result == nil {
		t.Fatal("DiscoverAgents() returned nil result")
	}

	// Should discover the test agent
	if result.TotalCount < 1 {
		t.Errorf("DiscoverAgents() TotalCount = %d, want >= 1", result.TotalCount)
	}

	// Should have the expected agent
	foundTestAgent := false
	for _, agent := range result.Subagents {
		if agent.Name == "test-agent" {
			foundTestAgent = true
			break
		}
	}
	if !foundTestAgent {
		t.Error("DiscoverAgents() should find 'test-agent'")
	}
}

// TestContainer_SubagentManagerAccessor_ConfiguresDiscoveryDirectories verifies
// that the SubagentManager is configured with the correct discovery directories
// following the priority order: ./agents, ./.claude/agents, ~/.claude/agents.
func TestContainer_SubagentManagerAccessor_ConfiguresDiscoveryDirectories(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	manager := container.SubagentManager()
	if manager == nil {
		t.Skip("SubagentManager() returned nil")
	}

	// Act
	ctx := context.Background()
	result, err := manager.DiscoverAgents(ctx)
	// Assert
	if err != nil {
		t.Errorf("DiscoverAgents() error = %v", err)
	}

	if result == nil {
		t.Fatal("DiscoverAgents() returned nil result")
	}

	// Should have at least the two project directories configured
	// (./agents and ./.claude/agents)
	// ~/.claude/agents is added only if home directory exists
	if len(result.AgentsDirs) < 2 {
		t.Errorf("DiscoverAgents() AgentsDirs count = %d, want >= 2", len(result.AgentsDirs))
	}

	// First directory should be ./agents (relative to working dir)
	expectedProjectAgentsDir := filepath.Join(cfg.WorkingDir, "agents")
	if len(result.AgentsDirs) > 0 && result.AgentsDirs[0] != expectedProjectAgentsDir {
		t.Errorf(
			"First AgentsDirs[0] = %v, want %v (project agents dir)",
			result.AgentsDirs[0],
			expectedProjectAgentsDir,
		)
	}

	// Second directory should be ./.claude/agents (relative to working dir)
	expectedClaudeAgentsDir := filepath.Join(cfg.WorkingDir, ".claude", "agents")
	if len(result.AgentsDirs) > 1 && result.AgentsDirs[1] != expectedClaudeAgentsDir {
		t.Errorf(
			"Second AgentsDirs[1] = %v, want %v (.claude agents dir)",
			result.AgentsDirs[1],
			expectedClaudeAgentsDir,
		)
	}
}

// =============================================================================
// SubagentUseCase Accessor Tests
// =============================================================================

// TestContainer_SubagentUseCaseAccessor_NotNil verifies that the container
// provides access to a non-nil SubagentUseCase instance.
func TestContainer_SubagentUseCaseAccessor_NotNil(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	uc := container.SubagentUseCase()

	// Assert
	if uc == nil {
		t.Error("SubagentUseCase() should not return nil")
	}
}

// TestContainer_SubagentUseCaseAccessor_SameInstance verifies that multiple
// calls to SubagentUseCase() return the same instance (singleton pattern).
func TestContainer_SubagentUseCaseAccessor_SameInstance(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	uc1 := container.SubagentUseCase()
	uc2 := container.SubagentUseCase()

	// Assert: should return the same instance (singleton)
	if uc1 != uc2 {
		t.Error("SubagentUseCase() should return the same instance on multiple calls")
	}
}

// =============================================================================
// Task Tool Integration Tests
// =============================================================================

// TestContainer_TaskToolIntegration_SetSubagentUseCaseIsCalled verifies that
// SetSubagentUseCase() is called on the ToolExecutor during container creation,
// which is required for the task tool to function.
func TestContainer_TaskToolIntegration_SetSubagentUseCaseIsCalled(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	toolExecutor := container.ToolExecutor()
	if toolExecutor == nil {
		t.Fatal("ToolExecutor() returned nil")
	}

	// Get available tools
	tools, err := toolExecutor.ListTools()
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	// Assert: task tool should be available
	taskToolFound := false
	for _, tool := range tools {
		if tool.Name == "task" {
			taskToolFound = true
			break
		}
	}

	if !taskToolFound {
		t.Error("task tool should be available in ToolExecutor after SetSubagentUseCase() is called")
	}
}

// TestContainer_TaskToolIntegration_TaskToolInAvailableTools verifies that
// the task tool appears in the available tools list after wiring.
func TestContainer_TaskToolIntegration_TaskToolInAvailableTools(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act
	toolExecutor := container.ToolExecutor()
	if toolExecutor == nil {
		t.Fatal("ToolExecutor() returned nil")
	}

	tools, err := toolExecutor.ListTools()
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	// Assert
	var taskTool *entity.Tool
	for _, tool := range tools {
		if tool.Name == "task" {
			taskTool = &tool
			break
		}
	}

	if taskTool == nil {
		t.Fatal("task tool not found in available tools")
	}

	// Verify task tool has expected properties
	if taskTool.Description == "" {
		t.Error("task tool should have a non-empty description")
	}

	// Task tool should have agent_name and prompt in its input schema
	// (This is a basic sanity check - full schema validation is in the tool tests)
	if taskTool.InputSchema == nil {
		t.Error("task tool should have an input schema")
	}
}

// TestContainer_TaskToolIntegration_TaskToolCanExecute verifies that the
// task tool is properly registered and can be retrieved.
// NOTE: This test verifies WIRING only, not actual execution (which would hit real APIs).
// Actual task tool execution is tested in subagent_runner_test.go with mocks.
func TestContainer_TaskToolIntegration_TaskToolCanExecute(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	createTestSubagent(t, cfg.WorkingDir, "test-executor")

	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	toolExecutor := container.ToolExecutor()
	if toolExecutor == nil {
		t.Fatal("ToolExecutor() returned nil")
	}

	// Assert: task tool is registered and can be retrieved (wiring check, no API call)
	tool, found := toolExecutor.GetTool("task")
	if !found {
		t.Fatal("task tool should be registered")
	}
	if tool.InputSchema == nil {
		t.Error("task tool should have input schema")
	}
}

// =============================================================================
// Integration Tests - All Components Work Together
// =============================================================================

// TestContainer_SubagentComponents_AllWiredTogether verifies that all
// subagent components are wired correctly and work together.
func TestContainer_SubagentComponents_AllWiredTogether(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	createTestSubagent(t, cfg.WorkingDir, "integration-test")

	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act: Access all components
	manager := container.SubagentManager()
	useCase := container.SubagentUseCase()
	toolExecutor := container.ToolExecutor()

	// Assert: All components should be non-nil
	if manager == nil {
		t.Error("SubagentManager() should not return nil")
	}
	if useCase == nil {
		t.Error("SubagentUseCase() should not return nil")
	}
	if toolExecutor == nil {
		t.Error("ToolExecutor() should not return nil")
	}
}

// TestContainer_SubagentComponents_EndToEndFlow verifies the end-to-end
// subagent wiring through the container-wired components.
// NOTE: This test verifies WIRING only, not actual execution (which would hit real APIs).
// Actual SpawnSubagent execution is tested in subagent_runner_test.go with mocks.
func TestContainer_SubagentComponents_EndToEndFlow(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	createTestSubagent(t, cfg.WorkingDir, "e2e-test")

	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	ctx := context.Background()

	// Step 1: Verify discovery works (no API call)
	manager := container.SubagentManager()
	if manager == nil {
		t.Fatal("SubagentManager() returned nil")
	}

	discoveryResult, err := manager.DiscoverAgents(ctx)
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}
	if discoveryResult.TotalCount < 1 {
		t.Fatal("Should discover at least 1 agent")
	}

	// Step 2: Verify SubagentUseCase is wired (no API call)
	useCase := container.SubagentUseCase()
	if useCase == nil {
		t.Fatal("SubagentUseCase() returned nil")
	}

	// Step 3: Verify task tool is registered (no API call)
	toolExecutor := container.ToolExecutor()
	if toolExecutor == nil {
		t.Fatal("ToolExecutor() returned nil")
	}

	_, found := toolExecutor.GetTool("task")
	if !found {
		t.Fatal("task tool should be registered")
	}
}

// =============================================================================
// Edge Cases and Validation Tests
// =============================================================================

// TestContainer_SubagentComponents_CoexistWithOtherServices verifies that
// the subagent components coexist properly with other container services.
func TestContainer_SubagentComponents_CoexistWithOtherServices(t *testing.T) {
	// Arrange
	cfg := createTestConfigForSubagent(t)
	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer() error = %v", err)
	}

	// Act: Access all major services
	chatService := container.ChatService()
	convService := container.ConversationService()
	skillManager := container.SkillManager()
	alertSourceManager := container.AlertSourceManager()
	investigationUseCase := container.InvestigationUseCase()
	subagentManager := container.SubagentManager()
	subagentUseCase := container.SubagentUseCase()

	// Assert: All should coexist without conflict
	if chatService == nil {
		t.Error("ChatService() should not return nil")
	}
	if convService == nil {
		t.Error("ConversationService() should not return nil")
	}
	if skillManager == nil {
		t.Error("SkillManager() should not return nil")
	}
	if alertSourceManager == nil {
		t.Error("AlertSourceManager() should not return nil")
	}
	if investigationUseCase == nil {
		t.Error("InvestigationUseCase() should not return nil")
	}
	if subagentManager == nil {
		t.Error("SubagentManager() should not return nil")
	}
	if subagentUseCase == nil {
		t.Error("SubagentUseCase() should not return nil")
	}
}

// TestContainer_SubagentComponents_NilConfigHandling verifies behavior
// when container creation fails due to nil config.
func TestContainer_SubagentComponents_NilConfigHandling(t *testing.T) {
	// Act
	container, err := NewContainer(nil)

	// Assert: Should fail gracefully
	if err == nil {
		t.Error("NewContainer(nil) should return an error")
	}
	if container != nil {
		t.Error("NewContainer(nil) should return nil container on error")
	}
}
