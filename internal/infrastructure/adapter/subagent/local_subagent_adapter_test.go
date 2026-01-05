package subagent

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// =============================================================================
// Helper Functions
// =============================================================================

// createAgentFile is a helper to create an AGENT.md file with given name and description.
func createAgentFile(t *testing.T, dir, agentName, description string) {
	t.Helper()
	agentDir := filepath.Join(dir, agentName)
	if err := os.MkdirAll(agentDir, 0o750); err != nil {
		t.Fatalf("Failed to create agent directory %s: %v", agentDir, err)
	}

	content := "---\nname: " + agentName + "\ndescription: " + description + "\n---\nContent for " + agentName
	agentFile := filepath.Join(agentDir, "AGENT.md")
	if err := os.WriteFile(agentFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write AGENT.md at %s: %v", agentFile, err)
	}
}

// =============================================================================
// Basic Discovery Tests
// =============================================================================

func TestLocalSubagentManager_DiscoverAgents_EmptyAgentsDirectory(t *testing.T) {
	tempDir := t.TempDir()
	emptyAgentsDir := filepath.Join(tempDir, "empty-agents")

	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: emptyAgentsDir, SourceType: entity.SubagentSourceProject},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}
	result, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("DiscoverAgents() returned nil result")
	}

	if result.TotalCount != 0 {
		t.Errorf("DiscoverAgents() TotalCount = %d, want 0", result.TotalCount)
	}

	if len(result.Subagents) != 0 {
		t.Errorf("DiscoverAgents() returned %d agents, want 0", len(result.Subagents))
	}
}

