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
