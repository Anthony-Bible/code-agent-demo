package port

import "context"

// sessionIDKey is the key for storing session ID in context.
type sessionIDKey struct{}

// WithSessionID adds a session ID to the context.
// This allows passing session information through the ToolExecutor interface
// without modifying the interface signature.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// SessionIDFromContext retrieves the session ID from the context.
// Returns the session ID and a boolean indicating if it was found.
func SessionIDFromContext(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(sessionIDKey{}).(string)
	return sessionID, ok
}

// planModeKey is the key for storing plan mode state in context.
type planModeKey struct{}

// PlanModeInfo contains plan mode configuration for the AI.
type PlanModeInfo struct {
	Enabled   bool
	SessionID string
	PlanPath  string // e.g., ".agent/plans/{session}.md"
}

// WithPlanMode adds plan mode info to the context.
func WithPlanMode(ctx context.Context, info PlanModeInfo) context.Context {
	return context.WithValue(ctx, planModeKey{}, info)
}

// PlanModeFromContext retrieves plan mode info from the context.
// Returns the plan mode info and a boolean indicating if it was found.
func PlanModeFromContext(ctx context.Context) (PlanModeInfo, bool) {
	info, ok := ctx.Value(planModeKey{}).(PlanModeInfo)
	return info, ok
}

// customSystemPromptKey is the key for storing custom system prompt state in context.
type customSystemPromptKey struct{}

// CustomSystemPromptInfo contains custom system prompt configuration for the AI.
// This allows sessions to override the default system prompt with custom instructions.
type CustomSystemPromptInfo struct {
	SessionID string // Session this prompt applies to
	Prompt    string // Custom system prompt text to use instead of default
}

// WithCustomSystemPrompt adds custom system prompt info to the context.
// This allows passing session-specific system prompt overrides through the call chain
// without modifying interface signatures.
func WithCustomSystemPrompt(ctx context.Context, info CustomSystemPromptInfo) context.Context {
	return context.WithValue(ctx, customSystemPromptKey{}, info)
}

// CustomSystemPromptFromContext retrieves custom system prompt info from the context.
// Returns the custom system prompt info and a boolean indicating if it was found.
func CustomSystemPromptFromContext(ctx context.Context) (CustomSystemPromptInfo, bool) {
	info, ok := ctx.Value(customSystemPromptKey{}).(CustomSystemPromptInfo)
	return info, ok
}