func TestLocalSubagentManager_DiscoverAgents_WithAgentFiles(t *testing.T) {
	// Create a temporary agents directory
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	testAgentDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(testAgentDir, 0o750); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create an AGENT.md file
	agentContent := `---
name: test-agent
description: A test agent
---
Test content`
	if err := os.WriteFile(filepath.Join(testAgentDir, "AGENT.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatalf("Failed to write AGENT.md: %v", err)
	}

	// Create agent manager with custom agents dir
	sm := &LocalSubagentManager{
		agentsDirs:   []DirConfig{{Path: agentsDir, SourceType: entity.SubagentSourceProject}},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	result, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	if result.TotalCount != 1 {
		t.Errorf("DiscoverAgents() TotalCount = %d, want 1", result.TotalCount)
	}

	if len(result.Subagents) != 1 {
		t.Fatalf("DiscoverAgents() returned %d agents, want 1", len(result.Subagents))
	}

	agent := result.Subagents[0]
	if agent.Name != "test-agent" {
		t.Errorf("DiscoverAgents() agent Name = %v, want 'test-agent'", agent.Name)
	}

	if agent.Description != "A test agent" {
		t.Errorf("DiscoverAgents() agent Description = %v, want 'A test agent'", agent.Description)
	}
}

// =============================================================================
// Multi-Directory Discovery Tests
// =============================================================================

// TestLocalSubagentManager_DiscoverAgents_MultipleDirectories verifies that agents are
// discovered from all three directory locations:
// - User global (~/.claude/agents)
// - Project .claude directory (./.claude/agents)
// - Project root (./agents).
func TestLocalSubagentManager_DiscoverAgents_MultipleDirectories(t *testing.T) {
	// Create a temporary base directory to simulate the environment
	tempDir := t.TempDir()

	// Simulate the three agent directories
	userAgentsDir := filepath.Join(tempDir, "home", ".claude", "agents")
	projectClaudeAgentsDir := filepath.Join(tempDir, "project", ".claude", "agents")
	projectAgentsDir := filepath.Join(tempDir, "project", "agents")

	// Create unique agents in each directory
	createAgentFile(t, userAgentsDir, "user-only-agent", "An agent only in user global directory")
	createAgentFile(t, projectClaudeAgentsDir, "project-claude-agent", "An agent only in project .claude directory")
	createAgentFile(t, projectAgentsDir, "project-agent", "An agent only in project root directory")

	// Create the agent manager with multiple directories
	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
			{Path: projectClaudeAgentsDir, SourceType: entity.SubagentSourceProjectClaude},
			{Path: userAgentsDir, SourceType: entity.SubagentSourceUser},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	result, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	// FAILING ASSERTION: We expect 3 agents from 3 directories
	if result.TotalCount != 3 {
		t.Errorf("DiscoverAgents() TotalCount = %d, want 3 (agents from all three directories)", result.TotalCount)
	}

	// Verify all three agents are discovered
	agentNames := make(map[string]bool)
	for _, agent := range result.Subagents {
		agentNames[agent.Name] = true
	}

	expectedAgents := []string{"user-only-agent", "project-claude-agent", "project-agent"}
	for _, expected := range expectedAgents {
		if !agentNames[expected] {
			t.Errorf("DiscoverAgents() missing expected agent %q", expected)
		}
	}

	// FAILING ASSERTION: AgentsDirs should list all searched directories
	if len(result.AgentsDirs) != 3 {
		t.Errorf("DiscoverAgents() AgentsDirs length = %d, want 3", len(result.AgentsDirs))
	}
}

// TestLocalSubagentManager_DiscoverAgents_PriorityOverride verifies that when the same
// agent name exists in multiple directories, the highest priority directory wins.
// Priority: ./agents > ./.claude/agents > ~/.claude/agents.
func TestLocalSubagentManager_DiscoverAgents_PriorityOverride(t *testing.T) {
	tempDir := t.TempDir()

	// Simulate the three agent directories
	userAgentsDir := filepath.Join(tempDir, "home", ".claude", "agents")
	projectClaudeAgentsDir := filepath.Join(tempDir, "project", ".claude", "agents")
	projectAgentsDir := filepath.Join(tempDir, "project", "agents")

	// Create the SAME agent in all three directories with different descriptions
	createAgentFile(t, userAgentsDir, "common-agent", "User global version of common-agent")
	createAgentFile(t, projectClaudeAgentsDir, "common-agent", "Project .claude version of common-agent")
	createAgentFile(t, projectAgentsDir, "common-agent", "Project root version of common-agent (highest priority)")

	// Also create an agent that only exists in user and project-claude (to test mid-priority override)
	createAgentFile(t, userAgentsDir, "mid-priority-agent", "User global version")
	createAgentFile(t, projectClaudeAgentsDir, "mid-priority-agent", "Project .claude version (should win)")

	// Create agent manager
	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
			{Path: projectClaudeAgentsDir, SourceType: entity.SubagentSourceProjectClaude},
			{Path: userAgentsDir, SourceType: entity.SubagentSourceUser},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	result, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	// Find the common-agent in results
	var commonAgent *struct {
		description string
		sourceType  entity.SubagentSourceType
	}
	var midPriorityAgent *struct {
		description string
		sourceType  entity.SubagentSourceType
	}

	for _, agent := range result.Subagents {
		if agent.Name == "common-agent" {
			commonAgent = &struct {
				description string
				sourceType  entity.SubagentSourceType
			}{agent.Description, agent.SourceType}
		}
		if agent.Name == "mid-priority-agent" {
			midPriorityAgent = &struct {
				description string
				sourceType  entity.SubagentSourceType
			}{agent.Description, agent.SourceType}
		}
	}

	// FAILING ASSERTION: common-agent should have description from highest priority (./agents)
	if commonAgent == nil {
		t.Fatal("DiscoverAgents() did not find common-agent")
	}
	if commonAgent.description != "Project root version of common-agent (highest priority)" {
		t.Errorf("DiscoverAgents() common-agent.Description = %q, want %q (highest priority should win)",
			commonAgent.description, "Project root version of common-agent (highest priority)")
	}
	if commonAgent.sourceType != entity.SubagentSourceProject {
		t.Errorf("DiscoverAgents() common-agent.SourceType = %q, want %q",
			commonAgent.sourceType, entity.SubagentSourceProject)
	}

	// FAILING ASSERTION: mid-priority-agent should exist and come from project-claude
	if midPriorityAgent == nil {
		t.Errorf("DiscoverAgents() did not find mid-priority-agent (should be discovered from project-claude or user)")
	} else {
		if midPriorityAgent.description != "Project .claude version (should win)" {
			t.Errorf("DiscoverAgents() mid-priority-agent.Description = %q, want %q",
				midPriorityAgent.description, "Project .claude version (should win)")
		}
		if midPriorityAgent.sourceType != entity.SubagentSourceProjectClaude {
			t.Errorf("DiscoverAgents() mid-priority-agent.SourceType = %q, want %q",
				midPriorityAgent.sourceType, entity.SubagentSourceProjectClaude)
		}
	}

	// We should have exactly 2 unique agents after priority resolution
	if result.TotalCount != 2 {
		t.Errorf("DiscoverAgents() TotalCount = %d, want 2 (after priority deduplication)", result.TotalCount)
	}
}

// TestLocalSubagentManager_DiscoverAgents_MissingDirectories verifies that discovery
// gracefully handles missing directories without erroring.
func TestLocalSubagentManager_DiscoverAgents_MissingDirectories(t *testing.T) {
	tempDir := t.TempDir()

	// Only create the project agents directory, leave others missing
	projectAgentsDir := filepath.Join(tempDir, "project", "agents")
	userAgentsDir := filepath.Join(tempDir, "home", ".claude", "agents")             // Does not exist
	projectClaudeAgentsDir := filepath.Join(tempDir, "project", ".claude", "agents") // Does not exist

	createAgentFile(t, projectAgentsDir, "existing-agent", "An agent in the existing directory")

	// Create agent manager that should handle missing directories gracefully
	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
			{Path: projectClaudeAgentsDir, SourceType: entity.SubagentSourceProjectClaude},
			{Path: userAgentsDir, SourceType: entity.SubagentSourceUser},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	result, err := sm.DiscoverAgents(context.Background())
	// Should NOT error when some directories are missing
	if err != nil {
		t.Fatalf("DiscoverAgents() should not error when some directories are missing: %v", err)
	}

	// Should still discover agents from existing directories
	if result.TotalCount != 1 {
		t.Errorf("DiscoverAgents() TotalCount = %d, want 1", result.TotalCount)
	}

	// FAILING ASSERTION: AgentsDirs should list all directories that were searched,
	// including those that don't exist (for transparency)
	if len(result.AgentsDirs) < 1 {
		t.Errorf("DiscoverAgents() AgentsDirs should list searched directories, got %d", len(result.AgentsDirs))
	}
}

// TestLocalSubagentManager_DiscoverAgents_SourceType verifies that each discovered agent
// has the correct SourceType field set based on which directory it was found in.
func TestLocalSubagentManager_DiscoverAgents_SourceType(t *testing.T) {
	tempDir := t.TempDir()

	// Create agent directories
	userAgentsDir := filepath.Join(tempDir, "home", ".claude", "agents")
	projectClaudeAgentsDir := filepath.Join(tempDir, "project", ".claude", "agents")
	projectAgentsDir := filepath.Join(tempDir, "project", "agents")

	// Create unique agents in each directory
	createAgentFile(t, userAgentsDir, "user-agent", "User global agent")
	createAgentFile(t, projectClaudeAgentsDir, "claude-agent", "Project .claude agent")
	createAgentFile(t, projectAgentsDir, "root-agent", "Project root agent")

	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
			{Path: projectClaudeAgentsDir, SourceType: entity.SubagentSourceProjectClaude},
			{Path: userAgentsDir, SourceType: entity.SubagentSourceUser},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	result, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	// Build a map of agent name to source type
	sourceTypes := make(map[string]entity.SubagentSourceType)
	for _, agent := range result.Subagents {
		sourceTypes[agent.Name] = agent.SourceType
	}

	// Test cases for expected source types
	tests := []struct {
		agentName      string
		wantSourceType entity.SubagentSourceType
	}{
		{"user-agent", entity.SubagentSourceUser},
		{"claude-agent", entity.SubagentSourceProjectClaude},
		{"root-agent", entity.SubagentSourceProject},
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			gotSourceType, found := sourceTypes[tt.agentName]

			// FAILING ASSERTION: Agent may not be found because multi-dir discovery is not implemented
			if !found {
				t.Errorf("DiscoverAgents() did not find agent %q", tt.agentName)
				return
			}

			// FAILING ASSERTION: SourceType is not set in current implementation
			if gotSourceType != tt.wantSourceType {
				t.Errorf("DiscoverAgents() %s.SourceType = %q, want %q",
					tt.agentName, gotSourceType, tt.wantSourceType)
			}
		})
	}
}

