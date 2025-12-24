// Package service provides application-level services that orchestrate
// the use cases and provide high-level interfaces for the application.
package service

import (
	"code-editing-agent/internal/application/dto"
	"code-editing-agent/internal/application/usecase"
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/domain/service"
	"context"
	"errors"
	"fmt"
)

var (
	// ErrMessageProcessUseCaseRequired is returned when MessageProcessUseCase is nil.
	ErrMessageProcessUseCaseRequired = errors.New("message process use case is required")

	// ErrToolExecutionUseCaseRequired is returned when ToolExecutionUseCase is nil.
	ErrToolExecutionUseCaseRequired = errors.New("tool execution use case is required")
)

// ChatService is the high-level orchestration service for chat operations.
// It coordinates the various use cases (message processing, tool execution)
// to provide a complete chat experience with tool support.
//
// This service serves as the main entry point for chat operations in the
// application layer, following hexagonal architecture principles.
type ChatService struct {
	messageProcessUseCase *usecase.MessageProcessUseCase
	toolExecutionUseCase  *usecase.ToolExecutionUseCase
	conversationService   *service.ConversationService
	userInterface         port.UserInterface
	aiProvider            port.AIProvider
	toolExecutor          port.ToolExecutor
	fileManager           port.FileManager
}

// NewChatService creates a new ChatService with all required dependencies.
//
// Parameters:
//   - msgProcUC: Message processing use case
//   - toolExecUC: Tool execution use case
//   - ui: User interface port for displaying messages
//   - ai: AI provider port
//   - toolExec: Tool executor port
//   - fm: File manager port
//
// Returns:
//   - *ChatService: A new chat service instance
//   - error: An error if any required dependency is nil
func NewChatService(
	msgProcUC *usecase.MessageProcessUseCase,
	toolExecUC *usecase.ToolExecutionUseCase,
	ui port.UserInterface,
	ai port.AIProvider,
	toolExec port.ToolExecutor,
	fm port.FileManager,
) (*ChatService, error) {
	if msgProcUC == nil {
		return nil, ErrMessageProcessUseCaseRequired
	}
	if toolExecUC == nil {
		return nil, ErrToolExecutionUseCaseRequired
	}
	if ui == nil {
		return nil, errors.New("user interface is required")
	}
	if ai == nil {
		return nil, errors.New("AI provider is required")
	}
	if toolExec == nil {
		return nil, errors.New("tool executor is required")
	}
	if fm == nil {
		return nil, errors.New("file manager is required")
	}

	// Extract conversation service from message process use case
	convService := msgProcUC.GetConversationService()

	return &ChatService{
		messageProcessUseCase: msgProcUC,
		toolExecutionUseCase:  toolExecUC,
		conversationService:   convService,
		userInterface:         ui,
		aiProvider:            ai,
		toolExecutor:          toolExec,
		fileManager:           fm,
	}, nil
}

// NewChatServiceFromDomain creates a ChatService directly from domain services and ports.
// This is a convenience factory method that creates the necessary use cases internally.
//
// Parameters:
//   - convService: Domain conversation service
//   - ui: User interface port
//   - ai: AI provider port
//   - toolExec: Tool executor port
//   - fm: File manager port
//
// Returns:
//   - *ChatService: A new chat service instance
//   - error: An error if any required dependency is nil
func NewChatServiceFromDomain(
	convService *service.ConversationService,
	ui port.UserInterface,
	ai port.AIProvider,
	toolExec port.ToolExecutor,
	fm port.FileManager,
) (*ChatService, error) {
	if convService == nil {
		return nil, errors.New("conversation service is required")
	}
	if ui == nil {
		return nil, errors.New("user interface is required")
	}
	if ai == nil {
		return nil, errors.New("AI provider is required")
	}
	if toolExec == nil {
		return nil, errors.New("tool executor is required")
	}
	if fm == nil {
		return nil, errors.New("file manager is required")
	}

	// Create use cases
	msgProcUC, err := usecase.NewMessageProcessUseCase(convService, ui)
	if err != nil {
		return nil, fmt.Errorf("failed to create message process use case: %w", err)
	}

	toolExecUC, err := usecase.NewToolExecutionUseCase(toolExec)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool execution use case: %w", err)
	}

	return &ChatService{
		messageProcessUseCase: msgProcUC,
		toolExecutionUseCase:  toolExecUC,
		conversationService:   convService,
		userInterface:         ui,
		aiProvider:            ai,
		toolExecutor:          toolExec,
		fileManager:           fm,
	}, nil
}

