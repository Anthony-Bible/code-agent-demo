package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"testing"
)

// TestSubagentManagerInterface_Contract validates that SubagentManager interface exists with expected methods.
func TestSubagentManagerInterface_Contract(_ *testing.T) {
	// Verify that SubagentManager interface exists
	var _ SubagentManager = (*mockSubagentManager)(nil)
}

// TestSubagentManagerInterface_DiscoverAgents validates DiscoverAgents method exists.
func TestSubagentManagerInterface_DiscoverAgents(_ *testing.T) {
	var manager SubagentManager = (*mockSubagentManager)(nil)

	// This will fail to compile if DiscoverAgents method doesn't exist with correct signature
	_ = manager.DiscoverAgents
}

// TestSubagentManagerInterface_LoadAgentMetadata validates LoadAgentMetadata method exists.
func TestSubagentManagerInterface_LoadAgentMetadata(_ *testing.T) {
	var manager SubagentManager = (*mockSubagentManager)(nil)

	// This will fail to compile if LoadAgentMetadata method doesn't exist with correct signature
	_ = manager.LoadAgentMetadata
}

// TestSubagentManagerInterface_RegisterAgent validates RegisterAgent method exists.
func TestSubagentManagerInterface_RegisterAgent(_ *testing.T) {
	var manager SubagentManager = (*mockSubagentManager)(nil)

	// This will fail to compile if RegisterAgent method doesn't exist with correct signature
	_ = manager.RegisterAgent
}

// TestSubagentManagerInterface_UnregisterAgent validates UnregisterAgent method exists.
func TestSubagentManagerInterface_UnregisterAgent(_ *testing.T) {
	var manager SubagentManager = (*mockSubagentManager)(nil)

	// This will fail to compile if UnregisterAgent method doesn't exist with correct signature
	_ = manager.UnregisterAgent
}

// TestSubagentManagerInterface_GetAgentByName validates GetAgentByName method exists.
func TestSubagentManagerInterface_GetAgentByName(_ *testing.T) {
	var manager SubagentManager = (*mockSubagentManager)(nil)

	// This will fail to compile if GetAgentByName method doesn't exist with correct signature
	_ = manager.GetAgentByName
}

// TestSubagentManagerInterface_ListAgents validates ListAgents method exists.
func TestSubagentManagerInterface_ListAgents(_ *testing.T) {
	var manager SubagentManager = (*mockSubagentManager)(nil)

	// This will fail to compile if ListAgents method doesn't exist with correct signature
	_ = manager.ListAgents
}

// TestSubagentManagerInterface_SubagentInfoStructure validates SubagentInfo struct has expected fields.
func TestSubagentManagerInterface_SubagentInfoStructure(_ *testing.T) {
	info := SubagentInfo{
		Name:          "test-agent",
		Description:   "A test subagent",
		AllowedTools:  []string{"bash", "read_file"},
		Model:         entity.ModelSonnet,
		SourceType:    entity.SubagentSourceProject,
		DirectoryPath: "/path/to/agent",
	}

	// Verify all fields can be accessed
	_ = info.Name
	_ = info.Description
	_ = info.AllowedTools
	_ = info.Model
	_ = info.SourceType
	_ = info.DirectoryPath
}

// TestSubagentManagerInterface_SubagentDiscoveryResultStructure validates SubagentDiscoveryResult struct.
func TestSubagentManagerInterface_SubagentDiscoveryResultStructure(_ *testing.T) {
	result := SubagentDiscoveryResult{
		Subagents: []SubagentInfo{
			{
				Name:        "agent1",
				Description: "First agent",
			},
		},
		AgentsDirs: []string{"/path/one", "/path/two"},
		TotalCount: 1,
	}

	// Verify all fields can be accessed
	_ = result.Subagents
	_ = result.AgentsDirs
	_ = result.TotalCount
}

// mockSubagentManager is a minimal implementation to validate interface contract.
type mockSubagentManager struct{}

func (m *mockSubagentManager) DiscoverAgents(_ context.Context) (*SubagentDiscoveryResult, error) {
	return &SubagentDiscoveryResult{}, nil
}

func (m *mockSubagentManager) LoadAgentMetadata(_ context.Context, _ string) (*entity.Subagent, error) {
	return &entity.Subagent{}, nil
}

