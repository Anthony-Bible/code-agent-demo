package service

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrToolNotFound         = errors.New("tool not found")
)

const (
	// minCompactionThreshold is the minimum allowed compaction threshold to prevent thrashing.
	minCompactionThreshold = int64(10000)
	// maxSummaryContentRunes is the maximum number of runes to include from a message's content in a compaction summary.
	maxSummaryContentRunes = 2000
	// maxSummaryToolResultRunes is the maximum number of runes to include from a tool result in a compaction summary.
	maxSummaryToolResultRunes = 200
)

// ConversationService handles the core business logic for managing conversations.
// It orchestrates the flow of messages between users and AI, processes tool executions,
// maintains conversation state, and coordinates with the AI provider.
type ConversationService struct {
	aiProvider    port.AIProvider
	toolExecutor  port.ToolExecutor
	conversations map[string]*entity.Conversation
	// conversationsMu protects conversations map reads in new skill-related methods.
	// Note: Pre-existing methods (GetConversation, EndConversation, etc.) access
	// conversations without this mutex. Consistent usage is a separate refactor.
	conversationsMu        sync.RWMutex
	currentSession         string
	processing             map[string]bool
	processingMu           sync.RWMutex // Protects processing map for concurrent access
	sessionModes           map[string]bool
	sessionModesMu         sync.RWMutex // Protects sessionModes map for concurrent access
	sessionThinkingModes   map[string]port.ThinkingModeInfo
	sessionThinkingModesMu sync.RWMutex // Protects sessionThinkingModes map for concurrent access
	sessionSystemPrompts   map[string]string
	sessionSystemPromptsMu sync.RWMutex     // Protects sessionSystemPrompts map for concurrent access
	sessionTokenUsage      map[string]int64 // total tokens (input+output) from latest API response
	sessionTokenUsageMu    sync.RWMutex
	sessionActiveSkills    map[string][]entity.Skill  // Active skills per session with allowed-tools
	sessionActiveSkillsMu  sync.RWMutex               // Protects sessionActiveSkills map for concurrent access
	sessionAllowedTools    map[string]map[string]bool // Cached union of allowed tools per session
	sessionAllowedToolsMu  sync.RWMutex               // Protects sessionAllowedTools map for concurrent access
	compactionThreshold    int64
}

// NewConversationService creates a new instance of ConversationService.
// It requires an AI provider and tool executor for operations.
// An optional compactionThreshold parameter sets the token threshold for auto-compaction.
func NewConversationService(
	aiProvider port.AIProvider,
	toolExecutor port.ToolExecutor,
	compactionThreshold ...int64,
) (*ConversationService, error) {
	if aiProvider == nil {
		return nil, errors.New("AI provider cannot be nil")
	}
	if toolExecutor == nil {
		return nil, errors.New("tool executor cannot be nil")
	}

	threshold := int64(160000)
	if len(compactionThreshold) > 0 && compactionThreshold[0] > 0 {
		threshold = compactionThreshold[0]
	}
	if threshold < minCompactionThreshold {
		threshold = minCompactionThreshold
	}

	return &ConversationService{
		aiProvider:           aiProvider,
		toolExecutor:         toolExecutor,
		conversations:        make(map[string]*entity.Conversation),
		processing:           make(map[string]bool),
		sessionModes:         make(map[string]bool),
		sessionThinkingModes: make(map[string]port.ThinkingModeInfo),
		sessionSystemPrompts: make(map[string]string),
		sessionTokenUsage:    make(map[string]int64),
		sessionActiveSkills:  make(map[string][]entity.Skill),
		sessionAllowedTools:  make(map[string]map[string]bool),
		compactionThreshold:  threshold,
	}, nil
}

// StartConversation creates a new conversation session with a unique identifier.
func (cs *ConversationService) StartConversation(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", context.Canceled
	default:
	}

	sessionID := generateSessionID()
	conversation, err := entity.NewConversation()
	if err != nil {
		return "", err
	}

	cs.conversationsMu.Lock()
	cs.conversations[sessionID] = conversation
	cs.conversationsMu.Unlock()

	cs.currentSession = sessionID
	cs.processingMu.Lock()
	cs.processing[sessionID] = false
	cs.processingMu.Unlock()

	return sessionID, nil
}

