package tool

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/file"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/logger"
)

func TestDeactivateSkill_WithCallback(t *testing.T) {
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()), logger.NewNop())
	var calledSession, calledSkill string
	adapter.SetSkillDeactivationCallback(func(sessionID, skillName string) error {
		calledSession = sessionID
		calledSkill = skillName
		return nil
	})

	ctx := port.WithSessionID(context.Background(), "sess-1")
	result, err := adapter.ExecuteTool(ctx, "deactivate_skill", `{"skill_name":"my-skill"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "deactivated successfully") {
		t.Errorf("expected success message, got: %s", result)
	}
	if calledSession != "sess-1" || calledSkill != "my-skill" {
		t.Errorf("callback args: session=%q skill=%q", calledSession, calledSkill)
	}
}

func TestDeactivateSkill_WithoutCallback(t *testing.T) {
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()), logger.NewNop())

	ctx := port.WithSessionID(context.Background(), "sess-1")
	result, err := adapter.ExecuteTool(ctx, "deactivate_skill", `{"skill_name":"my-skill"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "deactivated successfully") {
		t.Errorf("expected success message, got: %s", result)
	}
}

func TestDeactivateSkill_CallbackError(t *testing.T) {
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()), logger.NewNop())
	adapter.SetSkillDeactivationCallback(func(_, _ string) error {
		return errors.New("storage failure")
	})

	ctx := port.WithSessionID(context.Background(), "sess-1")
	_, err := adapter.ExecuteTool(ctx, "deactivate_skill", `{"skill_name":"my-skill"}`)
	if err == nil {
		t.Fatal("expected error from callback")
	}
	if !strings.Contains(err.Error(), "storage failure") {
		t.Errorf("expected 'storage failure' in error, got: %v", err)
	}
}

func TestDeactivateSkill_EmptySkillName(t *testing.T) {
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()), logger.NewNop())

	ctx := port.WithSessionID(context.Background(), "sess-1")
	_, err := adapter.ExecuteTool(ctx, "deactivate_skill", `{"skill_name":""}`)
	if err == nil {
		t.Fatal("expected error for empty skill_name")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}

func TestDeactivateSkill_MissingSessionID(t *testing.T) {
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()), logger.NewNop())

	_, err := adapter.ExecuteTool(context.Background(), "deactivate_skill", `{"skill_name":"my-skill"}`)
	if err == nil {
		t.Fatal("expected error for missing session ID")
	}
	if !strings.Contains(err.Error(), "session ID") {
		t.Errorf("expected 'session ID' in error, got: %v", err)
	}
}