// =============================================================================
// LoadAgentMetadata Tests
// =============================================================================

func TestLocalSubagentManager_LoadAgentMetadata_ExistingAgent(t *testing.T) {
	// Create a temporary agents directory
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	testAgentDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(testAgentDir, 0o750); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create an AGENT.md file with full content
	agentContent := `---
name: test-agent
description: A test agent
model: sonnet
max_actions: 10
allowed-tools: read_file write_file
---
System prompt content here`
	if err := os.WriteFile(filepath.Join(testAgentDir, "AGENT.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatalf("Failed to write AGENT.md: %v", err)
	}

	// Create agent manager and discover agents
	sm := &LocalSubagentManager{
		agentsDirs:   []DirConfig{{Path: agentsDir, SourceType: entity.SubagentSourceProject}},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	_, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover agents: %v", err)
	}

	// Load agent metadata
	agent, err := sm.LoadAgentMetadata(context.Background(), "test-agent")
	if err != nil {
		t.Fatalf("LoadAgentMetadata() returned unexpected error: %v", err)
	}

	if agent == nil {
		t.Fatal("LoadAgentMetadata() returned nil agent")
	}

	if agent.Name != "test-agent" {
		t.Errorf("LoadAgentMetadata() Name = %v, want 'test-agent'", agent.Name)
	}

	if agent.Description != "A test agent" {
		t.Errorf("LoadAgentMetadata() Description = %v, want 'A test agent'", agent.Description)
	}

	if agent.RawContent == "" {
		t.Error("LoadAgentMetadata() RawContent should be populated with full content")
	}
}

func TestLocalSubagentManager_LoadAgentMetadata_NonExistent(t *testing.T) {
	sm := NewLocalSubagentManager()
	_, err := sm.LoadAgentMetadata(context.Background(), "nonexistent-agent")

	if err == nil {
		t.Error("LoadAgentMetadata() should return error for nonexistent agent")
	}

	if !errors.Is(err, ErrAgentFileNotFound) {
		t.Errorf("LoadAgentMetadata() error = %v, want ErrAgentFileNotFound", err)
	}
}

func TestLocalSubagentManager_LoadAgentMetadata_PathTraversalPrevention(t *testing.T) {
	sm := NewLocalSubagentManager()

	pathTraversalAttempts := []struct {
		name      string
		agentName string
	}{
		{name: "parent directory", agentName: "../etc/passwd"},
		{name: "double parent", agentName: "../../secret"},
		{name: "absolute path", agentName: "/etc/passwd"},
		{name: "encoded slash", agentName: "agent%2F..%2Fetc"},
		{name: "dot dot", agentName: ".."},
		{name: "hidden file", agentName: ".hidden"},
	}

	for _, tt := range pathTraversalAttempts {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sm.LoadAgentMetadata(context.Background(), tt.agentName)
			if err == nil {
				t.Errorf("LoadAgentMetadata(%q) should have blocked path traversal attempt", tt.agentName)
			}
			// The error should be related to invalid agent name, not file not found
			if errors.Is(err, ErrAgentFileNotFound) {
				t.Errorf(
					"LoadAgentMetadata(%q) returned file not found instead of blocking path traversal",
					tt.agentName,
				)
			}
		})
	}
}