// AddUserMessage adds a user message to the current conversation.
func (cs *ConversationService) AddUserMessage(ctx context.Context, sessionID, content string) (*entity.Message, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	cs.conversationsMu.RLock()
	conversation, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return nil, ErrConversationNotFound
	}

	message, err := entity.NewMessage(entity.RoleUser, content)
	if err != nil {
		return nil, err
	}

	err = conversation.AddMessage(*message)
	if err != nil {
		return nil, err
	}

	return message, nil
}

// AddToolResultMessage adds tool execution results to the conversation.
func (cs *ConversationService) AddToolResultMessage(
	ctx context.Context,
	sessionID string,
	toolResults []entity.ToolResult,
) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	cs.conversationsMu.RLock()
	conversation, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return ErrConversationNotFound
	}

	message, err := entity.NewToolResultMessage(entity.RoleUser, toolResults)
	if err != nil {
		return err
	}

	return conversation.AddMessage(*message)
}

// ProcessAssistantResponse processes an AI assistant response, handling tools and text.
func (cs *ConversationService) ProcessAssistantResponse(
	ctx context.Context,
	sessionID string,
) (*entity.Message, []port.ToolCallInfo, error) {
	// Prepare context and parameters
	conversation, messageParams, toolParams, preparedCtx, err := cs.prepareAIRequest(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}

	// Send to AI provider
	response, toolCalls, err := cs.aiProvider.SendMessage(preparedCtx, messageParams, toolParams)
	if err != nil {
		return nil, nil, err
	}

	// Finalize response
	return cs.finalizeAIResponse(ctx, sessionID, conversation, response, toolCalls)
}

// ProcessAssistantResponseStreaming processes an AI assistant response with streaming support.
// It calls the provided callback for each text chunk as it arrives from the AI provider.
func (cs *ConversationService) ProcessAssistantResponseStreaming(
	ctx context.Context,
	sessionID string,
	textCallback port.StreamCallback,
	thinkingCallback port.ThinkingCallback,
) (*entity.Message, []port.ToolCallInfo, error) {
	// Prepare context and parameters
	conversation, messageParams, toolParams, preparedCtx, err := cs.prepareAIRequest(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}

	// Send to AI provider with streaming
	response, toolCalls, err := cs.aiProvider.SendMessageStreaming(
		preparedCtx,
		messageParams,
		toolParams,
		textCallback,
		thinkingCallback,
	)
	if err != nil {
		return nil, nil, err
	}

	// Finalize response
	return cs.finalizeAIResponse(ctx, sessionID, conversation, response, toolCalls)
}