func (m *mockSubagentManager) RegisterAgent(_ context.Context, _ *entity.Subagent) error {
	return nil
}

func (m *mockSubagentManager) UnregisterAgent(_ context.Context, _ string) error {
	return nil
}

func (m *mockSubagentManager) GetAgentByName(_ context.Context, _ string) (*SubagentInfo, error) {
	return &SubagentInfo{}, nil
}

func (m *mockSubagentManager) ListAgents(_ context.Context) ([]SubagentInfo, error) {
	return []SubagentInfo{}, nil
}

// TestSubagentManager_DiscoverAgents_ReturnsValidResult tests that DiscoverAgents returns valid discovery results.
func TestSubagentManager_DiscoverAgents_ReturnsValidResult(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	result, err := manager.DiscoverAgents(ctx)
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("DiscoverAgents() result = nil, want non-nil")
	}

	if result.Subagents == nil {
		t.Error("DiscoverAgents() result.Subagents = nil, want non-nil slice")
	}

	if result.AgentsDirs == nil {
		t.Error("DiscoverAgents() result.AgentsDirs = nil, want non-nil slice")
	}

	if result.TotalCount < 0 {
		t.Errorf("DiscoverAgents() result.TotalCount = %d, want >= 0", result.TotalCount)
	}

	// TotalCount should match number of subagents
	if result.TotalCount != len(result.Subagents) {
		t.Errorf(
			"DiscoverAgents() result.TotalCount = %d, want %d (len of Subagents)",
			result.TotalCount,
			len(result.Subagents),
		)
	}
}

// TestSubagentManager_DiscoverAgents_SearchesMultipleDirectories tests that discovery searches all expected directories.
func TestSubagentManager_DiscoverAgents_SearchesMultipleDirectories(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	result, err := manager.DiscoverAgents(ctx)
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v, want nil", err)
	}

	// Should search at least project root directory
	if len(result.AgentsDirs) == 0 {
		t.Error("DiscoverAgents() result.AgentsDirs is empty, want at least one directory")
	}

	// Expected directories based on spec:
	// 1. ./subagents (project root)
	// 2. ./.claude/subagents (project .claude)
	// 3. ~/.claude/subagents (user global)
	expectedMinDirs := 1 // At minimum should search project root
	if len(result.AgentsDirs) < expectedMinDirs {
		t.Errorf("DiscoverAgents() searched %d directories, want at least %d", len(result.AgentsDirs), expectedMinDirs)
	}
}

// TestSubagentManager_DiscoverAgents_WithContext tests that DiscoverAgents respects context cancellation.
func TestSubagentManager_DiscoverAgents_WithContext(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := manager.DiscoverAgents(ctx)

	// Should return error when context is cancelled
	if err == nil {
		t.Error("DiscoverAgents() with cancelled context error = nil, want context.Canceled or similar")
	}
}

// TestSubagentManager_LoadAgentMetadata_ValidAgent tests loading metadata for a valid subagent.
func TestSubagentManager_LoadAgentMetadata_ValidAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	agent, err := manager.LoadAgentMetadata(ctx, "test-agent")
	if err != nil {
		t.Fatalf("LoadAgentMetadata() error = %v, want nil", err)
	}

	if agent == nil {
		t.Fatal("LoadAgentMetadata() agent = nil, want non-nil")
	}

	if agent.Name == "" {
		t.Error("LoadAgentMetadata() agent.Name is empty, want non-empty")
	}

	if agent.Description == "" {
		t.Error("LoadAgentMetadata() agent.Description is empty, want non-empty")
	}
}

// TestSubagentManager_LoadAgentMetadata_NonExistentAgent tests loading metadata for non-existent subagent.
func TestSubagentManager_LoadAgentMetadata_NonExistentAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	_, err := manager.LoadAgentMetadata(ctx, "non-existent-agent")

	if err == nil {
		t.Error("LoadAgentMetadata() with non-existent agent error = nil, want error")
	}
}

// TestSubagentManager_LoadAgentMetadata_EmptyName tests loading metadata with empty agent name.
func TestSubagentManager_LoadAgentMetadata_EmptyName(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	_, err := manager.LoadAgentMetadata(ctx, "")

	if err == nil {
		t.Error("LoadAgentMetadata() with empty name error = nil, want error")
	}
}