func TestLocalSubagentManager_LoadAgentMetadata_FromCorrectDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create agent directories
	userAgentsDir := filepath.Join(tempDir, "home", ".claude", "agents")
	projectClaudeAgentsDir := filepath.Join(tempDir, "project", ".claude", "agents")
	projectAgentsDir := filepath.Join(tempDir, "project", "agents")

	// Create the same agent in user and project-claude directories
	// Project-claude should win (higher priority)
	createAgentFile(t, userAgentsDir, "shared-agent", "User version - should be overridden")
	createAgentFile(t, projectClaudeAgentsDir, "shared-agent", "Project .claude version - should win")

	// Create a user-only agent
	createAgentFile(t, userAgentsDir, "user-exclusive", "Only in user directory")

	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
			{Path: projectClaudeAgentsDir, SourceType: entity.SubagentSourceProjectClaude},
			{Path: userAgentsDir, SourceType: entity.SubagentSourceUser},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	// First discover agents
	_, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	// Test loading shared-agent - should get project-claude version
	t.Run("shared-agent loads from project-claude", func(t *testing.T) {
		agent, err := sm.LoadAgentMetadata(context.Background(), "shared-agent")
		// FAILING ASSERTION: Agent not found because multi-dir discovery not implemented
		if err != nil {
			t.Fatalf("LoadAgentMetadata(shared-agent) returned error: %v", err)
		}

		// FAILING ASSERTION: Description should be from project-claude (higher priority)
		if agent.Description != "Project .claude version - should win" {
			t.Errorf("LoadAgentMetadata(shared-agent).Description = %q, want %q",
				agent.Description, "Project .claude version - should win")
		}

		// FAILING ASSERTION: SourceType should indicate where it was loaded from
		if agent.SourceType != entity.SubagentSourceProjectClaude {
			t.Errorf("LoadAgentMetadata(shared-agent).SourceType = %q, want %q",
				agent.SourceType, entity.SubagentSourceProjectClaude)
		}
	})

	// Test loading user-exclusive - should come from user directory
	t.Run("user-exclusive loads from user directory", func(t *testing.T) {
		agent, err := sm.LoadAgentMetadata(context.Background(), "user-exclusive")
		// FAILING ASSERTION: Agent not found because multi-dir discovery not implemented
		if err != nil {
			t.Fatalf("LoadAgentMetadata(user-exclusive) returned error: %v", err)
		}

		if agent.Description != "Only in user directory" {
			t.Errorf("LoadAgentMetadata(user-exclusive).Description = %q, want %q",
				agent.Description, "Only in user directory")
		}

		// FAILING ASSERTION: SourceType should indicate user source
		if agent.SourceType != entity.SubagentSourceUser {
			t.Errorf("LoadAgentMetadata(user-exclusive).SourceType = %q, want %q",
				agent.SourceType, entity.SubagentSourceUser)
		}
	})
}