// prepareAIRequest prepares the context, message parameters, and tool parameters for an AI request.
// This is shared logic between streaming and non-streaming requests.
func (cs *ConversationService) prepareAIRequest(
	ctx context.Context,
	sessionID string,
) (*entity.Conversation, []port.MessageParam, []port.ToolParam, context.Context, error) {
	select {
	case <-ctx.Done():
		return nil, nil, nil, nil, fmt.Errorf("context cancelled before AI call: %w", ctx.Err())
	default:
	}

	cs.conversationsMu.RLock()
	conversation, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return nil, nil, nil, nil, ErrConversationNotFound
	}

	// Get conversation history for AI provider
	messages := conversation.Messages
	messageParams := make([]port.MessageParam, len(messages))
	for i, msg := range messages {
		// Convert ToolCalls from entity to port
		var toolCallParams []port.ToolCallParam
		if len(msg.ToolCalls) > 0 {
			toolCallParams = make([]port.ToolCallParam, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				toolCallParams[j] = port.ToolCallParam{
					ToolID:           tc.ToolID,
					ToolName:         tc.ToolName,
					Input:            tc.Input,
					ThoughtSignature: tc.ThoughtSignature, // Preserve Gemini thought signature
				}
			}
		}

		// Convert ToolResults from entity to port
		var toolResultParams []port.ToolResultParam
		if len(msg.ToolResults) > 0 {
			toolResultParams = make([]port.ToolResultParam, len(msg.ToolResults))
			for j, tr := range msg.ToolResults {
				toolResultParams[j] = port.ToolResultParam{
					ToolID:           tr.ToolID,
					Result:           tr.Result,
					IsError:          tr.IsError,
					ThoughtSignature: tr.ThoughtSignature, // Preserve Gemini thought signature
				}
			}
		}

		// Convert ThinkingBlocks from entity to port
		thinkingBlockParams := port.ConvertEntityThinkingBlocksToParams(msg.ThinkingBlocks)

		messageParams[i] = port.MessageParam{
			Role:           msg.Role,
			Content:        msg.Content,
			ToolCalls:      toolCallParams,
			ToolResults:    toolResultParams,
			ThinkingBlocks: thinkingBlockParams,
		}
	}

	// Get available tools
	tools, err := cs.toolExecutor.ListTools()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Filter tools based on active skills' allowed-tools restrictions
	filteredTools, err := cs.filterToolsByActiveSkills(sessionID, tools)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	toolParams := make([]port.ToolParam, len(filteredTools))
	for i, tool := range filteredTools {
		toolParams[i] = port.ToolParam{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	// Add plan mode info to context if enabled
	isPlanMode, _ := cs.IsPlanMode(sessionID)
	if isPlanMode {
		planInfo := port.PlanModeInfo{
			Enabled:   true,
			SessionID: sessionID,
			PlanPath:  fmt.Sprintf(".agent/plans/%s.md", sessionID),
		}
		ctx = port.WithPlanMode(ctx, planInfo)
	}

	// Add custom system prompt to context if set
	if customPrompt, ok := cs.GetCustomSystemPrompt(sessionID); ok {
		ctx = port.WithCustomSystemPrompt(ctx, port.CustomSystemPromptInfo{
			Prompt:    customPrompt,
			SessionID: sessionID,
		})
	}

	// Add thinking mode info to context if enabled
	if thinkingInfo, err := cs.GetThinkingMode(sessionID); err == nil && thinkingInfo.Enabled {
		ctx = port.WithThinkingMode(ctx, thinkingInfo)
	}

	return conversation, messageParams, toolParams, ctx, nil
}

// finalizeAIResponse adds the AI response to the conversation and updates processing state.
// This is shared logic between streaming and non-streaming requests.
func (cs *ConversationService) finalizeAIResponse(
	ctx context.Context,
	sessionID string,
	conversation *entity.Conversation,
	response *entity.Message,
	toolCalls []port.ToolCallInfo,
) (*entity.Message, []port.ToolCallInfo, error) {
	// Add response to conversation
	err := conversation.AddMessage(*response)
	if err != nil {
		return nil, nil, err
	}

	// Check if response contains tool usage
	hasToolCalls := len(toolCalls) > 0
	cs.processingMu.Lock()
	cs.processing[sessionID] = hasToolCalls
	cs.processingMu.Unlock()

	// Track token usage and check for compaction (atomic to avoid race between set and check)
	shouldCompact := cs.setTokensAndCheckThreshold(sessionID, response.InputTokens, response.OutputTokens)
	if !hasToolCalls && shouldCompact {
		if err := cs.compactConversation(ctx, sessionID, conversation); err != nil {
			fmt.Fprintf(
				os.Stderr,
				"[ConversationService] Auto-compaction failed for session %s (non-fatal, conversation continues): %v\n",
				sessionID,
				err,
			)
		}
	}

	return response, toolCalls, nil
}

// ExecuteToolsInResponse executes all tools requested in an assistant response.
func (cs *ConversationService) ExecuteToolsInResponse(
	ctx context.Context,
	sessionID string,
	assistantMessage *entity.Message,
) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	cs.conversationsMu.RLock()
	_, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return nil, ErrConversationNotFound
	}

	toolRequests := cs.parseToolRequests(assistantMessage.Content)
	results := make([]string, 0, len(toolRequests))

	for _, request := range toolRequests {
		// Enforce tool whitelisting
		if err := cs.ValidateToolAllowed(sessionID, request.Name); err != nil {
			results = append(results, fmt.Sprintf("tool blocked: %v", err))
			continue
		}

		_, found := cs.toolExecutor.GetTool(request.Name)
		if !found {
			results = append(results, "tool not found")
			continue
		}

		// Add sessionID to context so PlanningExecutorAdapter can check plan mode
		ctxWithSession := port.WithSessionID(ctx, sessionID)
		result, err := cs.toolExecutor.ExecuteTool(ctxWithSession, request.Name, request.Input)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	// Reset processing state after executing tools
	cs.processingMu.Lock()
	cs.processing[sessionID] = false
	cs.processingMu.Unlock()

	return results, nil
}

