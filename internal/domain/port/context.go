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