// StartSession starts a new chat session with an optional welcome message.
//
// Parameters:
//   - ctx: Context for the operation
//   - initialMessage: Optional initial message to send to the session
//
// Returns:
//   - *dto.StartChatResponse: The new session information
//   - error: An error if session creation fails
func (cs *ChatService) StartSession(
	ctx context.Context,
	initialMessage string,
) (*dto.StartChatResponse, error) {
	req := dto.StartChatRequest{
		InitialMessage: initialMessage,
	}

	resp, err := cs.messageProcessUseCase.StartNewSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start session: %w", err)
	}

	// Display welcome message through UI
	welcomeMsg := fmt.Sprintf("New session started: %s", resp.SessionID)
	_ = cs.userInterface.DisplaySystemMessage(welcomeMsg)

	return resp, nil
}

// SendMessage sends a user message and processes the AI's response.
// This is the main method for handling chat interactions.
//
// The flow is:
// 1. Send user message and get AI response
// 2. If AI requested tools, execute them
// 3. Send tool results back to AI
// 4. Repeat until AI has no more tool requests
// 5. Return final response
//
// Parameters:
//   - ctx: Context for the operation
//   - sessionID: The chat session ID
//   - message: The user's message
//
// Returns:
//   - *dto.SendMessageResponse: The AI's response with metadata
//   - error: An error if message processing fails
func (cs *ChatService) SendMessage(
	ctx context.Context,
	sessionID string,
	message string,
) (*dto.SendMessageResponse, error) {
	req := dto.SendMessageRequest{
		SessionID: sessionID,
		Message:   message,
	}

	// Process the user message
	resp, err := cs.messageProcessUseCase.ProcessUserMessage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to process message: %w", err)
	}

	// Display the assistant message if there is text content
	if resp.AssistantMsg != nil && resp.AssistantMsg.Content != "" {
		_ = cs.userInterface.DisplayMessage(resp.AssistantMsg.Content, entity.RoleAssistant)
	}

	// Handle tool requests if present
	if resp.HasTools {
		return cs.handleToolRequestCycle(ctx, resp)
	}

	return resp, nil
}

// handleToolRequestCycle manages the full cycle of tool execution and continuation.
// It executes tools and continues the conversation until the AI has no more tool requests.
//
// Parameters:
//   - ctx: Context for the operation
//   - initialResp: The initial response with tool requests
//
// Returns:
//   - *dto.SendMessageResponse: The final response after tool execution
//   - error: An error if tool execution fails
func (cs *ChatService) handleToolRequestCycle(
	ctx context.Context,
	initialResp *dto.SendMessageResponse,
) (*dto.SendMessageResponse, error) {
	currentResp := initialResp
	sessionID := initialResp.SessionID

	for currentResp.HasTools {
		// Execute tools for current iteration
		batchResp, err := cs.executeToolsForSession(ctx, sessionID, currentResp.ToolCalls)
		if err != nil {
			return nil, err
		}

		// Display the tool results
		cs.displayToolResults(batchResp.Results, currentResp.ToolCalls)

		// Add tool results to conversation so AI can see them
		err = cs.addToolResultsToConversation(ctx, sessionID, batchResp.Results, currentResp.ToolCalls)
		if err != nil {
			return nil, err
		}

		// Continue the chat and get next response
		currentResp, err = cs.continueAfterToolExecution(ctx, sessionID)
		if err != nil {
			return nil, err
		}
	}

	return currentResp, nil
}

