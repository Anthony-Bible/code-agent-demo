// Package usecase contains application use cases.
package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"fmt"
	"time"
)

// SubagentRunnerInterface defines the interface for running subagents.
// This allows SubagentUseCase to work with both real and mock runners.
type SubagentRunnerInterface interface {
	Run(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error)
}

// SubagentUseCase orchestrates subagent spawning and task delegation.
// It provides a high-level API for discovering subagents, loading their metadata,
// and executing tasks in isolated conversation contexts.
type SubagentUseCase struct {
	subagentManager port.SubagentManager
	subagentRunner  SubagentRunnerInterface
}

// NewSubagentUseCase creates a new SubagentUseCase with the required dependencies.
//
// Parameters:
//   - subagentManager: Manager for discovering and loading subagent metadata
//   - subagentRunner: Runner for executing subagent tasks in isolated contexts
//
// Panics if any required dependency is nil.
func NewSubagentUseCase(
	subagentManager port.SubagentManager,
	subagentRunner SubagentRunnerInterface,
) *SubagentUseCase {
	if subagentManager == nil {
		panic("subagentManager cannot be nil")
	}
	if subagentRunner == nil {
		panic("subagentRunner cannot be nil")
	}
	return &SubagentUseCase{
		subagentManager: subagentManager,
		subagentRunner:  subagentRunner,
	}
}

// SpawnSubagent spawns a subagent with the given name and executes the task prompt.
//
// The subagent spawning follows this flow:
//  1. Validate inputs (agentName and prompt must be non-empty)
//  2. Load agent metadata from the subagent manager
//  3. Generate a unique subagent ID for this execution
//  4. Delegate to subagent runner for task execution
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - agentName: Name of the subagent to spawn (must be non-empty)
//   - prompt: Task prompt to execute (must be non-empty)
//
// Returns:
//   - *SubagentResult: Result of the subagent execution (status, output, etc.)
//   - error: Validation errors, agent not found errors, or execution errors
func (uc *SubagentUseCase) SpawnSubagent(
	ctx context.Context,
	agentName string,
	prompt string,
) (*SubagentResult, error) {
	// Validate required inputs
	if err := validateSpawnInputs(agentName, prompt); err != nil {
		return nil, err
	}

	// Load agent metadata from manager
	agent, err := uc.subagentManager.LoadAgentMetadata(ctx, agentName)
	if err != nil {
		return nil, fmt.Errorf("failed to load subagent metadata: %w", err)
	}

	// Generate unique ID for this subagent execution
	subagentID := generateSubagentID()

	// Execute the subagent task
	return uc.subagentRunner.Run(ctx, agent, prompt, subagentID)
}

// validateSpawnInputs validates the inputs for SpawnSubagent.
func validateSpawnInputs(agentName, prompt string) error {
	if agentName == "" {
		return errors.New("agentName cannot be empty")
	}
	if prompt == "" {
		return errors.New("prompt cannot be empty")
	}
	return nil
}

// generateSubagentID generates a unique identifier for a subagent execution.
func generateSubagentID() string {
	return fmt.Sprintf("subagent-%d", time.Now().UnixNano())
}
