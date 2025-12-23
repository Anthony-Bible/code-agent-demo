// Package usecase provides use case implementations for the application layer.
package usecase

import (
	"code-editing-agent/internal/application/dto"
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrToolExecutorRequired is returned when ToolExecutor is nil.
	ErrToolExecutorRequired = errors.New("tool executor is required")

	// ErrToolNotFound is returned when a tool is not found in the registry.
	ErrToolNotFound = errors.New("tool not found")

	// ErrToolExecutionFailed is returned when tool execution fails.
	ErrToolExecutionFailed = errors.New("tool execution failed")
)

// ToolExecutionUseCase handles the execution of tools requested by the AI.
// It orchestrates the tool execution flow through the domain ToolExecutor port,
// handling validation, execution, and result collection.
//
// This use case works in conjunction with MessageProcessUseCase to provide
// a complete chat experience with tool capabilities.
type ToolExecutionUseCase struct {
	toolExecutor port.ToolExecutor
}

// NewToolExecutionUseCase creates a new ToolExecutionUseCase.
//
// Parameters:
//   - toolExecutor: The domain tool executor port for executing tools
//
// Returns:
//   - *ToolExecutionUseCase: A new use case instance
//   - error: An error if the tool executor is nil
func NewToolExecutionUseCase(
	toolExecutor port.ToolExecutor,
) (*ToolExecutionUseCase, error) {
	if toolExecutor == nil {
		return nil, ErrToolExecutorRequired
	}

	return &ToolExecutionUseCase{
		toolExecutor: toolExecutor,
	}, nil
}

// ExecuteTool executes a single tool with the given input.
//
// Parameters:
//   - ctx: Context for the operation (supports cancellation and timeout)
//   - req: The tool execution request containing tool name and input
//
// Returns:
//   - *dto.ToolExecutionResponse: The tool execution result
//   - error: An error if execution fails
func (uc *ToolExecutionUseCase) ExecuteTool(
	ctx context.Context,
	req dto.ToolExecuteRequest,
) (*dto.ToolExecutionResponse, error) {
	// Validate the request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Check if tool exists
	tool, found := uc.toolExecutor.GetTool(req.ToolName)
	if !found {
		return &dto.ToolExecutionResponse{
			SessionID:  "",
			ToolName:   req.ToolName,
			Success:    false,
			Error:      ErrToolNotFound.Error(),
			ExecutedAt: time.Now(),
			DurationMs: 0,
		}, nil
	}

	// Start execution timer
	startTime := time.Now()

	// Validate tool input if it has a schema
	if tool.HasSchema() && tool.GetRequiredFieldsCount() > 0 {
		// Input validation is handled by the tool executor
		if err := uc.toolExecutor.ValidateToolInput(req.ToolName, req.Input); err != nil {
			return dto.NewToolExecutionResponse("", req.ToolName, "", err, time.Since(startTime)), nil
		}
	}

	// Execute the tool
	result, err := uc.toolExecutor.ExecuteTool(ctx, req.ToolName, req.Input)
	duration := time.Since(startTime)

	return dto.NewToolExecutionResponse("", req.ToolName, result, err, duration), nil
}