// =============================================================================
// Programmatic Registration Tests
// =============================================================================

func TestLocalSubagentManager_RegisterAgent_Valid(t *testing.T) {
	sm := NewLocalSubagentManager()

	agent := &entity.Subagent{
		Name:        "test-agent",
		Description: "A programmatic test agent",
		Model:       "sonnet",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	err := sm.RegisterAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("RegisterAgent() returned unexpected error: %v", err)
	}

	// Verify agent is registered
	info, err := sm.GetAgentByName(context.Background(), "test-agent")
	if err != nil {
		t.Fatalf("GetAgentByName() returned unexpected error: %v", err)
	}

	if info.Name != "test-agent" {
		t.Errorf("GetAgentByName() Name = %v, want 'test-agent'", info.Name)
	}

	if info.SourceType != entity.SubagentSourceProgrammatic {
		t.Errorf("GetAgentByName() SourceType = %v, want 'programmatic'", info.SourceType)
	}
}

func TestLocalSubagentManager_RegisterAgent_Duplicate(t *testing.T) {
	sm := NewLocalSubagentManager()

	agent := &entity.Subagent{
		Name:        "duplicate-agent",
		Description: "First registration",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	// First registration should succeed
	err := sm.RegisterAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("RegisterAgent() first call returned unexpected error: %v", err)
	}

	// Second registration with same name should fail
	duplicateAgent := &entity.Subagent{
		Name:        "duplicate-agent",
		Description: "Second registration",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	err = sm.RegisterAgent(context.Background(), duplicateAgent)
	if err == nil {
		t.Error("RegisterAgent() should return error for duplicate registration")
	}

	if !errors.Is(err, ErrAgentAlreadyRegistered) {
		t.Errorf("RegisterAgent() error = %v, want ErrAgentAlreadyRegistered", err)
	}
}

func TestLocalSubagentManager_RegisterAgent_NilAgent(t *testing.T) {
	sm := NewLocalSubagentManager()

	err := sm.RegisterAgent(context.Background(), nil)
	if err == nil {
		t.Error("RegisterAgent() should return error for nil agent")
	}

	if !errors.Is(err, ErrInvalidAgent) {
		t.Errorf("RegisterAgent() error = %v, want ErrInvalidAgent", err)
	}
}

func TestLocalSubagentManager_RegisterAgent_InvalidAgent(t *testing.T) {
	sm := NewLocalSubagentManager()

	invalidAgent := &entity.Subagent{
		Name:        "", // Invalid: empty name
		Description: "Invalid agent",
	}

	err := sm.RegisterAgent(context.Background(), invalidAgent)
	if err == nil {
		t.Error("RegisterAgent() should return error for invalid agent")
	}
}

// =============================================================================
// Unregister Tests
// =============================================================================

func TestLocalSubagentManager_UnregisterAgent_Existing(t *testing.T) {
	sm := NewLocalSubagentManager()

	// Register an agent first
	agent := &entity.Subagent{
		Name:        "temp-agent",
		Description: "Temporary agent",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	err := sm.RegisterAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("RegisterAgent() returned unexpected error: %v", err)
	}

	// Unregister the agent
	err = sm.UnregisterAgent(context.Background(), "temp-agent")
	if err != nil {
		t.Fatalf("UnregisterAgent() returned unexpected error: %v", err)
	}

	// Verify agent is unregistered
	_, err = sm.GetAgentByName(context.Background(), "temp-agent")
	if err == nil {
		t.Error("GetAgentByName() should return error after unregistration")
	}

	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("GetAgentByName() error = %v, want ErrAgentNotFound", err)
	}
}

func TestLocalSubagentManager_UnregisterAgent_NonExistent(t *testing.T) {
	sm := NewLocalSubagentManager()

	err := sm.UnregisterAgent(context.Background(), "nonexistent-agent")
	if err == nil {
		t.Error("UnregisterAgent() should return error for nonexistent agent")
	}

	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("UnregisterAgent() error = %v, want ErrAgentNotFound", err)
	}
}

