package cmd

import (
	"context"
	"testing"
)

func TestHandleSkillsCommand_Prefix(t *testing.T) {
	// We only want to test the prefix matching logic.
	// Since handleSkillsCommand is unexported, we can access it from the same package.
	// We pass nil for dependencies because we expect the function to return false early
	// for invalid prefixes, before using any dependencies.
	
	tests := []struct {
		name     string
		cmdText  string
		expected bool // true = handled (or would try to handle), false = not handled
	}{
		{
			name:     "exact match",
			cmdText:  ":skills",
			expected: true,
		},
		{
			name:     "match with subcommand",
			cmdText:  ":skills list",
			expected: true,
		},
		{
			name:     "match with extra spaces",
			cmdText:  ":skills   list",
			expected: true,
		},
		{
			name:     "no match",
			cmdText:  "skills",
			expected: false,
		},
		{
			name:     "match prefix but not word boundary (the bug)",
			cmdText:  ":skillsomething",
			expected: false,
		},
		{
			name:     "match prefix but not word boundary 2",
			cmdText:  ":skillsreset",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				// recover from panic if dependencies are used (meaning it passed the prefix check)
				if r := recover(); r != nil {
					// If we expected true, a panic means we passed the prefix check and tried to use nil service.
					// This is actually a success for the prefix check part.
					if !tt.expected {
						t.Errorf("function panicked but we expected false (not handled): %v", r)
					}
				}
			}()

			// We use a cancellable context to ensure we don't hang if it somehow tries to run something async (unlikely here)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Call the function with nil dependencies
			result := handleSkillsCommand(ctx, "session-id", tt.cmdText, nil, nil)

			if result != tt.expected {
				// If we expected true but got false, that's a failure.
				// If we expected false but got true (and didn't panic?), that's a failure.
				// Note: if expected=true, it will likely panic before returning, so we catch that in defer.
				// If it returns without panic, check the result.
				t.Errorf("handleSkillsCommand(%q) returned %v, expected %v", tt.cmdText, result, tt.expected)
			}
		})
	}
}
