// Package usecase contains application use cases.
package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"fmt"
	"sync"
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

// DynamicSubagentConfig holds the configuration for creating a dynamic subagent.
//
// Dynamic subagents are created at runtime with custom system prompts, without requiring
// a pre-defined AGENT.md file. This is useful for delegating complex tasks that benefit
// from isolated context.
type DynamicSubagentConfig struct {
	Name         string   // Required: Short identifier for the agent (for logging/tracking)
	Description  string   // Optional: What this agent is for (for logging)
	SystemPrompt string   // Required: Instructions defining the agent's role, approach, and output format
	Model        string   // Optional: AI model to use (haiku, sonnet, opus, inherit). Default: "inherit"
	MaxActions   int      // Optional: Maximum tool calls before stopping. Default: 30
	AllowedTools []string // Optional: Tools this agent can use. nil = all tools (default)
}

// SpawnDynamicSubagent creates and spawns a dynamic subagent with custom configuration.
//
// Unlike SpawnSubagent which requires a pre-registered agent in AGENT.md files,
// this method creates an agent on-the-fly with inline configuration. The agent
// runs in an isolated conversation context with the provided system prompt.
//
// Configuration validation:
//   - Name must be non-empty
//   - SystemPrompt must be non-empty
//   - taskPrompt must be non-empty
//
// Default values applied:
//   - Model: "inherit" (use same model as parent agent)
//   - MaxActions: 30 (maximum tool calls before stopping)
//   - AllowedTools: nil (all tools available)
//
// The dynamic agent is created with SourceType = SubagentSourceProgrammatic.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - config: Dynamic agent configuration (name, system_prompt required)
//   - taskPrompt: The specific task for the agent to complete (required)
//
// Returns:
//   - *SubagentResult: Result of the subagent execution (status, output, etc.)
//   - error: Validation errors or execution errors
func (uc *SubagentUseCase) SpawnDynamicSubagent(
	ctx context.Context,
	config DynamicSubagentConfig,
	taskPrompt string,
) (*SubagentResult, error) {
	// 1. Validate required fields
	if err := validateDynamicConfig(config, taskPrompt); err != nil {
		return nil, err
	}

	// 2. Apply defaults
	model := config.Model
	if model == "" {
		model = "inherit"
	}

	maxActions := config.MaxActions
	if maxActions == 0 {
		maxActions = 30
	}

	// 3. Create entity.Subagent with programmatic source
	agent := &entity.Subagent{
		Name:         config.Name,
		Description:  config.Description,
		RawContent:   config.SystemPrompt, // System prompt goes into RawContent
		Model:        model,
		MaxActions:   maxActions,
		AllowedTools: config.AllowedTools, // nil means all tools
		SourceType:   entity.SubagentSourceProgrammatic,
	}

	// 4. Generate unique ID for this subagent execution
	subagentID := generateSubagentID()

	// 5. Execute the subagent task
	return uc.subagentRunner.Run(ctx, agent, taskPrompt, subagentID)
}

// validateDynamicConfig validates the configuration for dynamic subagent spawn.
func validateDynamicConfig(config DynamicSubagentConfig, taskPrompt string) error {
	if config.Name == "" {
		return errors.New("name cannot be empty")
	}
	if config.SystemPrompt == "" {
		return errors.New("system_prompt cannot be empty")
	}
	if taskPrompt == "" {
		return errors.New("task prompt cannot be empty")
	}
	return nil
}

// SubagentRequest represents a single subagent spawn request for batch operations.
//
// Used by SpawnMultiple to execute multiple subagent tasks in parallel.
// Each request specifies the agent to spawn and the task prompt to execute.
type SubagentRequest struct {
	AgentName string // Name of the subagent to spawn (must be non-empty)
	Prompt    string // Task prompt to execute (must be non-empty)
}

// SubagentBatchResult holds the results from spawning multiple subagents in parallel.
//
// The Results and Errors slices are guaranteed to match the order of the input requests.
// For each request at index i:
//   - If successful: Results[i] contains the result, Errors[i] is nil
//   - If failed: Results[i] is nil, Errors[i] contains the error
//
// This design allows callers to iterate through results and errors together,
// knowing they correspond to the same request index.
type SubagentBatchResult struct {
	Results []*SubagentResult // Results for each request, in same order as input
	Errors  []error           // Errors for each request, nil if successful
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

// SpawnMultiple spawns multiple subagents in parallel and returns results for all requests.
//
// This method executes each subagent request concurrently using goroutines, making it
// significantly faster than sequential execution for multiple requests. All spawns execute
// simultaneously, and the method waits for all to complete before returning.
//
// Ordering Guarantee:
//   - Results and Errors slices are pre-allocated with length = len(requests)
//   - Each goroutine writes to a unique index matching its request position
//   - No locks needed for writes (different memory locations)
//   - Results maintain input request order regardless of completion order
//
// Error Handling:
//   - This method never returns an error (second return value is always nil)
//   - Individual request errors are captured in SubagentBatchResult.Errors
//   - Failed requests don't affect other requests in the batch
//   - Use SubagentBatchResult.Errors to check for individual failures
//
// Behavior:
//   - Returns empty result (not error) for nil or empty requests
//   - Each request gets a unique SubagentID
//   - Context cancellation propagates to all spawns
//   - Thread-safe concurrent execution
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - requests: Array of subagent spawn requests
//
// Returns:
//   - *SubagentBatchResult: Results and errors for all requests
//   - error: Always nil (individual errors are in SubagentBatchResult.Errors)
func (uc *SubagentUseCase) SpawnMultiple(
	ctx context.Context,
	requests []*SubagentRequest,
) (*SubagentBatchResult, error) {
	// 1. Handle empty requests
	if len(requests) == 0 {
		return &SubagentBatchResult{
			Results: []*SubagentResult{},
			Errors:  []error{},
		}, nil
	}

	// 2. Pre-allocate results and errors slices
	results := make([]*SubagentResult, len(requests))
	errors := make([]error, len(requests))

	// 3. Use WaitGroup for coordination
	var wg sync.WaitGroup

	// 4. Spawn goroutines for parallel execution
	for i, req := range requests {
		wg.Add(1)
		go func(index int, request *SubagentRequest) {
			defer wg.Done()

			// Call synchronous SpawnSubagent (reuse existing logic)
			result, err := uc.SpawnSubagent(ctx, request.AgentName, request.Prompt)

			// Store in pre-allocated position (maintains order)
			results[index] = result
			errors[index] = err
		}(i, req)
	}

	// 5. Wait for all goroutines to complete
	wg.Wait()

	// 6. Return batch result
	return &SubagentBatchResult{
		Results: results,
		Errors:  errors,
	}, nil
}
