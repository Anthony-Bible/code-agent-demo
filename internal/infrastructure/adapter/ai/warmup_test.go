package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// fakeProvider is a port.AIProvider implementation that records SendMessage
// calls and returns a configurable error. Only the methods exercised by the
// warmup path have meaningful behavior; the rest exist to satisfy the
// interface.
type fakeProvider struct {
	sendErr   error
	sendCalls int
}

func (f *fakeProvider) SendMessage(
	_ context.Context,
	_ []port.MessageParam,
	_ []port.ToolParam,
	_ port.AIRequestOptions,
) (*entity.Message, []port.ToolCallInfo, error) {
	f.sendCalls++
	if f.sendErr != nil {
		return nil, nil, f.sendErr
	}
	return &entity.Message{}, nil, nil
}

func (f *fakeProvider) SendMessageStreaming(
	_ context.Context,
	_ []port.MessageParam,
	_ []port.ToolParam,
	_ port.AIRequestOptions,
	_ port.StreamCallback,
	_ port.ThinkingCallback,
) (*entity.Message, []port.ToolCallInfo, error) {
	return &entity.Message{}, nil, nil
}

func (f *fakeProvider) GenerateToolSchema() port.ToolInputSchemaParam {
	return make(port.ToolInputSchemaParam)
}

func (f *fakeProvider) HealthCheck(_ context.Context) error { return nil }
func (f *fakeProvider) SetModel(_ string) error             { return nil }
func (f *fakeProvider) GetModel() string                    { return "" }
func (f *fakeProvider) Clone() port.AIProvider              { return f }

// fakeExecutor returns a configurable tool list and error from ListTools.
// Other ToolExecutor methods are stubbed.
type fakeExecutor struct {
	tools   []entity.Tool
	listErr error
}

func (f *fakeExecutor) RegisterTool(_ entity.Tool) error { return nil }
func (f *fakeExecutor) UnregisterTool(_ string) error    { return nil }
func (f *fakeExecutor) ExecuteTool(_ context.Context, _ string, _ interface{}) (string, error) {
	return "", nil
}

func (f *fakeExecutor) ListTools() ([]entity.Tool, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tools, nil
}
func (f *fakeExecutor) GetTool(_ string) (entity.Tool, bool)            { return entity.Tool{}, false }
func (f *fakeExecutor) ValidateToolInput(_ string, _ interface{}) error { return nil }

func TestWarmCache_NilProvider_ReturnsError(t *testing.T) {
	t.Parallel()
	tools := []port.ToolParam{{Name: "x"}}
	err := WarmCache(context.Background(), nil, tools, port.NopLogger{})
	if err == nil {
		t.Fatal("expected error for nil provider, got nil")
	}
}

func TestWarmCache_NoTools_SkipsWithoutCallingProvider(t *testing.T) {
	t.Parallel()
	p := &fakeProvider{}
	if err := WarmCache(context.Background(), p, nil, port.NopLogger{}); err != nil {
		t.Fatalf("expected nil error on empty tool catalog, got %v", err)
	}
	if p.sendCalls != 0 {
		t.Errorf("expected SendMessage to be skipped, but it was called %d times", p.sendCalls)
	}
}

func TestWarmCache_ProviderError_IsWrapped(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("upstream unavailable")
	p := &fakeProvider{sendErr: sentinel}
	tools := []port.ToolParam{{Name: "ping"}}

	err := WarmCache(context.Background(), p, tools, port.NopLogger{})
	if err == nil {
		t.Fatal("expected error from provider, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
	if p.sendCalls != 1 {
		t.Errorf("expected exactly 1 SendMessage call, got %d", p.sendCalls)
	}
}

func TestWarmCache_HappyPath_CallsProviderOnce(t *testing.T) {
	t.Parallel()
	p := &fakeProvider{}
	tools := []port.ToolParam{{Name: "ping"}}
	if err := WarmCache(context.Background(), p, tools, port.NopLogger{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.sendCalls != 1 {
		t.Errorf("expected exactly 1 SendMessage call, got %d", p.sendCalls)
	}
}

func TestWarmCache_NilLogger_DoesNotPanic(t *testing.T) {
	t.Parallel()
	p := &fakeProvider{}
	tools := []port.ToolParam{{Name: "ping"}}
	if err := WarmCache(context.Background(), p, tools, nil); err != nil {
		t.Fatalf("unexpected error with nil logger: %v", err)
	}
}

func TestToolParamsFromExecutor_NilExecutor_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := ToolParamsFromExecutor(nil)
	if err == nil {
		t.Fatal("expected error for nil executor, got nil")
	}
}

func TestToolParamsFromExecutor_ListToolsError_IsWrapped(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("disk fell off the truck")
	exec := &fakeExecutor{listErr: sentinel}
	_, err := ToolParamsFromExecutor(exec)
	if err == nil {
		t.Fatal("expected error from ListTools, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

func TestToolParamsFromExecutor_ConvertsAllTools(t *testing.T) {
	t.Parallel()
	exec := &fakeExecutor{
		tools: []entity.Tool{
			{Name: "a", Description: "tool a", Strict: true},
			{Name: "b", Description: "tool b", Strict: false},
		},
	}
	got, err := ToolParamsFromExecutor(exec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 ToolParams, got %d", len(got))
	}
	if got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("tool order not preserved: %+v", got)
	}
	if !got[0].Strict || got[1].Strict {
		t.Errorf("Strict flag not propagated: %+v", got)
	}
}