// GetConversation retrieves a conversation by session ID.
func (cs *ConversationService) GetConversation(sessionID string) (*entity.Conversation, error) {
	cs.conversationsMu.RLock()
	defer cs.conversationsMu.RUnlock()
	conversation, exists := cs.conversations[sessionID]
	if !exists {
		return nil, ErrConversationNotFound
	}
	return conversation, nil
}

// GetCurrentSession returns the current active session ID.
func (cs *ConversationService) GetCurrentSession() (string, error) {
	return cs.currentSession, nil
}

// EndConversation concludes a conversation session, performing cleanup if needed.
// It removes session-specific state including processing flags and mode settings.
func (cs *ConversationService) EndConversation(ctx context.Context, sessionID string) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	cs.conversationsMu.Lock()
	defer cs.conversationsMu.Unlock()

	_, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}

	// If ending current session, clear it
	if cs.currentSession == sessionID {
		cs.currentSession = ""
	}

	// Remove processing state
	cs.processingMu.Lock()
	delete(cs.processing, sessionID)
	cs.processingMu.Unlock()

	// Remove mode state
	cs.sessionModesMu.Lock()
	delete(cs.sessionModes, sessionID)
	cs.sessionModesMu.Unlock()

	// Remove thinking mode state
	cs.sessionThinkingModesMu.Lock()
	delete(cs.sessionThinkingModes, sessionID)
	cs.sessionThinkingModesMu.Unlock()

	// Remove custom system prompt
	cs.sessionSystemPromptsMu.Lock()
	delete(cs.sessionSystemPrompts, sessionID)
	cs.sessionSystemPromptsMu.Unlock()

	// Remove token usage tracking
	cs.sessionTokenUsageMu.Lock()
	delete(cs.sessionTokenUsage, sessionID)
	cs.sessionTokenUsageMu.Unlock()

	// Remove active skills
	cs.sessionActiveSkillsMu.Lock()
	delete(cs.sessionActiveSkills, sessionID)
	cs.sessionActiveSkillsMu.Unlock()

	// Remove cached allowed tools
	cs.sessionAllowedToolsMu.Lock()
	delete(cs.sessionAllowedTools, sessionID)
	cs.sessionAllowedToolsMu.Unlock()

	// Clean up session state in tool executor if it supports it
	if cleaner, ok := cs.toolExecutor.(port.SessionCleaner); ok {
		cleaner.CleanupSession(sessionID)
	}

	// Remove conversation from map to prevent memory leak
	delete(cs.conversations, sessionID)

	return nil
}

// IsProcessing checks if the conversation is currently processing (waiting for tool results).
func (cs *ConversationService) IsProcessing(sessionID string) (bool, error) {
	cs.conversationsMu.RLock()
	_, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return false, ErrConversationNotFound
	}
	cs.processingMu.RLock()
	defer cs.processingMu.RUnlock()
	return cs.processing[sessionID], nil
}

// SetProcessingState sets the processing state of a conversation.
func (cs *ConversationService) SetProcessingState(sessionID string, processing bool) error {
	cs.conversationsMu.RLock()
	_, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return ErrConversationNotFound
	}
	cs.processingMu.Lock()
	cs.processing[sessionID] = processing
	cs.processingMu.Unlock()
	return nil
}