// executeToolsForSession executes the requested tools for a session.
func (cs *ChatService) executeToolsForSession(
	ctx context.Context,
	sessionID string,
	toolCalls []dto.ToolCallInfo,
) (*dto.ToolExecutionBatchResponse, error) {
	toolReqs := make([]dto.ToolExecuteRequest, len(toolCalls))
	for i, tc := range toolCalls {
		toolReqs[i] = dto.ToolExecuteRequest{
			ToolName: tc.ToolName,
			Input:    tc.Input,
		}
	}

	batchResp, err := cs.toolExecutionUseCase.ExecuteToolsInSession(ctx, sessionID, toolReqs)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tools: %w", err)
	}
	return batchResp, nil
}

// displayToolResults displays the results of executed tools.
func (cs *ChatService) displayToolResults(
	results []dto.ToolExecutionResponse,
	toolCalls []dto.ToolCallInfo,
) {
	for _, result := range results {
		inputJSON := cs.findInputJSONForTool(result.ToolName, toolCalls)
		_ = cs.userInterface.DisplayToolResult(result.ToolName, inputJSON, result.Result)
	}
}

// findInputJSONForTool finds the input JSON for a given tool name.
func (cs *ChatService) findInputJSONForTool(toolName string, toolCalls []dto.ToolCallInfo) string {
	for _, tc := range toolCalls {
		if tc.ToolName == toolName && tc.InputJSON != "" {
			return tc.InputJSON
		}
	}
	return "{}"
}

// addToolResultsToConversation converts DTO tool results to entity ToolResults
// and adds them to the conversation as a tool result message.
func (cs *ChatService) addToolResultsToConversation(
	ctx context.Context,
	sessionID string,
	toolResults []dto.ToolExecutionResponse,
	toolCalls []dto.ToolCallInfo,
) error {
	// Match each ToolExecutionResponse to its corresponding ToolCallInfo
	// to get the ToolID (required for Anthropic to match tool results)
	entityToolResults := make([]entity.ToolResult, 0, len(toolResults))

	for _, result := range toolResults {
		// Find the matching tool call to get the ToolID
		var toolID string
		for _, tc := range toolCalls {
			if tc.ToolName == result.ToolName {
				toolID = tc.ToolID
				break
			}
		}

		// If no tool ID found, we can't match this result properly
		if toolID == "" {
			// Try to use tool name as fallback with a prefix
			toolID = "tool_" + result.ToolName
		}

		// Convert DTO result to entity ToolResult
		// IsError is true if there's an error message
		toolResult := entity.ToolResult{
			ToolID:  toolID,
			Result:  result.Result,
			IsError: result.Error != "",
		}

		entityToolResults = append(entityToolResults, toolResult)
	}

	// Call ConversationService to add the tool result message to the conversation
	if cs.conversationService == nil {
		return errors.New("conversation service is not initialized")
	}

	return cs.conversationService.AddToolResultMessage(ctx, sessionID, entityToolResults)
}

// continueAfterToolExecution continues the chat after tool execution.
func (cs *ChatService) continueAfterToolExecution(
	ctx context.Context,
	sessionID string,
) (*dto.SendMessageResponse, error) {
	contResp, err := cs.messageProcessUseCase.ContinueChat(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to continue chat after tool execution: %w", err)
	}

	// Display the assistant message if there is text content
	if contResp.AssistantMsg != nil && contResp.AssistantMsg.Content != "" {
		_ = cs.userInterface.DisplayMessage(contResp.AssistantMsg.Content, entity.RoleAssistant)
	}

	return &dto.SendMessageResponse{
		SessionID:    sessionID,
		AssistantMsg: contResp.AssistantMsg,
		HasTools:     contResp.HasTools,
		IsFinished:   contResp.IsFinished,
		ToolCalls:    []dto.ToolCallInfo{},
	}, nil
}