// TestSubagentManager_RegisterAgent_ValidAgent tests registering a valid subagent.
func TestSubagentManager_RegisterAgent_ValidAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	agent := &entity.Subagent{
		Name:        "test-agent",
		Description: "A test subagent for unit tests",
		Model:       "sonnet",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	err := manager.RegisterAgent(ctx, agent)
	if err != nil {
		t.Errorf("RegisterAgent() error = %v, want nil", err)
	}

	// After registration, should be able to retrieve the agent
	info, err := manager.GetAgentByName(ctx, "test-agent")
	if err != nil {
		t.Errorf("GetAgentByName() after registration error = %v, want nil", err)
	}

	if info == nil {
		t.Error("GetAgentByName() after registration info = nil, want non-nil")
	}

	if info != nil && info.Name != "test-agent" {
		t.Errorf("GetAgentByName() after registration info.Name = %s, want 'test-agent'", info.Name)
	}
}

// TestSubagentManager_RegisterAgent_NilAgent tests registering a nil subagent.
func TestSubagentManager_RegisterAgent_NilAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	err := manager.RegisterAgent(ctx, nil)

	if err == nil {
		t.Error("RegisterAgent() with nil agent error = nil, want error")
	}
}

// TestSubagentManager_RegisterAgent_InvalidAgent tests registering an invalid subagent.
func TestSubagentManager_RegisterAgent_InvalidAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	tests := []struct {
		name  string
		agent *entity.Subagent
	}{
		{
			name: "empty name",
			agent: &entity.Subagent{
				Name:        "",
				Description: "Valid description",
			},
		},
		{
			name: "empty description",
			agent: &entity.Subagent{
				Name:        "valid-name",
				Description: "",
			},
		},
		{
			name: "invalid name format",
			agent: &entity.Subagent{
				Name:        "Invalid_Name",
				Description: "Valid description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.RegisterAgent(ctx, tt.agent)
			if err == nil {
				t.Error("RegisterAgent() with invalid agent error = nil, want error")
			}
		})
	}
}

// TestSubagentManager_RegisterAgent_DuplicateAgent tests registering a duplicate subagent.
func TestSubagentManager_RegisterAgent_DuplicateAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	agent := &entity.Subagent{
		Name:        "duplicate-agent",
		Description: "A test subagent",
		Model:       "sonnet",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	// Register once
	err := manager.RegisterAgent(ctx, agent)
	if err != nil {
		t.Fatalf("RegisterAgent() first time error = %v, want nil", err)
	}

	// Try to register again
	err = manager.RegisterAgent(ctx, agent)
	if err == nil {
		t.Error("RegisterAgent() duplicate agent error = nil, want error")
	}
}

// TestSubagentManager_UnregisterAgent_ExistingAgent tests unregistering an existing subagent.
func TestSubagentManager_UnregisterAgent_ExistingAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	// First register an agent
	agent := &entity.Subagent{
		Name:        "agent-to-remove",
		Description: "A test subagent to remove",
		Model:       "sonnet",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	err := manager.RegisterAgent(ctx, agent)
	if err != nil {
		t.Fatalf("RegisterAgent() error = %v, want nil", err)
	}

	// Now unregister it
	err = manager.UnregisterAgent(ctx, "agent-to-remove")
	if err != nil {
		t.Errorf("UnregisterAgent() error = %v, want nil", err)
	}

	// After unregistration, should not be found
	info, err := manager.GetAgentByName(ctx, "agent-to-remove")
	if err == nil {
		t.Error("GetAgentByName() after unregistration error = nil, want error")
	}

	if info != nil {
		t.Errorf("GetAgentByName() after unregistration info = %v, want nil", info)
	}
}

// TestSubagentManager_UnregisterAgent_NonExistentAgent tests unregistering a non-existent subagent.
func TestSubagentManager_UnregisterAgent_NonExistentAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	err := manager.UnregisterAgent(ctx, "non-existent-agent")

	if err == nil {
		t.Error("UnregisterAgent() with non-existent agent error = nil, want error")
	}
}

// TestSubagentManager_UnregisterAgent_EmptyName tests unregistering with empty agent name.
func TestSubagentManager_UnregisterAgent_EmptyName(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	err := manager.UnregisterAgent(ctx, "")

	if err == nil {
		t.Error("UnregisterAgent() with empty name error = nil, want error")
	}
}