// ExecuteToolsInSession executes tools requested during a chat session.
// This method is called when the AI requests tool execution during message processing.
//
// Parameters:
//   - ctx: Context for the operation
//   - sessionID: The conversation session ID
//   - tools: List of tool requests to execute
//
// Returns:
//   - *dto.ToolExecutionBatchResponse: The batch execution results
//   - error: An error if the request is invalid
func (uc *ToolExecutionUseCase) ExecuteToolsInSession(
	ctx context.Context,
	sessionID string,
	tools []dto.ToolExecuteRequest,
) (*dto.ToolExecutionBatchResponse, error) {
	if sessionID == "" {
		return nil, dto.ErrEmptySessionID
	}

	if len(tools) == 0 {
		return nil, dto.ErrEmptyToolList
	}

	totalStart := time.Now()
	results := make([]dto.ToolExecutionResponse, len(tools))
	successfulCount := 0

	for i, toolReq := range tools {
		// Add session ID to the execution context
		startTime := time.Now()

		// Validate request
		if err := toolReq.Validate(); err != nil {
			results[i] = dto.ToolExecutionResponse{
				SessionID:  sessionID,
				ToolName:   toolReq.ToolName,
				Success:    false,
				Error:      fmt.Sprintf("invalid request: %v", err),
				ExecutedAt: time.Now(),
				DurationMs: 0,
			}
			continue
		}

		// Check if tool exists
		tool, found := uc.toolExecutor.GetTool(toolReq.ToolName)
		if !found {
			results[i] = dto.ToolExecutionResponse{
				SessionID:  sessionID,
				ToolName:   toolReq.ToolName,
				Success:    false,
				Error:      ErrToolNotFound.Error(),
				ExecutedAt: time.Now(),
				DurationMs: 0,
			}
			continue
		}

		// Validate input if needed
		if tool.HasSchema() && tool.GetRequiredFieldsCount() > 0 {
			if err := uc.toolExecutor.ValidateToolInput(toolReq.ToolName, toolReq.Input); err != nil {
				results[i] = dto.ToolExecutionResponse{
					SessionID:  sessionID,
					ToolName:   toolReq.ToolName,
					Success:    false,
					Error:      fmt.Sprintf("input validation failed: %v", err),
					ExecutedAt: time.Now(),
					DurationMs: 0,
				}
				continue
			}
		}

		// Execute the tool
		result, err := uc.toolExecutor.ExecuteTool(ctx, toolReq.ToolName, toolReq.Input)
		duration := time.Since(startTime)

		if err != nil {
			results[i] = dto.ToolExecutionResponse{
				SessionID:  sessionID,
				ToolName:   toolReq.ToolName,
				Success:    false,
				Error:      err.Error(),
				ExecutedAt: time.Now(),
				DurationMs: duration.Milliseconds(),
			}
		} else {
			results[i] = dto.ToolExecutionResponse{
				SessionID:  sessionID,
				ToolName:   toolReq.ToolName,
				Success:    true,
				Result:     result,
				ExecutedAt: time.Now(),
				DurationMs: duration.Milliseconds(),
			}
			successfulCount++
		}
	}

	totalDuration := time.Since(totalStart)
	failedCount := len(tools) - successfulCount

	return &dto.ToolExecutionBatchResponse{
		SessionID:       sessionID,
		Results:         results,
		TotalTools:      len(tools),
		SuccessfulCount: successfulCount,
		FailedCount:     failedCount,
		TotalDurationMs: totalDuration.Milliseconds(),
	}, nil
}

// ListAvailableTools returns a list of all available tools.
//
// Returns:
//   - []dto.ToolDefinition: List of available tool definitions
//   - error: An error if tool listing fails
func (uc *ToolExecutionUseCase) ListAvailableTools() ([]dto.ToolDefinition, error) {
	domainTools, err := uc.toolExecutor.ListTools()
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	tools := make([]dto.ToolDefinition, len(domainTools))
	for i, domainTool := range domainTools {
		tools[i] = dto.ToolDefinition{
			Name:        domainTool.Name,
			Description: domainTool.Description,
			ID:          domainTool.ID,
			InputSchema: domainTool.InputSchema,
		}
	}

	return tools, nil
}

// GetToolDefinition retrieves a specific tool definition by name.
//
// Parameters:
//   - toolName: The name of the tool to retrieve
//
// Returns:
//   - *dto.ToolDefinition: The tool definition if found
//   - bool: True if the tool was found
func (uc *ToolExecutionUseCase) GetToolDefinition(
	toolName string,
) (*dto.ToolDefinition, bool) {
	if toolName == "" {
		return nil, false
	}

	domainTool, found := uc.toolExecutor.GetTool(toolName)
	if !found {
		return nil, false
	}

	return &dto.ToolDefinition{
		Name:        domainTool.Name,
		Description: domainTool.Description,
		ID:          domainTool.ID,
		InputSchema: domainTool.InputSchema,
	}, true
}

// RegisterTool registers a new tool for execution.
//
// Parameters:
//   - toolDef: The tool definition to register
//
// Returns:
//   - error: An error if registration fails
func (uc *ToolExecutionUseCase) RegisterTool(
	toolDef dto.ToolDefinition,
) error {
	if err := toolDef.Validate(); err != nil {
		return fmt.Errorf("invalid tool definition: %w", err)
	}

	// Convert DTO tool to domain entity tool
	domainTool := domainToolFromDTO(toolDef)

	return uc.toolExecutor.RegisterTool(domainTool)
}

// UnregisterTool removes a tool from the registry.
//
// Parameters:
//   - toolName: The name of the tool to unregister
//
// Returns:
//   - error: An error if unregistration fails
func (uc *ToolExecutionUseCase) UnregisterTool(toolName string) error {
	if toolName == "" {
		return dto.NewValidationError("tool name cannot be empty")
	}
	return uc.toolExecutor.UnregisterTool(toolName)
}

// Helper function: domainToolFromDTO converts a DTO tool definition to a domain tool entity.
func domainToolFromDTO(dtoTool dto.ToolDefinition) entity.Tool {
	tool, _ := entity.NewTool(dtoTool.ID, dtoTool.Name, dtoTool.Description)
	if dtoTool.InputSchema != nil {
		_ = tool.AddInputSchema(dtoTool.InputSchema, nil)
	}
	return *tool
}
