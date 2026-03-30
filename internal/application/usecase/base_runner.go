package usecase

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// conversationCleanupTimeout is the maximum time allowed for ending a conversation
// during cleanup, using a background context so it succeeds even if the parent
// context was cancelled.
const conversationCleanupTimeout = 5 * time.Second

// ToolPermissionChecker determines whether a tool call is allowed.
// Implementations wrap different permission models (SafetyEnforcer, AllowedTools list).
type ToolPermissionChecker interface {
	IsToolCallAllowed(tc port.ToolCallInfo) bool
}

// ToolCallExecutor is called for each allowed tool call. It wraps runner-specific
// pre-execution logic (safety checks, recursion prevention, display) around the
// base ExecuteToolCall.
type ToolCallExecutor func(ctx context.Context, tc port.ToolCallInfo) entity.ToolResult

// BaseRunContext holds state common to all runner execution contexts.
type BaseRunContext struct {
	Ctx          context.Context
	SessionID    string
	StartTime    time.Time
	ActionsTaken int
	MaxActions   int
}

// BaseRunner provides shared infrastructure for AI-driven execution loops.
// Both InvestigationRunner and SubagentRunner embed this struct.
type BaseRunner struct {
	ConvService       ConversationServiceInterface
	ToolExecutor      port.ToolExecutor
	PermissionChecker ToolPermissionChecker
	logger            port.Logger
}

// newBaseRunner creates a BaseRunner with the given dependencies.
func newBaseRunner(convService ConversationServiceInterface, toolExecutor port.ToolExecutor, permChecker ToolPermissionChecker, logger port.Logger) BaseRunner {
	return BaseRunner{
		ConvService:       convService,
		ToolExecutor:      toolExecutor,
		PermissionChecker: permChecker,
		logger:            logger,
	}
}

// newSafetyPermissionChecker creates a ToolPermissionChecker from a SafetyEnforcer.
// If enforcer is nil, returns an AllowAllPermissionChecker.
func newSafetyPermissionChecker(enforcer SafetyEnforcer) ToolPermissionChecker {
	if enforcer == nil {
		return &AllowAllPermissionChecker{}
	}
	return &SafetyEnforcerPermissionChecker{Enforcer: enforcer}
}

// log returns the injected logger, or a NopLogger if none was provided.
// This protects against tests that create BaseRunner via struct literals.
func (b *BaseRunner) log() port.Logger {
	return port.FirstOrNop(b.logger)
}

// CleanupConversation ends a conversation using a background context
// so cleanup succeeds even if the parent context was cancelled.
func (b *BaseRunner) CleanupConversation(sessionID, entityID, entityLabel string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), conversationCleanupTimeout)
	defer cancel()
	if err := b.ConvService.EndConversation(cleanupCtx, sessionID); err != nil {
		b.log().Error("failed to end conversation",
			"session_id", sessionID,
			entityLabel, entityID,
			"error", err,
		)
	}
}

// ExecuteToolCall executes a single tool call and returns the result.
// This is the pure execution without any safety or permission checks.
func (b *BaseRunner) ExecuteToolCall(ctx context.Context, tc port.ToolCallInfo) entity.ToolResult {
	result, execErr := b.ToolExecutor.ExecuteTool(ctx, tc.ToolName, tc.Input)
	if execErr != nil {
		return entity.ToolResult{ToolID: tc.ToolID, Result: execErr.Error(), IsError: true}
	}
	return entity.ToolResult{ToolID: tc.ToolID, Result: result, IsError: false}
}

// ProcessToolCalls executes tool calls and feeds results back to the conversation.
// Blocked tools return error results without counting toward the action limit.
// The executor function wraps runner-specific logic around each tool execution.
func (b *BaseRunner) ProcessToolCalls(
	rc *BaseRunContext,
	toolCalls []port.ToolCallInfo,
	blockMessage string,
	executor ToolCallExecutor,
) error {
	var toolResults []entity.ToolResult
	for _, tc := range toolCalls {
		if !b.PermissionChecker.IsToolCallAllowed(tc) {
			toolResults = append(toolResults, entity.ToolResult{
				ToolID:  tc.ToolID,
				Result:  fmt.Sprintf("tool '%s' %s", tc.ToolName, blockMessage),
				IsError: true,
			})
			continue
		}
		toolResults = append(toolResults, executor(rc.Ctx, tc))
		rc.ActionsTaken++
	}
	if len(toolResults) > 0 {
		return b.ConvService.AddToolResultMessage(rc.Ctx, rc.SessionID, toolResults)
	}
	return nil
}

// InjectTurnWarningIfNeeded injects a warning message if approaching the action limit.
func (b *BaseRunner) InjectTurnWarningIfNeeded(rc *BaseRunContext, cfg TurnWarningConfig) {
	remaining := rc.MaxActions - rc.ActionsTaken
	warningMsg := BuildTurnWarningMessage(remaining, cfg)
	if warningMsg != "" {
		if _, err := b.ConvService.AddUserMessage(rc.Ctx, rc.SessionID, warningMsg); err != nil {
			b.log().Error("failed to add warning message", "error", err)
		}
	}
}

// LimitToolCalls caps tool calls to the remaining action budget.
func (b *BaseRunner) LimitToolCalls(rc *BaseRunContext, toolCalls []port.ToolCallInfo) []port.ToolCallInfo {
	remaining := rc.MaxActions - rc.ActionsTaken
	if remaining <= 0 {
		return nil
	}
	if len(toolCalls) > remaining {
		return toolCalls[:remaining]
	}
	return toolCalls
}

// SafetyEnforcerPermissionChecker adapts SafetyEnforcer to ToolPermissionChecker.
type SafetyEnforcerPermissionChecker struct {
	Enforcer SafetyEnforcer
}

// IsToolCallAllowed checks if the tool is allowed by the safety enforcer.
// The Enforcer is guaranteed non-nil by newSafetyPermissionChecker.
func (s *SafetyEnforcerPermissionChecker) IsToolCallAllowed(tc port.ToolCallInfo) bool {
	return s.Enforcer.CheckToolAllowed(tc.ToolName) == nil
}

// AllowedListPermissionChecker checks against a whitelist of tool names.
// A nil list allows all tools. An empty list blocks all tools.
type AllowedListPermissionChecker struct {
	AllowedTools []string
}

// IsToolCallAllowed checks if the tool is in the allowed list.
func (a *AllowedListPermissionChecker) IsToolCallAllowed(tc port.ToolCallInfo) bool {
	if a.AllowedTools == nil {
		return true
	}
	return slices.Contains(a.AllowedTools, tc.ToolName)
}

// AllowAllPermissionChecker allows all tool calls unconditionally.
type AllowAllPermissionChecker struct{}

// IsToolCallAllowed always returns true.
func (a *AllowAllPermissionChecker) IsToolCallAllowed(_ port.ToolCallInfo) bool {
	return true
}