// AppendActiveSkill appends a single skill to the active skills for a session.
// This method performs the read-modify-write atomically under a single lock to prevent race conditions.
func (cs *ConversationService) AppendActiveSkill(sessionID string, skill entity.Skill) error {
	cs.conversationsMu.RLock()
	defer cs.conversationsMu.RUnlock()

	_, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}

	cs.sessionActiveSkillsMu.Lock()
	defer cs.sessionActiveSkillsMu.Unlock()

	// Read existing skills
	existingSkills := cs.sessionActiveSkills[sessionID]

	// Dedup: skip if skill already active
	for _, s := range existingSkills {
		if s.Name == skill.Name {
			return nil
		}
	}

	// Create new slice with capacity for existing + 1 new skill
	allSkills := make([]entity.Skill, 0, len(existingSkills)+1)
	allSkills = append(allSkills, existingSkills...)
	allSkills = append(allSkills, skill)

	cs.sessionActiveSkills[sessionID] = allSkills

	// Compute and cache the allowed tools union
	allowedTools := cs.computeAllowedTools(allSkills)
	cs.sessionAllowedToolsMu.Lock()
	cs.sessionAllowedTools[sessionID] = allowedTools
	cs.sessionAllowedToolsMu.Unlock()

	return nil
}

// RemoveActiveSkill removes a skill from the active skills for a session.
// This method is idempotent - returns nil if skill not found or not active.
func (cs *ConversationService) RemoveActiveSkill(sessionID string, skillName string) error {
	cs.conversationsMu.RLock()
	defer cs.conversationsMu.RUnlock()

	_, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}

	cs.sessionActiveSkillsMu.Lock()
	defer cs.sessionActiveSkillsMu.Unlock()

	// Read existing skills
	existingSkills := cs.sessionActiveSkills[sessionID]
	if len(existingSkills) == 0 {
		return nil // No skills active, nothing to remove (idempotent)
	}

	// Filter out the skill with matching name
	filteredSkills := make([]entity.Skill, 0, len(existingSkills))
	for _, skill := range existingSkills {
		if skill.Name != skillName {
			filteredSkills = append(filteredSkills, skill)
		}
	}

	cs.sessionActiveSkills[sessionID] = filteredSkills

	// Recompute and cache the allowed tools union
	allowedTools := cs.computeAllowedTools(filteredSkills)
	cs.sessionAllowedToolsMu.Lock()
	cs.sessionAllowedTools[sessionID] = allowedTools
	cs.sessionAllowedToolsMu.Unlock()

	return nil
}

// SetActiveSkills sets the active skills for a session.
// These skills are used to determine which tools are allowed for execution.
// The allowed tools cache is recomputed when skills are set.
func (cs *ConversationService) SetActiveSkills(sessionID string, skills []entity.Skill) error {
	cs.conversationsMu.RLock()
	defer cs.conversationsMu.RUnlock()

	_, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}
	cs.sessionActiveSkillsMu.Lock()
	defer cs.sessionActiveSkillsMu.Unlock()

	cs.sessionActiveSkills[sessionID] = skills

	// Compute and cache the allowed tools union
	allowedTools := cs.computeAllowedTools(skills)
	cs.sessionAllowedToolsMu.Lock()
	cs.sessionAllowedTools[sessionID] = allowedTools
	cs.sessionAllowedToolsMu.Unlock()

	return nil
}

// computeAllowedTools computes the union of allowed tools from all active skills.
// Returns an empty map if no skills have restrictions (meaning all tools allowed).
//
// Important: Skills without allowed-tools fields do NOT relax restrictions imposed
// by other skills. If any skill has allowed-tools, only those tools are permitted.
// Example: skill A (allowed-tools: [read_file]) + skill B (no allowed-tools)
// results in only [read_file] being allowed, not all tools.
func (cs *ConversationService) computeAllowedTools(skills []entity.Skill) map[string]bool {
	allowedTools := make(map[string]bool)
	hasRestrictions := false
	for _, skill := range skills {
		if len(skill.AllowedTools) > 0 {
			hasRestrictions = true
			for _, tool := range skill.AllowedTools {
				allowedTools[tool] = true
			}
		}
	}
	if !hasRestrictions {
		return make(map[string]bool) // Empty map means all tools allowed
	}
	return allowedTools
}

// GetActiveSkills returns the active skills for a session.
// Returns nil if no skills are active for the session.
func (cs *ConversationService) GetActiveSkills(sessionID string) ([]entity.Skill, error) {
	cs.conversationsMu.RLock()
	defer cs.conversationsMu.RUnlock()

	_, exists := cs.conversations[sessionID]
	if !exists {
		return nil, ErrConversationNotFound
	}
	cs.sessionActiveSkillsMu.RLock()
	defer cs.sessionActiveSkillsMu.RUnlock()
	skills := cs.sessionActiveSkills[sessionID]
	if skills == nil {
		return nil, nil
	}
	result := make([]entity.Skill, len(skills))
	copy(result, skills)
	return result, nil
}

