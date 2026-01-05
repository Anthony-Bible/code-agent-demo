package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
)

// SubagentInfo represents information about a discovered subagent.
type SubagentInfo struct {
	Name          string                    `json:"name"`           // Name of the subagent
	Description   string                    `json:"description"`    // Description of what the subagent does
	AllowedTools  []string                  `json:"allowed_tools"`  // Allowed tools for this subagent
	Model         entity.SubagentModel      `json:"model"`          // AI model to use
	SourceType    entity.SubagentSourceType `json:"source_type"`    // Where the subagent was discovered from
	DirectoryPath string                    `json:"directory_path"` // Path to subagent directory
}

// SubagentDiscoveryResult represents the result of a subagent discovery operation.
type SubagentDiscoveryResult struct {
	Subagents  []SubagentInfo `json:"subagents"`   // Discovered subagents
	AgentsDirs []string       `json:"agents_dirs"` // All directories that were searched for subagents
	TotalCount int            `json:"total_count"` // Total number of subagents discovered
}

// SubagentManager defines the interface for managing subagents.
// This port represents the outbound dependency for subagent operations and follows
// hexagonal architecture principles by abstracting subagent management implementations.
type SubagentManager interface {
	// DiscoverAgents scans the agents directories for available subagents.
	// Subagents are discovered from three locations in priority order:
	// 1. ./agents (project root, highest priority)
	// 2. ./.claude/agents (project .claude directory)
	// 3. ~/.claude/agents (user global, lowest priority)
	// Returns information about all discovered subagents including metadata.
	DiscoverAgents(ctx context.Context) (*SubagentDiscoveryResult, error)

	// LoadAgentMetadata loads the metadata for a specific subagent from its AGENT.md file.
	// The agentName should match the subagent directory name.
	// Returns the subagent entity with all parsed metadata.
	LoadAgentMetadata(ctx context.Context, agentName string) (*entity.Subagent, error)

	// RegisterAgent registers a subagent, making it available for use.
	// Registered subagents can be invoked by the AI through the tool system.
	// Returns an error if the subagent is invalid or already registered.
	RegisterAgent(ctx context.Context, agent *entity.Subagent) error

	// UnregisterAgent unregisters a subagent by name, removing it from available subagents.
	// Returns an error if the subagent is not found or cannot be unregistered.
	UnregisterAgent(ctx context.Context, agentName string) error

	// GetAgentByName returns information about a specific subagent by name.
	// Returns nil if the subagent is not found.
	GetAgentByName(ctx context.Context, agentName string) (*SubagentInfo, error)

	// ListAgents returns a list of all registered subagents.
	// Registered subagents are those that have been registered and are available for use.
	ListAgents(ctx context.Context) ([]SubagentInfo, error)
}
