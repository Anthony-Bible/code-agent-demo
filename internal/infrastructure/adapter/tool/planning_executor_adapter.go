package tool

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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

// PlanEntry represents a single tool execution plan written to the plan file.
type PlanEntry struct {
	SessionID string      `json:"session_id"`
	ToolName  string      `json:"tool_name"`
	Input     interface{} `json:"input"`
	Timestamp time.Time   `json:"timestamp"`
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
func (p *PlanningExecutorAdapter) SetPlanMode(sessionID string, enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessionModes[sessionID] = enabled
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

// ExecuteTool executes a tool, or writes a plan entry if in plan mode for mutating tools.
func (p *PlanningExecutorAdapter) ExecuteTool(ctx context.Context, name string, input interface{}) (string, error) {
	// Handle enter_plan_mode tool specially
	if name == "enter_plan_mode" {
		return p.handleEnterPlanMode(ctx, input)
	}

	// Get session ID from context (if available)
	sessionID := getSessionIDFromContext(ctx)

	// Check if we're in plan mode for this session
	if sessionID != "" && p.IsPlanMode(sessionID) && !isReadOnlyTool(name) {
		return p.writePlanEntry(sessionID, name, input)
	}

	// Otherwise, execute normally
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
	sessionID := getSessionIDFromContext(ctx)
	if sessionID != "" {
		p.SetPlanMode(sessionID, true)
	}

	return fmt.Sprintf(
		"Plan mode enabled. Reason: %s\n\nMutating tool executions will now be written to plan files instead of being executed directly. Use :mode normal to exit plan mode.",
		planInput.Reason,
	), nil
}

// writePlanEntry writes a tool execution plan to the plan file.
func (p *PlanningExecutorAdapter) writePlanEntry(sessionID, toolName string, input interface{}) (string, error) {
	// Create plans directory
	plansDir := filepath.Join(p.workingDir, ".agent", "plans")
	if err := os.MkdirAll(plansDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create plans directory: %w", err)
	}

	// Create plan entry
	entry := PlanEntry{
		SessionID: sessionID,
		ToolName:  toolName,
		Input:     input,
		Timestamp: time.Now(),
	}

	// Generate filename
	filename := fmt.Sprintf("%s_%d.json", sessionID, time.Now().UnixNano())
	planPath := filepath.Join(plansDir, filename)

	// Marshal to JSON
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal plan entry: %w", err)
	}

	// Write to file
	if err := os.WriteFile(planPath, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to write plan file: %w", err)
	}

	return fmt.Sprintf("[PLAN MODE] Tool execution '%s' written to plan file: %s", toolName, planPath), nil
}

// Context key for session ID.
type contextKey string

const sessionIDKey contextKey = "session_id"

// WithSessionID adds a session ID to the context.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// getSessionIDFromContext retrieves the session ID from context.
func getSessionIDFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value(sessionIDKey).(string); ok {
		return sessionID
	}
	return ""
}