// GetAllowedToolsForSession returns the cached union of all allowed tools from active skills.
// Returns an empty map if no skills have allowed-tools restrictions (meaning all tools are allowed).
// The cache is maintained by SetActiveSkills for O(1) lookup.
func (cs *ConversationService) GetAllowedToolsForSession(sessionID string) (map[string]bool, error) {
	cs.conversationsMu.RLock()
	defer cs.conversationsMu.RUnlock()

	_, exists := cs.conversations[sessionID]
	if !exists {
		return nil, ErrConversationNotFound
	}

	cs.sessionAllowedToolsMu.RLock()
	defer cs.sessionAllowedToolsMu.RUnlock()

	// Return a copy to prevent external modification
	cached := cs.sessionAllowedTools[sessionID]
	result := make(map[string]bool, len(cached))
	maps.Copy(result, cached)
	return result, nil
}

// ValidateToolAllowed checks if a tool is allowed for the given session.
// Returns nil if allowed, or an error if blocked by skill restrictions.
func (cs *ConversationService) ValidateToolAllowed(sessionID string, toolName string) error {
	allowedTools, err := cs.GetAllowedToolsForSession(sessionID)
	if err != nil {
		return err
	}

	// If allowedTools is empty, no restrictions are in place
	if len(allowedTools) == 0 {
		return nil
	}

	if !allowedTools[toolName] {
		// Try to format the list of allowed tools for better error message
		return fmt.Errorf("tool '%s' is not in the allowed set for active skills", toolName)
	}
	return nil
}

// Helper methods for ConversationService

// generateSessionID generates a unique session ID using crypto/rand.
func generateSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Use fallback if crypto/rand fails (should be extremely rare)
		fmt.Fprintf(os.Stderr, "[ConversationService] CRITICAL: crypto/rand failed: %v\n", err)
	}
	return hex.EncodeToString(bytes)
}

// ToolRequest represents a parsed tool request from AI response.
type ToolRequest struct {
	Name  string      `json:"name"`
	Input interface{} `json:"input"`
}

// parseToolRequests parses tool requests from AI response content.
func (cs *ConversationService) parseToolRequests(content string) []ToolRequest {
	var requests []ToolRequest

	// Try parsing as single object
	var singleRequest ToolRequest
	if err := json.Unmarshal([]byte(content), &singleRequest); err == nil {
		if singleRequest.Name != "" {
			requests = append(requests, singleRequest)
		}
	}

	// Try parsing as array
	var arrayRequests []ToolRequest
	if err := json.Unmarshal([]byte(content), &arrayRequests); err == nil {
		requests = append(requests, arrayRequests...)
	}

	return requests
}

// SetPlanMode sets the plan mode state for a session.
// When plan mode is enabled, tool executions are written to plan files instead of being executed.
// The operation is thread-safe.
func (cs *ConversationService) SetPlanMode(sessionID string, enabled bool) error {
	cs.conversationsMu.RLock()
	_, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return ErrConversationNotFound
	}
	cs.sessionModesMu.Lock()
	cs.sessionModes[sessionID] = enabled
	cs.sessionModesMu.Unlock()
	return nil
}

// IsPlanMode returns whether plan mode is enabled for a session.
// Returns false for non-existent sessions.
// The operation is thread-safe for concurrent reads.
func (cs *ConversationService) IsPlanMode(sessionID string) (bool, error) {
	cs.conversationsMu.RLock()
	_, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return false, ErrConversationNotFound
	}
	cs.sessionModesMu.RLock()
	defer cs.sessionModesMu.RUnlock()
	return cs.sessionModes[sessionID], nil
}

