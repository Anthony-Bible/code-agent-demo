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
