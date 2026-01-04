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

// SubagentHandle provides access to an asynchronously running subagent.
// It contains channels for receiving either the result or error from the background execution.
//
// Channel behavior:
//   - Exactly ONE message will be sent (either Result OR Error, never both)
//   - Both channels are closed after the message is sent
//   - Channels are buffered (size 1) to prevent goroutine leaks if the caller doesn't read
//
// Example usage:
//
//	handle, err := usecase.SpawnSubagentAsync(ctx, "agent-name", "prompt")
//	if err != nil { ... }
//	select {
//	case result := <-handle.Result:
//	    // Handle success
//	case err := <-handle.Error:
//	    // Handle error
//	}
type SubagentHandle struct {
	SubagentID string                 // Unique identifier for this subagent execution
	AgentName  string                 // Name of the spawned agent
	Result     <-chan *SubagentResult // Channel that receives the result on success (closed after sending)
	Error      <-chan error           // Channel that receives error on failure (closed after sending)
}

// SpawnSubagentAsync spawns a subagent asynchronously and returns immediately with a handle.
//
// Unlike SpawnSubagent which blocks until completion, this method:
//  1. Validates inputs synchronously (returns error immediately if invalid)
//  2. Loads agent metadata synchronously (returns error immediately if not found)
//  3. Spawns a goroutine for background execution
//  4. Returns immediately with a SubagentHandle
//
// The handle provides Result and Error channels for receiving the outcome:
//   - Exactly ONE message will be sent (either Result OR Error, never both)
//   - Both channels are closed after the message is sent
//   - Channels are buffered (size 1) to prevent goroutine leaks
//
// Context cancellation is supported - if ctx is cancelled during execution,
// an error will be sent to the Error channel. If ctx is already cancelled
// before spawning, an error is returned immediately without starting a goroutine.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - agentName: Name of the subagent to spawn (must be non-empty)
//   - prompt: Task prompt to execute (must be non-empty)
//
// Returns:
//   - *SubagentHandle: Handle with Result/Error channels for async completion
//   - error: Validation errors, agent not found errors, or pre-cancelled context
func (uc *SubagentUseCase) SpawnSubagentAsync(
	ctx context.Context,
	agentName string,
	prompt string,
) (*SubagentHandle, error) {
	// 1. Validate inputs (synchronous - fail fast)
	if err := validateSpawnInputs(agentName, prompt); err != nil {
		return nil, err
	}

	// 2. Check if context is already cancelled (fail fast - no goroutine needed)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 3. Load agent metadata (synchronous - fail fast if agent not found)
	agent, err := uc.subagentManager.LoadAgentMetadata(ctx, agentName)
	if err != nil {
		return nil, fmt.Errorf("failed to load subagent metadata: %w", err)
	}

	// 4. Generate unique subagent ID
	subagentID := generateSubagentID()

	// 5. Create buffered channels (buffer=1 to prevent goroutine leaks)
	resultChan := make(chan *SubagentResult, 1)
	errorChan := make(chan error, 1)

	// 6. Create handle
	handle := &SubagentHandle{
		SubagentID: subagentID,
		AgentName:  agent.Name,
		Result:     resultChan,
		Error:      errorChan,
	}

	// 7. Spawn goroutine for background execution
	go uc.executeInBackground(ctx, resultChan, errorChan, agent, prompt, subagentID)

	// 8. Return immediately (non-blocking)
	return handle, nil
}

// executeInBackground runs the subagent task and sends the result to the appropriate channel.
// Both channels are closed after sending to signal completion and prevent goroutine leaks.
//
// This method is extracted from SpawnSubagentAsync to improve code organization and testability.
func (uc *SubagentUseCase) executeInBackground(
	ctx context.Context,
	resultChan chan *SubagentResult,
	errorChan chan error,
	agent *entity.Subagent,
	prompt string,
	subagentID string,
) {
	defer close(resultChan)
	defer close(errorChan)

	result, err := uc.subagentRunner.Run(ctx, agent, prompt, subagentID)
	if err != nil {
		errorChan <- err
	} else {
		resultChan <- result
	}
}
