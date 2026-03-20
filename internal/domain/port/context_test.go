package port

import (
	"context"
	"testing"
)

// TestWithSessionID_SetAndRetrieve verifies that session ID
// can be stored in context and retrieved correctly.
func TestWithSessionID_SetAndRetrieve(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		expectOk  bool
	}{
		{
			name:      "valid session ID",
			sessionID: "test-session-123",
			expectOk:  true,
		},
		{
			name:      "empty session ID",
			sessionID: "",
			expectOk:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctxWithSession := WithSessionID(ctx, tt.sessionID)

			retrievedID, ok := SessionIDFromContext(ctxWithSession)

			if ok != tt.expectOk {
				t.Errorf("expected ok=%v, got ok=%v", tt.expectOk, ok)
			}
			if ok && retrievedID != tt.sessionID {
				t.Errorf("expected sessionID=%q, got %q", tt.sessionID, retrievedID)
			}
		})
	}
}

// TestSessionIDFromContext_Missing verifies that retrieving session ID
// from a context without one returns false.
func TestSessionIDFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	_, ok := SessionIDFromContext(ctx)

	if ok {
		t.Error("expected ok=false for context without session ID")
	}
}

// TestSubagentContext_SetAndRetrieve verifies that SubagentContextInfo
// can be stored in context and retrieved correctly.
func TestSubagentContext_SetAndRetrieve(t *testing.T) {
	info := SubagentContextInfo{
		SubagentID:      "sub-1",
		ParentSessionID: "parent-1",
		IsSubagent:      true,
		Depth:           1,
	}

	ctx := context.Background()
	ctxWithSubagent := WithSubagentContext(ctx, info)

	retrievedInfo, ok := SubagentContextFromContext(ctxWithSubagent)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}

	if retrievedInfo != info {
		t.Errorf("expected %+v, got %+v", info, retrievedInfo)
	}

	if !IsSubagentContext(ctxWithSubagent) {
		t.Error("expected IsSubagentContext to be true")
	}
}

// TestSubagentContextFromContext_Missing verifies that retrieving subagent context
// from a context without one returns false.
func TestSubagentContextFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	_, ok := SubagentContextFromContext(ctx)

	if ok {
		t.Error("expected ok=false for context without subagent context")
	}

	if IsSubagentContext(ctx) {
		t.Error("expected IsSubagentContext to be false")
	}
}