// =============================================================================
// GetAgentByName Tests
// =============================================================================

func TestLocalSubagentManager_GetAgentByName(t *testing.T) {
	// Create a temporary agents directory
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	testAgentDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(testAgentDir, 0o750); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create an AGENT.md file
	agentContent := `---
name: test-agent
description: A test agent
model: haiku
---
Test content`
	if err := os.WriteFile(filepath.Join(testAgentDir, "AGENT.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatalf("Failed to write AGENT.md: %v", err)
	}

	// Create agent manager and discover agents
	sm := &LocalSubagentManager{
		agentsDirs:   []DirConfig{{Path: agentsDir, SourceType: entity.SubagentSourceProject}},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	_, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover agents: %v", err)
	}

	// Get agent by name
	agentInfo, err := sm.GetAgentByName(context.Background(), "test-agent")
	if err != nil {
		t.Fatalf("GetAgentByName() returned unexpected error: %v", err)
	}

	if agentInfo == nil {
		t.Fatal("GetAgentByName() returned nil agent")
	}

	if agentInfo.Name != "test-agent" {
		t.Errorf("GetAgentByName() Name = %v, want 'test-agent'", agentInfo.Name)
	}

	if agentInfo.Description != "A test agent" {
		t.Errorf("GetAgentByName() Description = %v, want 'A test agent'", agentInfo.Description)
	}
}

func TestLocalSubagentManager_GetAgentByName_NotFound(t *testing.T) {
	sm := NewLocalSubagentManager()
	_, err := sm.GetAgentByName(context.Background(), "nonexistent-agent")

	if err == nil {
		t.Error("GetAgentByName() should return error for nonexistent agent")
	}

	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("GetAgentByName() error = %v, want ErrAgentNotFound", err)
	}
}

func TestLocalSubagentManager_GetAgentByName_ReturnsSourceType(t *testing.T) {
	tempDir := t.TempDir()

	projectAgentsDir := filepath.Join(tempDir, "project", "agents")
	createAgentFile(t, projectAgentsDir, "test-agent", "A test agent")

	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	_, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	agentInfo, err := sm.GetAgentByName(context.Background(), "test-agent")
	if err != nil {
		t.Fatalf("GetAgentByName() returned unexpected error: %v", err)
	}

	// FAILING ASSERTION: SourceType should be set to "project" for agents from ./agents
	if agentInfo.SourceType != entity.SubagentSourceProject {
		t.Errorf("GetAgentByName().SourceType = %q, want %q",
			agentInfo.SourceType, entity.SubagentSourceProject)
	}
}

// =============================================================================
// ListAgents Tests
// =============================================================================

func TestLocalSubagentManager_ListAgents(t *testing.T) {
	// Create a temporary agents directory
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")

	// Create two agent directories with explicit names
	for _, agentName := range []string{"agent-a", "agent-b"} {
		testAgentDir := filepath.Join(agentsDir, agentName)
		if err := os.MkdirAll(testAgentDir, 0o750); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		agentContent := `---
name: ` + agentName + `
description: Test agent
---
Content`
		if err := os.WriteFile(filepath.Join(testAgentDir, "AGENT.md"), []byte(agentContent), 0o644); err != nil {
			t.Fatalf("Failed to write AGENT.md: %v", err)
		}
	}

	// Create agent manager and discover agents
	sm := &LocalSubagentManager{
		agentsDirs:   []DirConfig{{Path: agentsDir, SourceType: entity.SubagentSourceProject}},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	_, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover agents: %v", err)
	}

	// List all agents
	agents, err := sm.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents() returned unexpected error: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("ListAgents() returned %d agents, want 2", len(agents))
	}

	// Check that both agents are in the list
	agentNames := make(map[string]bool)
	for _, agent := range agents {
		agentNames[agent.Name] = true
	}

	if !agentNames["agent-a"] {
		t.Error("ListAgents() missing 'agent-a'")
	}
	if !agentNames["agent-b"] {
		t.Error("ListAgents() missing 'agent-b'")
	}
}

func TestLocalSubagentManager_ListAgents_Empty(t *testing.T) {
	sm := NewLocalSubagentManager()

	agents, err := sm.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents() returned unexpected error: %v", err)
	}

	if len(agents) != 0 {
		t.Errorf("ListAgents() returned %d agents, want 0 for empty manager", len(agents))
	}
}