// TestSubagentManager_GetAgentByName_ExistingAgent tests getting an existing subagent by name.
func TestSubagentManager_GetAgentByName_ExistingAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	// Register a test agent
	agent := &entity.Subagent{
		Name:         "retrievable-agent",
		Description:  "A test subagent to retrieve",
		Model:        "sonnet",
		AllowedTools: []string{"bash", "read_file"},
		SourceType:   entity.SubagentSourceProgrammatic,
	}

	err := manager.RegisterAgent(ctx, agent)
	if err != nil {
		t.Fatalf("RegisterAgent() error = %v, want nil", err)
	}

	// Retrieve it
	info, err := manager.GetAgentByName(ctx, "retrievable-agent")
	if err != nil {
		t.Fatalf("GetAgentByName() error = %v, want nil", err)
	}

	if info == nil {
		t.Fatal("GetAgentByName() info = nil, want non-nil")
	}

	if info.Name != "retrievable-agent" {
		t.Errorf("GetAgentByName() info.Name = %s, want 'retrievable-agent'", info.Name)
	}

	if info.Description != "A test subagent to retrieve" {
		t.Errorf("GetAgentByName() info.Description = %s, want 'A test subagent to retrieve'", info.Description)
	}

	if len(info.AllowedTools) != 2 {
		t.Errorf("GetAgentByName() len(info.AllowedTools) = %d, want 2", len(info.AllowedTools))
	}
}

// TestSubagentManager_GetAgentByName_NonExistentAgent tests getting a non-existent subagent.
func TestSubagentManager_GetAgentByName_NonExistentAgent(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	info, err := manager.GetAgentByName(ctx, "non-existent-agent")

	if err == nil {
		t.Error("GetAgentByName() with non-existent agent error = nil, want error")
	}

	if info != nil {
		t.Errorf("GetAgentByName() with non-existent agent info = %v, want nil", info)
	}
}

// TestSubagentManager_GetAgentByName_EmptyName tests getting agent with empty name.
func TestSubagentManager_GetAgentByName_EmptyName(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	_, err := manager.GetAgentByName(ctx, "")

	if err == nil {
		t.Error("GetAgentByName() with empty name error = nil, want error")
	}
}

// TestSubagentManager_ListAgents_Empty tests listing agents when none are registered.
func TestSubagentManager_ListAgents_Empty(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	agents, err := manager.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() error = %v, want nil", err)
	}

	if agents == nil {
		t.Fatal("ListAgents() agents = nil, want non-nil empty slice")
	}

	if len(agents) != 0 {
		t.Errorf("ListAgents() len(agents) = %d, want 0", len(agents))
	}
}

// TestSubagentManager_ListAgents_MultipleAgents tests listing multiple registered agents.
func TestSubagentManager_ListAgents_MultipleAgents(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	// Register multiple agents
	agents := []*entity.Subagent{
		{
			Name:        "agent-one",
			Description: "First test agent",
			Model:       "sonnet",
			SourceType:  entity.SubagentSourceProgrammatic,
		},
		{
			Name:        "agent-two",
			Description: "Second test agent",
			Model:       "haiku",
			SourceType:  entity.SubagentSourceProgrammatic,
		},
		{
			Name:        "agent-three",
			Description: "Third test agent",
			Model:       "opus",
			SourceType:  entity.SubagentSourceProgrammatic,
		},
	}

	for _, agent := range agents {
		err := manager.RegisterAgent(ctx, agent)
		if err != nil {
			t.Fatalf("RegisterAgent(%s) error = %v, want nil", agent.Name, err)
		}
	}

	// List all agents
	list, err := manager.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() error = %v, want nil", err)
	}

	if len(list) != 3 {
		t.Errorf("ListAgents() len(list) = %d, want 3", len(list))
	}

	// Verify all registered agents are in the list
	agentNames := make(map[string]bool)
	for _, info := range list {
		agentNames[info.Name] = true
	}

	for _, expectedAgent := range agents {
		if !agentNames[expectedAgent.Name] {
			t.Errorf("ListAgents() missing agent %s", expectedAgent.Name)
		}
	}
}

