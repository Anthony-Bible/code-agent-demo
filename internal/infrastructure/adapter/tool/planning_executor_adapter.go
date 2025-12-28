package tool

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PlanModeConfirmCallback is called when the agent wants to enter plan mode.
// It receives the reason and returns true if the user approves.
type PlanModeConfirmCallback func(reason string) bool

// PlanningExecutorAdapter is a decorator that wraps a ToolExecutor and adds plan mode support.
// In plan mode, mutating tool executions are written to plan files instead of being executed.
// Read-only tools (read_file, list_files) are still executed normally.
type PlanningExecutorAdapter struct {
	baseExecutor                *ExecutorAdapter
	fileManager                 port.FileManager
	workingDir                  string
	sessionModes                map[string]bool // sessionID -> isPlanMode
	mu                          sync.RWMutex
	planModeConfirmCallback     PlanModeConfirmCallback
	commandConfirmationCallback CommandConfirmationCallback
}

// NewPlanningExecutorAdapter creates a new PlanningExecutorAdapter wrapping the given base executor.
func NewPlanningExecutorAdapter(
	baseExecutor *ExecutorAdapter,
	fileManager port.FileManager,
	workingDir string,
) *PlanningExecutorAdapter {
	return &PlanningExecutorAdapter{
		baseExecutor: baseExecutor,
		fileManager:  fileManager,
		workingDir:   workingDir,
		sessionModes: make(map[string]bool),
	}
}

// SetPlanModeConfirmCallback sets the callback for plan mode confirmation.
func (p *PlanningExecutorAdapter) SetPlanModeConfirmCallback(cb PlanModeConfirmCallback) {
	p.planModeConfirmCallback = cb
}

// SetCommandConfirmationCallback sets the callback for command confirmation on the base executor.
func (p *PlanningExecutorAdapter) SetCommandConfirmationCallback(cb CommandConfirmationCallback) {
	p.commandConfirmationCallback = cb
	p.baseExecutor.SetCommandConfirmationCallback(cb)
}

// SetPlanMode sets the plan mode for a given session.
// When enabling plan mode, it also creates the plans directory.
func (p *PlanningExecutorAdapter) SetPlanMode(sessionID string, enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessionModes[sessionID] = enabled

	// Create plans directory when enabling plan mode
	if enabled {
		plansDir := filepath.Join(p.workingDir, ".agent", "plans")
		_ = os.MkdirAll(plansDir, 0o750)
	}
}

// IsPlanMode returns whether plan mode is enabled for a given session.
func (p *PlanningExecutorAdapter) IsPlanMode(sessionID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessionModes[sessionID]
}

// RegisterTool delegates to the base executor.
func (p *PlanningExecutorAdapter) RegisterTool(tool entity.Tool) error {
	return p.baseExecutor.RegisterTool(tool)
}

// UnregisterTool delegates to the base executor.
func (p *PlanningExecutorAdapter) UnregisterTool(name string) error {
	return p.baseExecutor.UnregisterTool(name)
}

// ListTools delegates to the base executor.
func (p *PlanningExecutorAdapter) ListTools() ([]entity.Tool, error) {
	return p.baseExecutor.ListTools()
}

// GetTool delegates to the base executor.
func (p *PlanningExecutorAdapter) GetTool(name string) (entity.Tool, bool) {
	return p.baseExecutor.GetTool(name)
}

// ValidateToolInput delegates to the base executor.
func (p *PlanningExecutorAdapter) ValidateToolInput(name string, input interface{}) error {
	return p.baseExecutor.ValidateToolInput(name, input)
}

// isReadOnlyTool returns true if the tool is read-only and should always execute.
func isReadOnlyTool(name string) bool {
	readOnlyTools := map[string]bool{
		"read_file":  true,
		"list_files": true,
	}
	return readOnlyTools[name]
}