func TestLocalSubagentManager_ListAgents_IncludesProgrammatic(t *testing.T) {
	sm := NewLocalSubagentManager()

	// Register a programmatic agent
	agent := &entity.Subagent{
		Name:        "programmatic-agent",
		Description: "A programmatic agent",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	err := sm.RegisterAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("RegisterAgent() returned unexpected error: %v", err)
	}

	// List agents should include programmatic agent
	agents, err := sm.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents() returned unexpected error: %v", err)
	}

	if len(agents) != 1 {
		t.Errorf("ListAgents() returned %d agents, want 1", len(agents))
	}

	if agents[0].Name != "programmatic-agent" {
		t.Errorf("ListAgents()[0].Name = %v, want 'programmatic-agent'", agents[0].Name)
	}

	if agents[0].SourceType != entity.SubagentSourceProgrammatic {
		t.Errorf("ListAgents()[0].SourceType = %v, want 'programmatic'", agents[0].SourceType)
	}
}

// =============================================================================
// Thread-Safety Tests
// =============================================================================

func TestLocalSubagentManager_ConcurrentDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")

	// Create multiple agents
	for i := range 5 {
		agentName := "concurrent-agent-" + string(rune('a'+i))
		createAgentFile(t, agentsDir, agentName, "Concurrent test agent")
	}

	sm := &LocalSubagentManager{
		agentsDirs:   []DirConfig{{Path: agentsDir, SourceType: entity.SubagentSourceProject}},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	// Run concurrent discoveries
	var wg sync.WaitGroup
	numGoroutines := 10

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := sm.DiscoverAgents(context.Background())
			if err != nil {
				t.Errorf("DiscoverAgents() returned error in concurrent execution: %v", err)
			}
		}()
	}

	wg.Wait()

	// Verify final state is consistent
	result, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("Final DiscoverAgents() returned error: %v", err)
	}

	if result.TotalCount != 5 {
		t.Errorf("After concurrent discovery, TotalCount = %d, want 5", result.TotalCount)
	}
}

func TestLocalSubagentManager_ConcurrentRegistration(t *testing.T) {
	sm := NewLocalSubagentManager()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Register different agents concurrently
	for i := range numGoroutines {
		wg.Add(1)
		agentNum := i
		go func() {
			defer wg.Done()
			agent := &entity.Subagent{
				Name:        "concurrent-reg-agent-" + string(rune('a'+agentNum)),
				Description: "Concurrent registration test",
				SourceType:  entity.SubagentSourceProgrammatic,
			}
			err := sm.RegisterAgent(context.Background(), agent)
			if err != nil {
				t.Errorf("RegisterAgent() returned error in concurrent execution: %v", err)
			}
		}()
	}

	wg.Wait()

	// Verify all agents were registered
	agents, err := sm.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents() returned error: %v", err)
	}

	if len(agents) != numGoroutines {
		t.Errorf("After concurrent registration, ListAgents() returned %d agents, want %d",
			len(agents), numGoroutines)
	}
}

// =============================================================================
// Additional Edge Case Tests
// =============================================================================

func TestLocalSubagentManager_DiscoverAgents_AllDirectoriesMissing(t *testing.T) {
	tempDir := t.TempDir()

	// None of these directories exist
	userAgentsDir := filepath.Join(tempDir, "nonexistent", "home", ".claude", "agents")
	projectClaudeAgentsDir := filepath.Join(tempDir, "nonexistent", "project", ".claude", "agents")
	projectAgentsDir := filepath.Join(tempDir, "nonexistent", "project", "agents")

	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
			{Path: projectClaudeAgentsDir, SourceType: entity.SubagentSourceProjectClaude},
			{Path: userAgentsDir, SourceType: entity.SubagentSourceUser},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	result, err := sm.DiscoverAgents(context.Background())
	// Should NOT error - missing directories are acceptable
	if err != nil {
		t.Fatalf("DiscoverAgents() should not error when all directories are missing: %v", err)
	}

	// Should return empty result
	if result.TotalCount != 0 {
		t.Errorf("DiscoverAgents() TotalCount = %d, want 0", result.TotalCount)
	}

	if len(result.Subagents) != 0 {
		t.Errorf("DiscoverAgents() returned %d agents, want 0", len(result.Subagents))
	}
}

