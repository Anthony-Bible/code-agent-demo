package service

import (
	"context"
	"errors"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
)

func TestValidateToolAllowed(t *testing.T) {
	service, _ := NewConversationService(&mockAIProvider{}, &mockToolExecutor{})
	ctx := context.Background()
	sessionID, _ := service.StartConversation(ctx)

	// Test 1: No restrictions (empty allowed tools)
	t.Run("no restrictions", func(t *testing.T) {
		err := service.ValidateToolAllowed(sessionID, "any_tool")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	// Test 2: Active skill with restrictions
	t.Run("with restrictions", func(t *testing.T) {
		skill := entity.Skill{
			Name:         "restricted-skill",
			AllowedTools: []string{"allowed_tool"},
		}
		_ = service.SetActiveSkills(sessionID, []entity.Skill{skill})

		// Allowed tool
		err := service.ValidateToolAllowed(sessionID, "allowed_tool")
		if err != nil {
			t.Errorf("Expected no error for allowed tool, got: %v", err)
		}

		// Blocked tool
		err = service.ValidateToolAllowed(sessionID, "blocked_tool")
		if err == nil {
			t.Error("Expected error for blocked tool, got nil")
		} else {
			expectedMsg := "tool 'blocked_tool' is not allowed for active skills. Allowed tools: allowed_tool"
			if err.Error() != expectedMsg {
				t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
			}
		}
	})

	// Test 3: Multiple skills union
	t.Run("multiple skills union", func(t *testing.T) {
		skill1 := entity.Skill{
			Name:         "skill1",
			AllowedTools: []string{"tool1"},
		}
		skill2 := entity.Skill{
			Name:         "skill2",
			AllowedTools: []string{"tool2"},
		}
		_ = service.SetActiveSkills(sessionID, []entity.Skill{skill1, skill2})

		// Both tools allowed
		if err := service.ValidateToolAllowed(sessionID, "tool1"); err != nil {
			t.Errorf("tool1 should be allowed")
		}
		if err := service.ValidateToolAllowed(sessionID, "tool2"); err != nil {
			t.Errorf("tool2 should be allowed")
		}

		// Unrelated tool blocked
		if err := service.ValidateToolAllowed(sessionID, "tool3"); err == nil {
			t.Error("tool3 should be blocked")
		} else {
			// Check sorted order in error message
			expectedMsg := "tool 'tool3' is not allowed for active skills. Allowed tools: tool1, tool2"
			if err.Error() != expectedMsg {
				t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
			}
		}
	})

	// Test 4: Non-existent session
	t.Run("non-existent session", func(t *testing.T) {
		err := service.ValidateToolAllowed("fake-session", "any_tool")
		if !errors.Is(err, ErrConversationNotFound) {
			t.Errorf("Expected ErrConversationNotFound, got: %v", err)
		}
	})
}