// SetThinkingMode sets the extended thinking mode configuration for a session.
// The configuration includes whether thinking is enabled, the token budget, and display settings.
// The operation is thread-safe.
func (cs *ConversationService) SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error {
	cs.conversationsMu.RLock()
	_, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return ErrConversationNotFound
	}
	cs.sessionThinkingModesMu.Lock()
	cs.sessionThinkingModes[sessionID] = info
	cs.sessionThinkingModesMu.Unlock()
	return nil
}

// GetThinkingMode returns the extended thinking mode configuration for a session.
// Returns zero-value ThinkingModeInfo for non-existent sessions or if not set.
// The operation is thread-safe for concurrent reads.
func (cs *ConversationService) GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error) {
	cs.conversationsMu.RLock()
	_, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return port.ThinkingModeInfo{}, ErrConversationNotFound
	}
	cs.sessionThinkingModesMu.RLock()
	defer cs.sessionThinkingModesMu.RUnlock()
	return cs.sessionThinkingModes[sessionID], nil
}

// SetCustomSystemPrompt sets a custom system prompt for a session.
// This allows overriding the default AI system prompt with session-specific instructions.
// The custom prompt is included in the context when calling the AI provider.
// The operation is thread-safe.
func (cs *ConversationService) SetCustomSystemPrompt(ctx context.Context, sessionID, prompt string) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	cs.conversationsMu.RLock()
	_, exists := cs.conversations[sessionID]
	cs.conversationsMu.RUnlock()
	if !exists {
		return ErrConversationNotFound
	}
	cs.sessionSystemPromptsMu.Lock()
	cs.sessionSystemPrompts[sessionID] = prompt
	cs.sessionSystemPromptsMu.Unlock()
	return nil
}

// GetCustomSystemPrompt retrieves the custom system prompt for a session.
// Returns the prompt and true if set, or empty string and false if not set.
// Returns false for non-existent sessions (graceful handling).
// The operation is thread-safe for concurrent reads.
func (cs *ConversationService) GetCustomSystemPrompt(sessionID string) (string, bool) {
	cs.sessionSystemPromptsMu.RLock()
	defer cs.sessionSystemPromptsMu.RUnlock()
	prompt, ok := cs.sessionSystemPrompts[sessionID]
	return prompt, ok
}

// setTokenUsage stores the total token count (input+output) for a session.
// The API reports input_tokens as the full conversation size each time, so
// input+output represents the total conversation footprint after each response.
func (cs *ConversationService) setTokenUsage(sessionID string, input, output int64) {
	cs.sessionTokenUsageMu.Lock()
	defer cs.sessionTokenUsageMu.Unlock()
	cs.sessionTokenUsage[sessionID] = input + output
}

// setTokensAndCheckThreshold atomically sets token usage and checks whether
// the compaction threshold has been reached. This avoids a race condition
// between separate set and check calls.
func (cs *ConversationService) setTokensAndCheckThreshold(sessionID string, input, output int64) bool {
	cs.sessionTokenUsageMu.Lock()
	defer cs.sessionTokenUsageMu.Unlock()
	total := input + output
	cs.sessionTokenUsage[sessionID] = total
	return total >= cs.compactionThreshold
}

// getTokenUsage returns the current total token count for a session.
func (cs *ConversationService) getTokenUsage(sessionID string) int64 {
	cs.sessionTokenUsageMu.RLock()
	defer cs.sessionTokenUsageMu.RUnlock()
	return cs.sessionTokenUsage[sessionID]
}

// GetTokenUsage returns the current total token count and compaction threshold for a session.
func (cs *ConversationService) GetTokenUsage(sessionID string) (int64, int64) {
	return cs.getTokenUsage(sessionID), cs.compactionThreshold
}

// formatMessageForSummary writes a single message's content to the builder for summarization.
func formatMessageForSummary(sb *strings.Builder, msg entity.Message) {
	fmt.Fprintf(sb, "[%s]: ", msg.Role)
	if msg.Content != "" {
		sb.WriteString(truncateByRunes(msg.Content, maxSummaryContentRunes, "... (truncated)"))
	}
	formatToolCallsForSummary(sb, msg.ToolCalls)
	formatToolResultsForSummary(sb, msg.ToolResults)
	sb.WriteString("\n\n")
}