// TestSubagentManager_ListAgents_AfterUnregister tests listing agents after unregistering one.
func TestSubagentManager_ListAgents_AfterUnregister(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	// Register two agents
	agent1 := &entity.Subagent{
		Name:        "agent-one",
		Description: "First test agent",
		Model:       "sonnet",
		SourceType:  entity.SubagentSourceProgrammatic,
	}
	agent2 := &entity.Subagent{
		Name:        "agent-two",
		Description: "Second test agent",
		Model:       "haiku",
		SourceType:  entity.SubagentSourceProgrammatic,
	}

	_ = manager.RegisterAgent(ctx, agent1)
	_ = manager.RegisterAgent(ctx, agent2)

	// Unregister one
	err := manager.UnregisterAgent(ctx, "agent-one")
	if err != nil {
		t.Fatalf("UnregisterAgent() error = %v, want nil", err)
	}

	// List should only contain one agent
	list, err := manager.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() error = %v, want nil", err)
	}

	if len(list) != 1 {
		t.Errorf("ListAgents() after unregister len(list) = %d, want 1", len(list))
	}

	if len(list) > 0 && list[0].Name != "agent-two" {
		t.Errorf("ListAgents() after unregister list[0].Name = %s, want 'agent-two'", list[0].Name)
	}
}

// TestSubagentManager_ConcurrentOperations tests thread-safety of concurrent operations.
func TestSubagentManager_ConcurrentOperations(t *testing.T) {
	manager := &testableSubagentManager{}
	ctx := context.Background()

	// This test verifies that concurrent operations don't cause race conditions
	// Run with -race flag to detect data races

	done := make(chan bool)

	// Concurrent registrations
	go func() {
		_ = manager.RegisterAgent(ctx, &entity.Subagent{
			Name:        "concurrent-agent-1",
			Description: "Concurrent test agent 1",
			Model:       "sonnet",
			SourceType:  entity.SubagentSourceProgrammatic,
		})
		done <- true
	}()

	go func() {
		_ = manager.RegisterAgent(ctx, &entity.Subagent{
			Name:        "concurrent-agent-2",
			Description: "Concurrent test agent 2",
			Model:       "haiku",
			SourceType:  entity.SubagentSourceProgrammatic,
		})
		done <- true
	}()

	// Concurrent list operations
	go func() {
		_, _ = manager.ListAgents(ctx)
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// Verify final state is consistent
	list, err := manager.ListAgents(ctx)
	if err != nil {
		t.Errorf("ListAgents() after concurrent operations error = %v, want nil", err)
	}

	// Should have at least the registered agents (may have 0, 1, or 2 depending on timing)
	if list == nil {
		t.Error("ListAgents() after concurrent operations list = nil, want non-nil")
	}
}

// testableSubagentManager is an implementation that will fail all behavioral tests.
// This forces the real implementation to be created in the infrastructure layer.
type testableSubagentManager struct{}

func (m *testableSubagentManager) DiscoverAgents(_ context.Context) (*SubagentDiscoveryResult, error) {
	// This will fail the behavioral tests
	return nil, ErrNotImplemented
}

func (m *testableSubagentManager) LoadAgentMetadata(_ context.Context, _ string) (*entity.Subagent, error) {
	// This will fail the behavioral tests
	return nil, ErrNotImplemented
}

func (m *testableSubagentManager) RegisterAgent(_ context.Context, _ *entity.Subagent) error {
	// This will fail the behavioral tests
	return ErrNotImplemented
}

func (m *testableSubagentManager) UnregisterAgent(_ context.Context, _ string) error {
	// This will fail the behavioral tests
	return ErrNotImplemented
}

func (m *testableSubagentManager) GetAgentByName(_ context.Context, _ string) (*SubagentInfo, error) {
	// This will fail the behavioral tests
	return nil, ErrNotImplemented
}

func (m *testableSubagentManager) ListAgents(_ context.Context) ([]SubagentInfo, error) {
	// This will fail the behavioral tests
	return nil, ErrNotImplemented
}

// ErrNotImplemented is returned by the testable mock to indicate the feature is not yet implemented.
var ErrNotImplemented = &NotImplementedError{}

// NotImplementedError represents an unimplemented feature.
type NotImplementedError struct{}

func (e *NotImplementedError) Error() string {
	return "subagent manager feature not yet implemented"
}