// EndSession ends a chat session.
//
// Parameters:
//   - ctx: Context for the operation
//   - sessionID: The session ID to end
//
// Returns:
//   - *dto.EndChatResponse: Session termination information
//   - error: An error if session termination fails
func (cs *ChatService) EndSession(
	ctx context.Context,
	sessionID string,
) (*dto.EndChatResponse, error) {
	resp, err := cs.messageProcessUseCase.EndSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	// Display goodbye message
	_ = cs.userInterface.DisplaySystemMessage(fmt.Sprintf("Session ended: %s", sessionID))

	return resp, nil
}

// GetSessionState retrieves the current state of a session.
//
// Parameters:
//   - sessionID: The session ID to query
//
// Returns:
//   - *dto.ConversationState: The current session state
//   - error: An error if state retrieval fails
func (cs *ChatService) GetSessionState(sessionID string) (*dto.ConversationState, error) {
	return cs.messageProcessUseCase.GetConversationState(sessionID)
}

// ListTools returns a list of all available tools.
//
// Returns:
//   - []dto.ToolDefinition: List of available tool definitions
//   - error: An error if tool listing fails
func (cs *ChatService) ListTools() ([]dto.ToolDefinition, error) {
	return cs.toolExecutionUseCase.ListAvailableTools()
}

// ExecuteTool executes a single tool (outside of a chat session).
//
// Parameters:
//   - ctx: Context for the operation
//   - toolName: The name of the tool to execute
//   - input: The input parameters for the tool
//
// Returns:
//   - *dto.ToolExecutionResponse: The tool execution result
//   - error: An error if execution fails
func (cs *ChatService) ExecuteTool(
	ctx context.Context,
	toolName string,
	input interface{},
) (*dto.ToolExecutionResponse, error) {
	req := dto.ToolExecuteRequest{
		ToolName: toolName,
		Input:    input,
	}

	return cs.toolExecutionUseCase.ExecuteTool(ctx, req)
}

// RegisterTool registers a new tool for use in chat sessions.
//
// Parameters:
//   - toolDef: The tool definition to register
//
// Returns:
//   - error: An error if registration fails
func (cs *ChatService) RegisterTool(toolDef dto.ToolDefinition) error {
	if err := cs.toolExecutionUseCase.RegisterTool(toolDef); err != nil {
		return fmt.Errorf("failed to register tool: %w", err)
	}
	return nil
}

// UnregisterTool removes a tool from the registry.
//
// Parameters:
//   - toolName: The name of the tool to unregister
//
// Returns:
//   - error: An error if unregistration fails
func (cs *ChatService) UnregisterTool(toolName string) error {
	return cs.toolExecutionUseCase.UnregisterTool(toolName)
}

// HealthCheck performs a health check on all service dependencies.
//
// Parameters:
//   - ctx: Context for the operation
//
// Returns:
//   - error: An error if any dependency is unhealthy
func (cs *ChatService) HealthCheck(ctx context.Context) error {
	// Check AI provider health
	if err := cs.aiProvider.HealthCheck(ctx); err != nil {
		return fmt.Errorf("AI provider health check failed: %w", err)
	}

	// File manager doesn't have a health check in the port interface
	// but we could verify file operations are working

	return nil
}

// GetAIModel returns the currently configured AI model.
//
// Returns:
//   - string: The current AI model identifier
func (cs *ChatService) GetAIModel() string {
	return cs.aiProvider.GetModel()
}

// SetAIModel sets the AI model to use for subsequent requests.
//
// Parameters:
//   - model: The model identifier to use
//
// Returns:
//   - error: An error if model setting fails
func (cs *ChatService) SetAIModel(model string) error {
	return cs.aiProvider.SetModel(model)
}

// GetPorts returns references to the internal ports for advanced use cases.
// This is primarily intended for testing or scenarios where direct port access is needed.
//
// Returns:
//   - port.AIProvider: The AI provider port
//   - port.ToolExecutor: The tool executor port
//   - port.FileManager: The file manager port
//   - port.UserInterface: The user interface port
func (cs *ChatService) GetPorts() (port.AIProvider, port.ToolExecutor, port.FileManager, port.UserInterface) {
	return cs.aiProvider, cs.toolExecutor, cs.fileManager, cs.userInterface
}