// ExecuteTool executes a tool, or blocks it if in plan mode and not allowed.
func (p *PlanningExecutorAdapter) ExecuteTool(ctx context.Context, name string, input interface{}) (string, error) {
	// Handle enter_plan_mode tool specially
	if name == "enter_plan_mode" {
		return p.handleEnterPlanMode(ctx, input)
	}

	// Get session ID from context (if available)
	sessionID, _ := port.SessionIDFromContext(ctx)

	// Check if we're in plan mode for this session
	if sessionID != "" && p.IsPlanMode(sessionID) {
		if !p.isAllowedInPlanMode(name, input) {
			return p.getPlanBlockedMessage(sessionID, name), nil
		}
	}

	// Execute normally (either not in plan mode, or tool is allowed in plan mode)
	return p.baseExecutor.ExecuteTool(ctx, name, input)
}

// handleEnterPlanMode handles the enter_plan_mode tool execution.
func (p *PlanningExecutorAdapter) handleEnterPlanMode(ctx context.Context, input interface{}) (string, error) {
	// Parse input to get reason
	var planInput struct {
		Reason string `json:"reason"`
	}

	switch v := input.(type) {
	case json.RawMessage:
		if err := json.Unmarshal(v, &planInput); err != nil {
			return "", fmt.Errorf("failed to parse enter_plan_mode input: %w", err)
		}
	case map[string]interface{}:
		if reason, ok := v["reason"].(string); ok {
			planInput.Reason = reason
		}
	case string:
		if err := json.Unmarshal([]byte(v), &planInput); err != nil {
			return "", fmt.Errorf("failed to parse enter_plan_mode input: %w", err)
		}
	}

	// Call confirmation callback if set
	if p.planModeConfirmCallback != nil {
		if !p.planModeConfirmCallback(planInput.Reason) {
			return "Plan mode request denied by user", nil
		}
	}

	// Get session ID and enable plan mode
	sessionID, _ := port.SessionIDFromContext(ctx)
	if sessionID != "" {
		p.SetPlanMode(sessionID, true)
	}

	return fmt.Sprintf(
		"Plan mode enabled. Reason: %s\n\nMutating tool executions will now be written to plan files instead of being executed directly. Use :mode normal to exit plan mode.",
		planInput.Reason,
	), nil
}

// isAllowedInPlanMode checks if a tool execution is allowed in plan mode.
// Allows read-only tools and writes to the plan file (.agent/plans/*.md).
func (p *PlanningExecutorAdapter) isAllowedInPlanMode(name string, input interface{}) bool {
	if isReadOnlyTool(name) {
		return true
	}

	// Allow edit_file to .agent/plans/*.md
	if name == "edit_file" {
		return p.isPlanFileEdit(input)
	}

	return false
}

// isPlanFileEdit checks if an edit_file input targets a plan file.
// Plan files are identified by having ".agent/plans/" in the path and ending with ".md".
func (p *PlanningExecutorAdapter) isPlanFileEdit(input interface{}) bool {
	var editInput struct {
		Path string `json:"path"`
	}

	data, err := json.Marshal(input)
	if err != nil {
		return false
	}

	if err := json.Unmarshal(data, &editInput); err != nil {
		return false
	}

	// Check if path contains .agent/plans/ and ends with .md
	// This handles both relative paths (.agent/plans/x.md) and
	// absolute paths (/tmp/xxx/.agent/plans/x.md)
	return strings.Contains(editInput.Path, ".agent/plans/") &&
		strings.HasSuffix(editInput.Path, ".md")
}

// getPlanBlockedMessage returns a message telling the agent to write to the plan file instead.
func (p *PlanningExecutorAdapter) getPlanBlockedMessage(sessionID, toolName string) string {
	planPath := fmt.Sprintf(".agent/plans/%s.md", sessionID)
	return fmt.Sprintf(
		"[PLAN MODE] Tool '%s' is blocked in plan mode. Write your planned changes to %s instead using edit_file.",
		toolName, planPath,
	)
}