// formatToolCallsForSummary appends tool call names to the builder.
func formatToolCallsForSummary(sb *strings.Builder, toolCalls []entity.ToolCall) {
	if len(toolCalls) == 0 {
		return
	}
	sb.WriteString(" [Tool calls: ")
	for i, tc := range toolCalls {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(tc.ToolName)
	}
	sb.WriteString("]")
}

// formatToolResultsForSummary appends truncated tool results to the builder.
func formatToolResultsForSummary(sb *strings.Builder, toolResults []entity.ToolResult) {
	for _, tr := range toolResults {
		result := truncateByRunes(tr.Result, maxSummaryToolResultRunes, "...")
		fmt.Fprintf(sb, " [Tool result: %s]", result)
	}
}

// truncateByRunes truncates a string to maxRunes runes and appends a suffix if truncation occurred.
// Unlike byte-slice truncation, this is safe for multi-byte UTF-8 characters.
func truncateByRunes(s string, maxRunes int, suffix string) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + suffix
}

// filterToolsByActiveSkills filters the tool list based on active skills' allowed-tools restrictions.
// If no skills have allowed-tools restrictions, returns all tools.
// If skills have allowed-tools, returns only tools that are in the union of all allowed tools.
// Returns an error if the session is not found (fail-closed).
func (cs *ConversationService) filterToolsByActiveSkills(sessionID string, tools []entity.Tool) ([]entity.Tool, error) {
	allowedTools, err := cs.GetAllowedToolsForSession(sessionID)
	if err != nil {
		// If we can't get allowed tools, return error (fail-closed)
		return nil, err
	}

	// If allowedTools is empty, no restrictions are in place
	if len(allowedTools) == 0 {
		return tools, nil
	}

	// Filter tools to only include allowed ones
	filtered := make([]entity.Tool, 0, len(tools))
	for _, tool := range tools {
		if allowedTools[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered, nil
}

// buildSummaryRequest formats conversation messages into a prompt for the AI to summarize.
func (cs *ConversationService) buildSummaryRequest(messages []entity.Message) []port.MessageParam {
	var sb strings.Builder
	sb.WriteString("Please provide a detailed summary of the following conversation. ")
	sb.WriteString("Preserve key decisions, code changes, file paths, error messages, and any important context. ")
	sb.WriteString("The summary should allow the conversation to continue without losing critical information.\n\n")
	sb.WriteString("=== CONVERSATION TO SUMMARIZE ===\n\n")

	for _, msg := range messages {
		formatMessageForSummary(&sb, msg)
	}

	sb.WriteString("=== END CONVERSATION ===\n\n")
	sb.WriteString("Provide a comprehensive summary that captures all important details:")

	return []port.MessageParam{
		{
			Role:    entity.RoleUser,
			Content: sb.String(),
		},
	}
}

// compactConversation summarizes the conversation via an AI call and replaces history with the summary.
func (cs *ConversationService) compactConversation(
	ctx context.Context,
	sessionID string,
	conversation *entity.Conversation,
) error {
	messages := conversation.GetMessages()
	if len(messages) == 0 {
		return nil
	}

	summaryMessages := cs.buildSummaryRequest(messages)

	summaryResponse, _, err := cs.aiProvider.SendMessage(ctx, summaryMessages, nil)
	if err != nil {
		return fmt.Errorf("failed to generate conversation summary: %w", err)
	}

	conversation.Clear()

	summaryContent := "[CONVERSATION SUMMARY - Auto-compacted]\n\n" + summaryResponse.Content
	summaryMsg, err := entity.NewMessage(entity.RoleUser, summaryContent)
	if err != nil {
		return fmt.Errorf("failed to create summary message: %w", err)
	}

	if err := conversation.AddMessage(*summaryMsg); err != nil {
		return fmt.Errorf("failed to add summary message: %w", err)
	}

	// Reset token counter based on summary size
	cs.sessionTokenUsageMu.Lock()
	cs.sessionTokenUsage[sessionID] = summaryResponse.InputTokens + summaryResponse.OutputTokens
	cs.sessionTokenUsageMu.Unlock()

	fmt.Fprintf(
		os.Stderr,
		"[ConversationService] Auto-compacted conversation %s: %d messages → 1 summary message\n",
		sessionID,
		len(messages),
	)

	return nil
}
