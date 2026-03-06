package tool

import (
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/infrastructure/adapter/file"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDeactivateSkill_WithCallback(t *testing.T) {
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()))
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
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()))

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
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()))
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
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()))

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
	adapter := NewExecutorAdapter(file.NewLocalFileManager(t.TempDir()))

	_, err := adapter.ExecuteTool(context.Background(), "deactivate_skill", `{"skill_name":"my-skill"}`)
	if err == nil {
		t.Fatal("expected error for missing session ID")
	}
	if !strings.Contains(err.Error(), "session ID") {
		t.Errorf("expected 'session ID' in error, got: %v", err)
	}
}