func TestLocalSubagentManager_DiscoverAgents_DirectoryOrderInResult(t *testing.T) {
	tempDir := t.TempDir()

	projectAgentsDir := filepath.Join(tempDir, "project", "agents")
	projectClaudeAgentsDir := filepath.Join(tempDir, "project", ".claude", "agents")
	userAgentsDir := filepath.Join(tempDir, "home", ".claude", "agents")

	// Create all directories with agents
	createAgentFile(t, projectAgentsDir, "agent-a", "Agent A")
	createAgentFile(t, projectClaudeAgentsDir, "agent-b", "Agent B")
	createAgentFile(t, userAgentsDir, "agent-c", "Agent C")

	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
			{Path: projectClaudeAgentsDir, SourceType: entity.SubagentSourceProjectClaude},
			{Path: userAgentsDir, SourceType: entity.SubagentSourceUser},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	result, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	// FAILING ASSERTION: AgentsDirs should be populated with all directories
	if len(result.AgentsDirs) != 3 {
		t.Fatalf("DiscoverAgents() AgentsDirs length = %d, want 3", len(result.AgentsDirs))
	}

	// FAILING ASSERTION: Directories should be in priority order (highest first)
	// Expected order: ./agents, ./.claude/agents, ~/.claude/agents
	expectedOrder := []string{projectAgentsDir, projectClaudeAgentsDir, userAgentsDir}

	// Make copies to sort for comparison since we care about order
	gotDirs := make([]string, len(result.AgentsDirs))
	copy(gotDirs, result.AgentsDirs)

	for i, expected := range expectedOrder {
		if i >= len(gotDirs) {
			t.Errorf("DiscoverAgents() AgentsDirs[%d] missing, want %q", i, expected)
			continue
		}
		// Use filepath.Clean for comparison to handle path normalization
		if filepath.Clean(gotDirs[i]) != filepath.Clean(expected) {
			t.Errorf("DiscoverAgents() AgentsDirs[%d] = %q, want %q (priority order)", i, gotDirs[i], expected)
		}
	}
}

func TestLocalSubagentManager_DiscoverAgents_AgentsListedByPriority(t *testing.T) {
	tempDir := t.TempDir()

	userAgentsDir := filepath.Join(tempDir, "home", ".claude", "agents")
	projectClaudeAgentsDir := filepath.Join(tempDir, "project", ".claude", "agents")
	projectAgentsDir := filepath.Join(tempDir, "project", "agents")

	// Create agents in reverse priority order to test sorting
	createAgentFile(t, userAgentsDir, "aaa-agent", "User agent (lowest priority)")
	createAgentFile(t, projectClaudeAgentsDir, "bbb-agent", "Project .claude agent")
	createAgentFile(t, projectAgentsDir, "ccc-agent", "Project agent (highest priority)")

	sm := &LocalSubagentManager{
		agentsDirs: []DirConfig{
			{Path: projectAgentsDir, SourceType: entity.SubagentSourceProject},
			{Path: projectClaudeAgentsDir, SourceType: entity.SubagentSourceProjectClaude},
			{Path: userAgentsDir, SourceType: entity.SubagentSourceUser},
		},
		agents:       make(map[string]*entity.Subagent),
		programmatic: make(map[string]*entity.Subagent),
	}

	result, err := sm.DiscoverAgents(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAgents() returned unexpected error: %v", err)
	}

	// FAILING ASSERTION: Should find all 3 agents
	if result.TotalCount != 3 {
		t.Fatalf("DiscoverAgents() TotalCount = %d, want 3", result.TotalCount)
	}

	// Get source types in order
	sourceTypePriority := map[entity.SubagentSourceType]int{
		entity.SubagentSourceProject:       0, // Highest priority
		entity.SubagentSourceProjectClaude: 1,
		entity.SubagentSourceUser:          2, // Lowest priority
	}

	// Sort by source type priority
	sortedAgents := make([]struct {
		name       string
		sourceType entity.SubagentSourceType
	}, len(result.Subagents))

	for i, s := range result.Subagents {
		sortedAgents[i] = struct {
			name       string
			sourceType entity.SubagentSourceType
		}{s.Name, s.SourceType}
	}

	sort.Slice(sortedAgents, func(i, j int) bool {
		return sourceTypePriority[sortedAgents[i].sourceType] < sourceTypePriority[sortedAgents[j].sourceType]
	})

	// Verify we have agents from each source type
	foundSourceTypes := make(map[entity.SubagentSourceType]bool)
	for _, s := range sortedAgents {
		foundSourceTypes[s.sourceType] = true
	}

	if !foundSourceTypes[entity.SubagentSourceProject] {
		t.Error("DiscoverAgents() missing agents from project directory")
	}
	if !foundSourceTypes[entity.SubagentSourceProjectClaude] {
		t.Error("DiscoverAgents() missing agents from project-claude directory")
	}
	if !foundSourceTypes[entity.SubagentSourceUser] {
		t.Error("DiscoverAgents() missing agents from user directory")
	}
}
