package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MockSubagentManager implements port.SubagentManager for testing.
type MockSubagentManager struct {
	DiscoverAgentsFunc    func(ctx context.Context) (*port.SubagentDiscoveryResult, error)
	LoadAgentMetadataFunc func(ctx context.Context, agentName string) (*entity.Subagent, error)
	RegisterAgentFunc     func(ctx context.Context, agent *entity.Subagent) error
	UnregisterAgentFunc   func(ctx context.Context, agentName string) error
	GetAgentByNameFunc    func(ctx context.Context, agentName string) (*port.SubagentInfo, error)
	ListAgentsFunc        func(ctx context.Context) ([]port.SubagentInfo, error)
}

func (m *MockSubagentManager) DiscoverAgents(ctx context.Context) (*port.SubagentDiscoveryResult, error) {
	if m.DiscoverAgentsFunc != nil {
		return m.DiscoverAgentsFunc(ctx)
	}
	return &port.SubagentDiscoveryResult{}, nil
}

func (m *MockSubagentManager) LoadAgentMetadata(ctx context.Context, agentName string) (*entity.Subagent, error) {
	if m.LoadAgentMetadataFunc != nil {
		return m.LoadAgentMetadataFunc(ctx, agentName)
	}
	return nil, errors.New("agent not found")
}

func (m *MockSubagentManager) RegisterAgent(ctx context.Context, agent *entity.Subagent) error {
	if m.RegisterAgentFunc != nil {
		return m.RegisterAgentFunc(ctx, agent)
	}
	return nil
}

func (m *MockSubagentManager) UnregisterAgent(ctx context.Context, agentName string) error {
	if m.UnregisterAgentFunc != nil {
		return m.UnregisterAgentFunc(ctx, agentName)
	}
	return nil
}

func (m *MockSubagentManager) GetAgentByName(ctx context.Context, agentName string) (*port.SubagentInfo, error) {
	if m.GetAgentByNameFunc != nil {
		return m.GetAgentByNameFunc(ctx, agentName)
	}
	return nil, errors.New("agent not found")
}

func (m *MockSubagentManager) ListAgents(ctx context.Context) ([]port.SubagentInfo, error) {
	if m.ListAgentsFunc != nil {
		return m.ListAgentsFunc(ctx)
	}
	return nil, nil
}

// MockSubagentRunner mocks the SubagentRunner for testing.
type MockSubagentRunner struct {
	RunFunc func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error)
}

func (m *MockSubagentRunner) Run(
	ctx context.Context,
	agent *entity.Subagent,
	taskPrompt string,
	subagentID string,
) (*SubagentResult, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, agent, taskPrompt, subagentID)
	}
	return nil, errors.New("not implemented")
}

// ==================== Constructor Tests ====================

func TestNewSubagentUseCase_ValidDependencies(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}

	uc := NewSubagentUseCase(manager, runner)

	if uc == nil {
		t.Fatal("NewSubagentUseCase() returned nil with valid dependencies")
	}
}

func TestNewSubagentUseCase_NilSubagentManager(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSubagentUseCase() did not panic with nil SubagentManager")
		}
	}()

	runner := &MockSubagentRunner{}
	NewSubagentUseCase(nil, runner)
}

func TestNewSubagentUseCase_NilSubagentRunner(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSubagentUseCase() did not panic with nil SubagentRunner")
		}
	}()

	manager := &MockSubagentManager{}
	NewSubagentUseCase(manager, nil)
}

// ==================== Input Validation Tests ====================

func TestSpawnSubagent_EmptyAgentName(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	result, err := uc.SpawnSubagent(ctx, "", "some prompt")

	if err == nil {
		t.Error("SpawnSubagent() did not return error for empty agentName")
	}
	if result != nil {
		t.Error("SpawnSubagent() should return nil result for empty agentName")
	}
	if err != nil && !strings.Contains(err.Error(), "agentName") {
		t.Errorf("Expected error message to mention 'agentName', got: %v", err)
	}
}

func TestSpawnSubagent_EmptyPrompt(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	result, err := uc.SpawnSubagent(ctx, "test-agent", "")

	if err == nil {
		t.Error("SpawnSubagent() did not return error for empty prompt")
	}
	if result != nil {
		t.Error("SpawnSubagent() should return nil result for empty prompt")
	}
	if err != nil && !strings.Contains(err.Error(), "prompt") {
		t.Errorf("Expected error message to mention 'prompt', got: %v", err)
	}
}

func TestSpawnSubagent_AgentNotFound(t *testing.T) {
	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return nil, errors.New("agent not found")
		},
	}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	result, err := uc.SpawnSubagent(ctx, "nonexistent-agent", "do something")

	if err == nil {
		t.Error("SpawnSubagent() did not return error when agent not found")
	}
	if result != nil {
		t.Error("SpawnSubagent() should return nil result when agent not found")
	}
}

func TestSpawnSubagent_LoadAgentMetadataError(t *testing.T) {
	expectedErr := errors.New("metadata load failure")
	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return nil, expectedErr
		},
	}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	result, err := uc.SpawnSubagent(ctx, "test-agent", "do something")

	if err == nil {
		t.Error("SpawnSubagent() did not propagate LoadAgentMetadata error")
	}
	if result != nil {
		t.Error("SpawnSubagent() should return nil result on LoadAgentMetadata error")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to be or wrap %v, got: %v", expectedErr, err)
	}
}

// ==================== Successful Spawn Tests ====================

func TestSpawnSubagent_SuccessfulSpawn(t *testing.T) {
	testAgent := &entity.Subagent{
		Name:        "test-agent",
		Description: "A test agent",
		RawContent:  "Test system prompt",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			if agentName == "test-agent" {
				return testAgent, nil
			}
			return nil, errors.New("agent not found")
		},
	}

	expectedResult := &SubagentResult{
		Status:    "completed",
		AgentName: "test-agent",
		Output:    "Task completed successfully",
		Error:     nil,
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return expectedResult, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	result, err := uc.SpawnSubagent(ctx, "test-agent", "do something cool")
	if err != nil {
		t.Fatalf("SpawnSubagent() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("SpawnSubagent() returned nil result on success")
	}
	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", result.Status)
	}
	if result.AgentName != "test-agent" {
		t.Errorf("Expected AgentName 'test-agent', got '%s'", result.AgentName)
	}
	if result.Output != "Task completed successfully" {
		t.Errorf("Expected output 'Task completed successfully', got '%s'", result.Output)
	}
}

func TestSpawnSubagent_GeneratesUniqueID(t *testing.T) {
	testAgent := &entity.Subagent{
		Name:        "test-agent",
		Description: "A test agent",
		RawContent:  "Test system prompt",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	var capturedIDs []string
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			capturedIDs = append(capturedIDs, subagentID)
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	// Spawn multiple times
	_, err1 := uc.SpawnSubagent(ctx, "test-agent", "task 1")
	_, err2 := uc.SpawnSubagent(ctx, "test-agent", "task 2")

	if err1 != nil || err2 != nil {
		t.Fatalf("SpawnSubagent() returned errors: %v, %v", err1, err2)
	}

	if len(capturedIDs) != 2 {
		t.Fatalf("Expected 2 subagent IDs to be captured, got %d", len(capturedIDs))
	}

	if capturedIDs[0] == "" || capturedIDs[1] == "" {
		t.Error("SpawnSubagent() generated empty subagent ID")
	}

	if capturedIDs[0] == capturedIDs[1] {
		t.Error("SpawnSubagent() generated duplicate subagent IDs")
	}
}

func TestSpawnSubagent_PassesCorrectAgentToRunner(t *testing.T) {
	testAgent := &entity.Subagent{
		Name:        "super-agent",
		Description: "The best agent ever",
		RawContent:  "You are super awesome",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	var capturedAgent *entity.Subagent
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			capturedAgent = agent
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	_, err := uc.SpawnSubagent(ctx, "super-agent", "do stuff")
	if err != nil {
		t.Fatalf("SpawnSubagent() returned error: %v", err)
	}

	if capturedAgent == nil {
		t.Fatal("SubagentRunner.Run() was not called with agent")
	}

	if capturedAgent.Name != "super-agent" {
		t.Errorf("Expected agent name 'super-agent', got '%s'", capturedAgent.Name)
	}
	if capturedAgent.Description != "The best agent ever" {
		t.Errorf("Expected agent description 'The best agent ever', got '%s'", capturedAgent.Description)
	}
	if capturedAgent.RawContent != "You are super awesome" {
		t.Errorf("Expected agent RawContent 'You are super awesome', got '%s'", capturedAgent.RawContent)
	}
}

func TestSpawnSubagent_PassesCorrectPromptToRunner(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	var capturedPrompt string
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			capturedPrompt = taskPrompt
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	expectedPrompt := "analyze this codebase thoroughly"
	_, err := uc.SpawnSubagent(ctx, "test-agent", expectedPrompt)
	if err != nil {
		t.Fatalf("SpawnSubagent() returned error: %v", err)
	}

	if capturedPrompt != expectedPrompt {
		t.Errorf("Expected prompt '%s', got '%s'", expectedPrompt, capturedPrompt)
	}
}

// ==================== Error Handling Tests ====================

func TestSpawnSubagent_RunnerErrorPropagates(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	expectedErr := errors.New("runner execution failed")
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return nil, expectedErr
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	result, err := uc.SpawnSubagent(ctx, "test-agent", "do something")

	if err == nil {
		t.Error("SpawnSubagent() did not propagate runner error")
	}
	if !errors.Is(err, expectedErr) && err.Error() != expectedErr.Error() {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Error("SpawnSubagent() should return nil result when runner returns error")
	}
}

func TestSpawnSubagent_FailedResultWithErrorDetails(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	failedResult := &SubagentResult{
		Status:    "failed",
		AgentName: "test-agent",
		Output:    "",
		Error:     errors.New("task execution failed"),
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return failedResult, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	result, err := uc.SpawnSubagent(ctx, "test-agent", "do something risky")
	if err != nil {
		t.Errorf("SpawnSubagent() returned error when runner returned failed result: %v", err)
	}
	if result == nil {
		t.Fatal("SpawnSubagent() returned nil result")
	}
	if result.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", result.Status)
	}
	if result.Error == nil {
		t.Error("Expected result.Error to be set for failed result")
	}
	if result.Error != nil && result.Error.Error() != "task execution failed" {
		t.Errorf("Expected error 'task execution failed', got '%v'", result.Error)
	}
}

func TestSpawnSubagent_ContextCancellation(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Check if context was passed correctly
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &SubagentResult{Status: "completed"}, nil
			}
		},
	}

	uc := NewSubagentUseCase(manager, runner)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := uc.SpawnSubagent(ctx, "test-agent", "do something")

	// Should propagate context cancellation
	if err == nil {
		t.Error("SpawnSubagent() did not return error for cancelled context")
	}
	if result != nil && result.Status == "completed" {
		t.Error("SpawnSubagent() should not return success result with cancelled context")
	}
}

// ==================== Async Spawn Tests (Cycle 5.2) ====================

// ==================== Input Validation Tests (Async) ====================

func TestSpawnSubagentAsync_EmptyAgentName(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	handle, err := uc.SpawnSubagentAsync(ctx, "", "some prompt")

	if err == nil {
		t.Error("SpawnSubagentAsync() did not return error for empty agentName")
	}
	if handle != nil {
		t.Error("SpawnSubagentAsync() should return nil handle for empty agentName")
	}
	if err != nil && !strings.Contains(err.Error(), "agentName") {
		t.Errorf("Expected error message to mention 'agentName', got: %v", err)
	}
}

func TestSpawnSubagentAsync_EmptyPrompt(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "")

	if err == nil {
		t.Error("SpawnSubagentAsync() did not return error for empty prompt")
	}
	if handle != nil {
		t.Error("SpawnSubagentAsync() should return nil handle for empty prompt")
	}
	if err != nil && !strings.Contains(err.Error(), "prompt") {
		t.Errorf("Expected error message to mention 'prompt', got: %v", err)
	}
}

func TestSpawnSubagentAsync_AgentNotFound(t *testing.T) {
	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return nil, errors.New("agent not found")
		},
	}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	handle, err := uc.SpawnSubagentAsync(ctx, "nonexistent-agent", "do something")

	if err == nil {
		t.Error("SpawnSubagentAsync() did not return error when agent not found")
	}
	if handle != nil {
		t.Error("SpawnSubagentAsync() should return nil handle when agent not found")
	}
}

// ==================== Non-Blocking Behavior Tests ====================

func TestSpawnSubagentAsync_ReturnsImmediately(t *testing.T) {
	testAgent := &entity.Subagent{
		Name:        "test-agent",
		Description: "A test agent",
		RawContent:  "Test system prompt",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	// Mock runner with intentional delay to verify non-blocking behavior
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Simulate slow execution (100ms)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return &SubagentResult{
					Status:    "completed",
					AgentName: "test-agent",
					Output:    "Task completed",
				}, nil
			}
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	start := time.Now()
	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}

	// Should return immediately (much less than 100ms - allow 50ms for overhead)
	if elapsed > 50*time.Millisecond {
		t.Errorf("Expected non-blocking return within 50ms, but took %v", elapsed)
	}

	if handle == nil {
		t.Fatal("SpawnSubagentAsync() returned nil handle")
	}

	// Wait for result to avoid goroutine leak
	select {
	case <-handle.Result:
	case <-handle.Error:
	case <-time.After(200 * time.Millisecond):
		t.Error("Timeout waiting for async execution to complete")
	}
}

func TestSpawnSubagentAsync_ReturnsValidHandle(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")
	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}
	if handle == nil {
		t.Fatal("SpawnSubagentAsync() returned nil handle")
	}
	if handle.Result == nil {
		t.Error("Handle.Result channel is nil")
	}
	if handle.Error == nil {
		t.Error("Handle.Error channel is nil")
	}

	// Wait for result to avoid goroutine leak
	select {
	case <-handle.Result:
	case <-handle.Error:
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for async execution")
	}
}

func TestSpawnSubagentAsync_GeneratesUniqueSubagentID(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	// Spawn multiple async subagents
	handle1, err1 := uc.SpawnSubagentAsync(ctx, "test-agent", "task 1")
	handle2, err2 := uc.SpawnSubagentAsync(ctx, "test-agent", "task 2")

	if err1 != nil || err2 != nil {
		t.Fatalf("SpawnSubagentAsync() returned errors: %v, %v", err1, err2)
	}

	if handle1.SubagentID == "" || handle2.SubagentID == "" {
		t.Error("SpawnSubagentAsync() generated empty subagent ID")
	}

	if handle1.SubagentID == handle2.SubagentID {
		t.Error("SpawnSubagentAsync() generated duplicate subagent IDs")
	}

	// Cleanup
	for _, h := range []*SubagentHandle{handle1, handle2} {
		select {
		case <-h.Result:
		case <-h.Error:
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestSpawnSubagentAsync_HandleContainsCorrectAgentName(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "super-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	handle, err := uc.SpawnSubagentAsync(ctx, "super-agent", "test prompt")
	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}
	if handle.AgentName != "super-agent" {
		t.Errorf("Expected handle.AgentName to be 'super-agent', got '%s'", handle.AgentName)
	}

	// Cleanup
	select {
	case <-handle.Result:
	case <-handle.Error:
	case <-time.After(100 * time.Millisecond):
	}
}

// ==================== Background Execution Tests ====================

func TestSpawnSubagentAsync_ResultSentToChannel(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	expectedResult := &SubagentResult{
		Status:    "completed",
		AgentName: "test-agent",
		Output:    "Task completed successfully",
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return expectedResult, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")
	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}

	// Wait for result from channel
	select {
	case result := <-handle.Result:
		if result == nil {
			t.Fatal("Received nil result from Result channel")
		}
		if result.Status != "completed" {
			t.Errorf("Expected status 'completed', got '%s'", result.Status)
		}
		if result.AgentName != "test-agent" {
			t.Errorf("Expected AgentName 'test-agent', got '%s'", result.AgentName)
		}
		if result.Output != "Task completed successfully" {
			t.Errorf("Expected output 'Task completed successfully', got '%s'", result.Output)
		}
	case err := <-handle.Error:
		t.Fatalf("Received error instead of result: %v", err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timeout waiting for result on Result channel")
	}
}

func TestSpawnSubagentAsync_ErrorSentToChannel(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	expectedErr := errors.New("runner execution failed")
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return nil, expectedErr
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")
	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}

	// Wait for error from channel
	select {
	case result := <-handle.Result:
		t.Fatalf("Received result instead of error: %v", result)
	case err := <-handle.Error:
		if err == nil {
			t.Fatal("Received nil error from Error channel")
		}
		if !errors.Is(err, expectedErr) && err.Error() != expectedErr.Error() {
			t.Errorf("Expected error '%v', got '%v'", expectedErr, err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timeout waiting for error on Error channel")
	}
}

func TestSpawnSubagentAsync_ChannelsClosedAfterResult(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")
	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}

	// Receive result
	select {
	case <-handle.Result:
	case <-handle.Error:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timeout waiting for result")
	}

	// Give a moment for channels to close
	time.Sleep(10 * time.Millisecond)

	// Verify Result channel is closed
	select {
	case _, ok := <-handle.Result:
		if ok {
			t.Error("Result channel should be closed after sending result")
		}
	default:
		// Channel might be closed but no value available
	}

	// Verify Error channel is closed
	select {
	case _, ok := <-handle.Error:
		if ok {
			t.Error("Error channel should be closed after sending result")
		}
	default:
		// Channel might be closed but no value available
	}
}

func TestSpawnSubagentAsync_OnlyOneMessageSent(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")
	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}

	resultReceived := false
	errorReceived := false

	// Receive from Result channel
	select {
	case _, ok := <-handle.Result:
		if ok {
			resultReceived = true
		}
	case <-time.After(200 * time.Millisecond):
	}

	// Receive from Error channel
	select {
	case _, ok := <-handle.Error:
		if ok {
			errorReceived = true
		}
	case <-time.After(10 * time.Millisecond):
	}

	// Verify only one message was sent (result OR error, not both)
	if resultReceived && errorReceived {
		t.Error("Both result and error were sent to channels (should be mutually exclusive)")
	}
	if !resultReceived && !errorReceived {
		t.Error("Neither result nor error was sent to channels")
	}
}

func TestSpawnSubagentAsync_MultipleSpawnsDontInterfere(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	var callCount atomic.Int32
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			count := callCount.Add(1)
			// Different delays to ensure they execute concurrently
			delay := time.Duration(count*10) * time.Millisecond
			time.Sleep(delay)
			return &SubagentResult{
				Status: "completed",
				Output: taskPrompt, // Echo prompt to verify isolation
			}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	// Spawn 3 subagents concurrently
	handle1, err1 := uc.SpawnSubagentAsync(ctx, "test-agent", "task 1")
	handle2, err2 := uc.SpawnSubagentAsync(ctx, "test-agent", "task 2")
	handle3, err3 := uc.SpawnSubagentAsync(ctx, "test-agent", "task 3")

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("SpawnSubagentAsync() returned errors: %v, %v, %v", err1, err2, err3)
	}

	// Collect results
	results := make(map[string]string)
	for _, h := range []*SubagentHandle{handle1, handle2, handle3} {
		select {
		case result := <-h.Result:
			results[h.SubagentID] = result.Output
		case err := <-h.Error:
			t.Fatalf("Received error from handle: %v", err)
		case <-time.After(300 * time.Millisecond):
			t.Fatal("Timeout waiting for results")
		}
	}

	// Verify all 3 tasks completed with correct outputs
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

// ==================== Context Cancellation Tests ====================

func TestSpawnSubagentAsync_ContextCancellationDuringExecution(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Simulate long-running task that respects context
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return &SubagentResult{Status: "completed"}, nil
			}
		},
	}

	uc := NewSubagentUseCase(manager, runner)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")
	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}

	// Cancel context after spawn but during execution
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should receive error from Error channel
	select {
	case result := <-handle.Result:
		t.Fatalf("Received result instead of error after cancellation: %v", result)
	case err := <-handle.Error:
		if err == nil {
			t.Fatal("Received nil error after context cancellation")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Timeout waiting for cancellation error")
	}
}

func TestSpawnSubagentAsync_CancelledContextBeforeSpawn(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &SubagentResult{Status: "completed"}, nil
			}
		},
	}

	uc := NewSubagentUseCase(manager, runner)

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")

	// Should return immediately with error (no goroutine spawned for cancelled context)
	if err == nil {
		t.Error("SpawnSubagentAsync() did not return error for already-cancelled context")
	}
	if handle != nil {
		t.Error("SpawnSubagentAsync() should return nil handle for cancelled context")
	}
}

func TestSpawnSubagentAsync_ContextTimeoutPropagates(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Simulate long task
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(300 * time.Millisecond):
				return &SubagentResult{Status: "completed"}, nil
			}
		},
	}

	uc := NewSubagentUseCase(manager, runner)

	// Context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")
	if err != nil {
		t.Fatalf("SpawnSubagentAsync() returned error: %v", err)
	}

	// Should receive timeout error
	select {
	case result := <-handle.Result:
		t.Fatalf("Received result instead of timeout error: %v", result)
	case err := <-handle.Error:
		if err == nil {
			t.Fatal("Received nil error after timeout")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded error, got: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timeout waiting for context timeout error")
	}
}

// ==================== LoadAgentMetadata Error Tests ====================

func TestSpawnSubagentAsync_LoadAgentMetadataError(t *testing.T) {
	expectedErr := errors.New("metadata load failure")
	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return nil, expectedErr
		},
	}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")

	// Should return error immediately (synchronously) without spawning goroutine
	if err == nil {
		t.Error("SpawnSubagentAsync() did not return error for LoadAgentMetadata failure")
	}
	if handle != nil {
		t.Error("SpawnSubagentAsync() should return nil handle for LoadAgentMetadata failure")
	}
	if !errors.Is(err, expectedErr) && !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Errorf("Expected error to wrap '%v', got: %v", expectedErr, err)
	}
}

func TestSpawnSubagentAsync_LoadAgentMetadataNoChannelsSent(t *testing.T) {
	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return nil, errors.New("agent not found")
		},
	}

	// Runner should NOT be called if LoadAgentMetadata fails
	runnerCalled := false
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			runnerCalled = true
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	handle, err := uc.SpawnSubagentAsync(ctx, "test-agent", "test prompt")

	if err == nil {
		t.Error("Expected error when LoadAgentMetadata fails")
	}
	if handle != nil {
		t.Error("Handle should be nil when LoadAgentMetadata fails")
	}

	// Wait a bit to ensure no goroutine was spawned
	time.Sleep(50 * time.Millisecond)

	if runnerCalled {
		t.Error("SubagentRunner.Run() should not be called when LoadAgentMetadata fails")
	}
}

// ==================== Parallel Spawn Tests (Cycle 5.3) ====================

// ==================== Input Validation Tests (Parallel) ====================

func TestSpawnMultiple_NilRequestsSlice(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	result, err := uc.SpawnMultiple(ctx, nil)
	if err != nil {
		t.Errorf("SpawnMultiple() returned error for nil requests: %v", err)
	}
	if result == nil {
		t.Fatal("SpawnMultiple() should return empty result, not nil, for nil requests")
	}
	if len(result.Results) != 0 {
		t.Errorf("Expected 0 results for nil requests, got %d", len(result.Results))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors for nil requests, got %d", len(result.Errors))
	}
}

func TestSpawnMultiple_EmptyRequestsSlice(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	result, err := uc.SpawnMultiple(ctx, []*SubagentRequest{})
	if err != nil {
		t.Errorf("SpawnMultiple() returned error for empty requests: %v", err)
	}
	if result == nil {
		t.Fatal("SpawnMultiple() should return empty result, not nil, for empty requests")
	}
	if len(result.Results) != 0 {
		t.Errorf("Expected 0 results for empty requests, got %d", len(result.Results))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors for empty requests, got %d", len(result.Errors))
	}
}

func TestSpawnMultiple_SingleRequest(t *testing.T) {
	testAgent := &entity.Subagent{
		Name:        "test-agent",
		Description: "A test agent",
		RawContent:  "Test system prompt",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	expectedResult := &SubagentResult{
		Status:    "completed",
		AgentName: "test-agent",
		Output:    "Task completed",
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return expectedResult, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "do something"},
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("SpawnMultiple() returned nil result")
	}
	if len(result.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(result.Results))
	}
	if len(result.Errors) != 1 {
		t.Fatalf("Expected 1 error entry, got %d", len(result.Errors))
	}
	if result.Results[0].Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", result.Results[0].Status)
	}
	if result.Errors[0] != nil {
		t.Errorf("Expected nil error for successful request, got: %v", result.Errors[0])
	}
}

// ==================== Parallel Execution Tests ====================

func TestSpawnMultiple_MultipleSuccessfulSpawns(t *testing.T) {
	testAgent := &entity.Subagent{
		Name:        "test-agent",
		Description: "A test agent",
		RawContent:  "Test system prompt",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Simulate work to verify parallel execution
			time.Sleep(10 * time.Millisecond)
			return &SubagentResult{
				Status:    "completed",
				AgentName: agent.Name,
				Output:    taskPrompt,
			}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	// Create 3 requests
	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
	}

	start := time.Now()
	result, err := uc.SpawnMultiple(ctx, requests)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// Should complete in ~10ms (parallel) not ~30ms (sequential)
	// Allow 25ms overhead for goroutine scheduling
	if elapsed > 25*time.Millisecond {
		t.Errorf("Expected parallel execution ~10ms, took %v (likely sequential)", elapsed)
	}

	if len(result.Results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(result.Results))
	}
	if len(result.Errors) != 3 {
		t.Fatalf("Expected 3 error entries, got %d", len(result.Errors))
	}

	// Verify all successful
	for i, res := range result.Results {
		if res == nil {
			t.Errorf("Result %d is nil", i)
			continue
		}
		if res.Status != "completed" {
			t.Errorf("Request %d failed with status '%s'", i, res.Status)
		}
		if result.Errors[i] != nil {
			t.Errorf("Request %d has error: %v", i, result.Errors[i])
		}
	}
}

func TestSpawnMultiple_ResultsMatchRequestOrder(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Different delays to ensure non-sequential completion
			// This tests that results are properly ordered despite completion order
			delay := time.Duration(0)
			switch taskPrompt {
			case "task 1":
				delay = 20 * time.Millisecond
			case "task 2":
				delay = 10 * time.Millisecond
			case "task 3":
				delay = 5 * time.Millisecond
			}
			time.Sleep(delay)

			return &SubagentResult{
				Status:    "completed",
				AgentName: agent.Name,
				Output:    taskPrompt, // Echo prompt to verify ordering
			}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// Verify results match request order despite task 3 finishing first
	if result.Results[0].Output != "task 1" {
		t.Errorf("Result[0] expected 'task 1', got '%s'", result.Results[0].Output)
	}
	if result.Results[1].Output != "task 2" {
		t.Errorf("Result[1] expected 'task 2', got '%s'", result.Results[1].Output)
	}
	if result.Results[2].Output != "task 3" {
		t.Errorf("Result[2] expected 'task 3', got '%s'", result.Results[2].Output)
	}
}

func TestSpawnMultiple_EachSpawnGetsUniqueSubagentID(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	var capturedIDs []string
	var mu sync.Mutex
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			mu.Lock()
			capturedIDs = append(capturedIDs, subagentID)
			mu.Unlock()
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
	}

	_, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	if len(capturedIDs) != 3 {
		t.Fatalf("Expected 3 subagent IDs, got %d", len(capturedIDs))
	}

	// Verify all IDs are unique
	idSet := make(map[string]bool)
	for _, id := range capturedIDs {
		if id == "" {
			t.Error("Generated empty subagent ID")
		}
		if idSet[id] {
			t.Errorf("Duplicate subagent ID: %s", id)
		}
		idSet[id] = true
	}
}

func TestSpawnMultiple_ActuallyRunsInParallel(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	// Track concurrent execution using counter
	var concurrentCount int32
	var maxConcurrent int32
	var mu sync.Mutex

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Increment concurrent counter
			current := atomic.AddInt32(&concurrentCount, 1)

			// Track maximum concurrent executions
			mu.Lock()
			if current > maxConcurrent {
				maxConcurrent = current
			}
			mu.Unlock()

			// Simulate work
			time.Sleep(20 * time.Millisecond)

			// Decrement concurrent counter
			atomic.AddInt32(&concurrentCount, -1)

			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	// Create 5 requests to ensure we see parallel execution
	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
		{AgentName: "test-agent", Prompt: "task 4"},
		{AgentName: "test-agent", Prompt: "task 5"},
	}

	_, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// If running in parallel, we should have seen multiple concurrent executions
	if maxConcurrent < 2 {
		t.Errorf(
			"Expected concurrent execution (maxConcurrent >= 2), got maxConcurrent=%d (likely sequential)",
			maxConcurrent,
		)
	}
}

func TestSpawnMultiple_LargeBatch(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			return &SubagentResult{
				Status: "completed",
				Output: taskPrompt,
			}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	// Create 15 requests
	requests := make([]*SubagentRequest, 15)
	for i := range 15 {
		requests[i] = &SubagentRequest{
			AgentName: "test-agent",
			Prompt:    fmt.Sprintf("task %d", i),
		}
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	if len(result.Results) != 15 {
		t.Fatalf("Expected 15 results, got %d", len(result.Results))
	}
	if len(result.Errors) != 15 {
		t.Fatalf("Expected 15 error entries, got %d", len(result.Errors))
	}

	// Verify all completed successfully
	for i := range 15 {
		if result.Results[i].Status != "completed" {
			t.Errorf("Request %d failed", i)
		}
		if result.Errors[i] != nil {
			t.Errorf("Request %d has error: %v", i, result.Errors[i])
		}
		expectedOutput := fmt.Sprintf("task %d", i)
		if result.Results[i].Output != expectedOutput {
			t.Errorf("Result %d output mismatch: expected '%s', got '%s'", i, expectedOutput, result.Results[i].Output)
		}
	}
}

// ==================== Individual Error Handling Tests ====================

func TestSpawnMultiple_OneFailedSpawnDoesntAffectOthers(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Fail only the second task
			if taskPrompt == "task 2" {
				return nil, errors.New("task 2 failed")
			}
			return &SubagentResult{
				Status: "completed",
				Output: taskPrompt,
			}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// Verify task 1 succeeded
	if result.Results[0] == nil || result.Results[0].Status != "completed" {
		t.Error("Task 1 should have succeeded")
	}
	if result.Errors[0] != nil {
		t.Errorf("Task 1 should have no error, got: %v", result.Errors[0])
	}

	// Verify task 2 failed
	if result.Results[1] != nil {
		t.Error("Task 2 should have nil result on error")
	}
	if result.Errors[1] == nil {
		t.Error("Task 2 should have error")
	}
	if result.Errors[1] != nil && !strings.Contains(result.Errors[1].Error(), "task 2 failed") {
		t.Errorf("Expected 'task 2 failed' error, got: %v", result.Errors[1])
	}

	// Verify task 3 succeeded
	if result.Results[2] == nil || result.Results[2].Status != "completed" {
		t.Error("Task 3 should have succeeded")
	}
	if result.Errors[2] != nil {
		t.Errorf("Task 3 should have no error, got: %v", result.Errors[2])
	}
}

func TestSpawnMultiple_MixedSuccessAndFailure(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Fail tasks with even numbers in the prompt
			if strings.Contains(taskPrompt, "2") || strings.Contains(taskPrompt, "4") {
				return nil, fmt.Errorf("%s failed", taskPrompt)
			}
			return &SubagentResult{
				Status: "completed",
				Output: taskPrompt,
			}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
		{AgentName: "test-agent", Prompt: "task 4"},
		{AgentName: "test-agent", Prompt: "task 5"},
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// Verify successes (1, 3, 5)
	successIndices := []int{0, 2, 4}
	for _, i := range successIndices {
		if result.Results[i] == nil {
			t.Errorf("Task %d should have result", i+1)
		}
		if result.Errors[i] != nil {
			t.Errorf("Task %d should have no error, got: %v", i+1, result.Errors[i])
		}
	}

	// Verify failures (2, 4)
	failureIndices := []int{1, 3}
	for _, i := range failureIndices {
		if result.Results[i] != nil {
			t.Errorf("Task %d should have nil result on error", i+1)
		}
		if result.Errors[i] == nil {
			t.Errorf("Task %d should have error", i+1)
		}
	}
}

func TestSpawnMultiple_ErrorsArrayMatchesRequestOrder(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Fail with error message containing task name
			if strings.Contains(taskPrompt, "fail") {
				return nil, fmt.Errorf("error for %s", taskPrompt)
			}
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1 success"},
		{AgentName: "test-agent", Prompt: "task 2 fail"},
		{AgentName: "test-agent", Prompt: "task 3 success"},
		{AgentName: "test-agent", Prompt: "task 4 fail"},
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// Verify error at index 1 contains "task 2 fail"
	if result.Errors[1] == nil {
		t.Error("Expected error at index 1")
	} else if !strings.Contains(result.Errors[1].Error(), "task 2 fail") {
		t.Errorf("Error at index 1 should contain 'task 2 fail', got: %v", result.Errors[1])
	}

	// Verify error at index 3 contains "task 4 fail"
	if result.Errors[3] == nil {
		t.Error("Expected error at index 3")
	} else if !strings.Contains(result.Errors[3].Error(), "task 4 fail") {
		t.Errorf("Error at index 3 should contain 'task 4 fail', got: %v", result.Errors[3])
	}

	// Verify successes have nil errors
	if result.Errors[0] != nil {
		t.Errorf("Expected nil error at index 0, got: %v", result.Errors[0])
	}
	if result.Errors[2] != nil {
		t.Errorf("Expected nil error at index 2, got: %v", result.Errors[2])
	}
}

func TestSpawnMultiple_AllFailuresReturnsAllErrors(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// All tasks fail
			return nil, fmt.Errorf("error: %s", taskPrompt)
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// Verify all results are nil
	for i := range 3 {
		if result.Results[i] != nil {
			t.Errorf("Result %d should be nil on error", i)
		}
	}

	// Verify all errors are set
	for i := range 3 {
		if result.Errors[i] == nil {
			t.Errorf("Error %d should be set", i)
		}
		expectedMsg := fmt.Sprintf("task %d", i+1)
		if !strings.Contains(result.Errors[i].Error(), expectedMsg) {
			t.Errorf("Error %d should contain '%s', got: %v", i, expectedMsg, result.Errors[i])
		}
	}
}

// ==================== Context Cancellation Tests ====================

func TestSpawnMultiple_ContextCancellationStopsPendingSpawns(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	var startedCount int32
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			atomic.AddInt32(&startedCount, 1)

			// Simulate long-running task that respects context
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return &SubagentResult{Status: "completed"}, nil
			}
		},
	}

	uc := NewSubagentUseCase(manager, runner)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
		{AgentName: "test-agent", Prompt: "task 4"},
		{AgentName: "test-agent", Prompt: "task 5"},
	}

	// Cancel context shortly after starting
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// All results should have errors due to cancellation
	cancelledCount := 0
	for i := range len(result.Errors) {
		if result.Errors[i] != nil && errors.Is(result.Errors[i], context.Canceled) {
			cancelledCount++
		}
	}

	// At least some should be cancelled
	if cancelledCount == 0 {
		t.Error("Expected at least some spawns to be cancelled")
	}
}

func TestSpawnMultiple_PreCancelledContext(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &SubagentResult{Status: "completed"}, nil
			}
		},
	}

	uc := NewSubagentUseCase(manager, runner)

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// All should have cancellation errors
	for i := range requests {
		if result.Results[i] != nil {
			t.Errorf("Result %d should be nil for cancelled context", i)
		}
		if result.Errors[i] == nil {
			t.Errorf("Error %d should be set for cancelled context", i)
		}
		if result.Errors[i] != nil && !errors.Is(result.Errors[i], context.Canceled) {
			t.Errorf("Error %d should be context.Canceled, got: %v", i, result.Errors[i])
		}
	}
}

func TestSpawnMultiple_TimeoutDuringExecution(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Simulate long task
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return &SubagentResult{Status: "completed"}, nil
			}
		},
	}

	uc := NewSubagentUseCase(manager, runner)

	// Context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	requests := []*SubagentRequest{
		{AgentName: "test-agent", Prompt: "task 1"},
		{AgentName: "test-agent", Prompt: "task 2"},
		{AgentName: "test-agent", Prompt: "task 3"},
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// All should timeout
	for i := range requests {
		if result.Errors[i] == nil {
			t.Errorf("Error %d should be set for timeout", i)
			continue
		}
		if !errors.Is(result.Errors[i], context.DeadlineExceeded) {
			t.Errorf("Error %d should be context.DeadlineExceeded, got: %v", i, result.Errors[i])
		}
	}
}

// ==================== Race Condition Tests ====================

func TestSpawnMultiple_ConcurrentWritesDontCorruptResults(t *testing.T) {
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Random small delay to increase likelihood of races
			delay := time.Duration(1+len(taskPrompt)%5) * time.Millisecond
			time.Sleep(delay)
			return &SubagentResult{
				Status: "completed",
				Output: taskPrompt,
			}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	// Create many requests to stress-test concurrent writes
	requests := make([]*SubagentRequest, 20)
	for i := range 20 {
		requests[i] = &SubagentRequest{
			AgentName: "test-agent",
			Prompt:    fmt.Sprintf("task %d", i),
		}
	}

	result, err := uc.SpawnMultiple(ctx, requests)
	if err != nil {
		t.Fatalf("SpawnMultiple() returned error: %v", err)
	}

	// Verify no results were corrupted or lost
	if len(result.Results) != 20 {
		t.Fatalf("Expected 20 results, got %d (possible corruption)", len(result.Results))
	}

	// Verify each result matches its request
	for i := range 20 {
		if result.Results[i] == nil {
			t.Errorf("Result %d is nil (possible corruption)", i)
			continue
		}
		expectedOutput := fmt.Sprintf("task %d", i)
		if result.Results[i].Output != expectedOutput {
			t.Errorf("Result %d corrupted: expected '%s', got '%s'", i, expectedOutput, result.Results[i].Output)
		}
	}
}

func TestSpawnMultiple_RaceFlagPasses(t *testing.T) {
	// NOTE: This test should be run with: go test -race ./internal/application/usecase -v -run TestSpawnMultiple_RaceFlagPasses
	testAgent := &entity.Subagent{
		Name: "test-agent",
	}

	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			return testAgent, nil
		},
	}

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			// Small random delays to trigger race detector if there are issues
			delay := time.Duration(1+(len(taskPrompt)*7)%10) * time.Millisecond
			time.Sleep(delay)
			return &SubagentResult{
				Status: "completed",
				Output: taskPrompt,
			}, nil
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	// Run multiple parallel batches to stress test
	for batch := range 3 {
		requests := make([]*SubagentRequest, 10)
		for i := range 10 {
			requests[i] = &SubagentRequest{
				AgentName: "test-agent",
				Prompt:    fmt.Sprintf("batch %d task %d", batch, i),
			}
		}

		result, err := uc.SpawnMultiple(ctx, requests)
		if err != nil {
			t.Fatalf("Batch %d failed: %v", batch, err)
		}

		if len(result.Results) != 10 {
			t.Fatalf("Batch %d: expected 10 results, got %d", batch, len(result.Results))
		}
	}

	// If there are race conditions, the -race flag will detect them
}

// ==================== Dynamic Subagent Tests ====================

func TestSpawnDynamicSubagent_Success(t *testing.T) {
	var capturedAgent *entity.Subagent
	var capturedPrompt string

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			capturedAgent = agent
			capturedPrompt = taskPrompt
			return &SubagentResult{
				Status:    "completed",
				AgentName: agent.Name,
				Output:    "Task completed successfully",
			}, nil
		},
	}

	// Manager should NOT be called for dynamic agents
	manager := &MockSubagentManager{
		LoadAgentMetadataFunc: func(ctx context.Context, agentName string) (*entity.Subagent, error) {
			t.Error("LoadAgentMetadata should not be called for dynamic subagents")
			return nil, errors.New("should not be called")
		},
	}

	uc := NewSubagentUseCase(manager, runner)
	ctx := context.Background()

	config := DynamicSubagentConfig{
		Name:         "test-analyzer",
		Description:  "A test analyzer agent",
		SystemPrompt: "You are a test analyzer. Analyze the code.",
		Model:        "haiku",
		MaxActions:   15,
		AllowedTools: []string{"read_file", "list_files"},
	}

	result, err := uc.SpawnDynamicSubagent(ctx, config, "analyze the auth module")
	if err != nil {
		t.Fatalf("SpawnDynamicSubagent() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("SpawnDynamicSubagent() returned nil result")
	}
	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", result.Status)
	}

	// Verify the agent was created correctly
	if capturedAgent == nil {
		t.Fatal("Runner was not called with agent")
	}
	if capturedAgent.Name != "test-analyzer" {
		t.Errorf("Expected agent name 'test-analyzer', got '%s'", capturedAgent.Name)
	}
	if capturedAgent.RawContent != "You are a test analyzer. Analyze the code." {
		t.Errorf("Expected RawContent to be system prompt, got '%s'", capturedAgent.RawContent)
	}
	if capturedAgent.Model != "haiku" {
		t.Errorf("Expected model 'haiku', got '%s'", capturedAgent.Model)
	}
	if capturedAgent.MaxActions != 15 {
		t.Errorf("Expected max_actions 15, got %d", capturedAgent.MaxActions)
	}
	if len(capturedAgent.AllowedTools) != 2 {
		t.Errorf("Expected 2 allowed tools, got %d", len(capturedAgent.AllowedTools))
	}
	if capturedAgent.SourceType != entity.SubagentSourceProgrammatic {
		t.Errorf("Expected SourceType 'programmatic', got '%s'", capturedAgent.SourceType)
	}

	// Verify task prompt was passed correctly
	if capturedPrompt != "analyze the auth module" {
		t.Errorf("Expected prompt 'analyze the auth module', got '%s'", capturedPrompt)
	}
}

func TestSpawnDynamicSubagent_MissingName(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	config := DynamicSubagentConfig{
		Name:         "", // Missing
		SystemPrompt: "You are an agent",
	}

	result, err := uc.SpawnDynamicSubagent(ctx, config, "do something")
	if err == nil {
		t.Error("SpawnDynamicSubagent() did not return error for missing name")
	}
	if result != nil {
		t.Error("SpawnDynamicSubagent() should return nil result for missing name")
	}
	if err != nil && !strings.Contains(err.Error(), "name") {
		t.Errorf("Expected error message to mention 'name', got: %v", err)
	}
}

func TestSpawnDynamicSubagent_MissingSystemPrompt(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	config := DynamicSubagentConfig{
		Name:         "test-agent",
		SystemPrompt: "", // Missing
	}

	result, err := uc.SpawnDynamicSubagent(ctx, config, "do something")
	if err == nil {
		t.Error("SpawnDynamicSubagent() did not return error for missing system_prompt")
	}
	if result != nil {
		t.Error("SpawnDynamicSubagent() should return nil result for missing system_prompt")
	}
	if err != nil && !strings.Contains(err.Error(), "system_prompt") && !strings.Contains(err.Error(), "SystemPrompt") {
		t.Errorf("Expected error message to mention 'system_prompt', got: %v", err)
	}
}

func TestSpawnDynamicSubagent_MissingTask(t *testing.T) {
	manager := &MockSubagentManager{}
	runner := &MockSubagentRunner{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	config := DynamicSubagentConfig{
		Name:         "test-agent",
		SystemPrompt: "You are an agent",
	}

	result, err := uc.SpawnDynamicSubagent(ctx, config, "") // Missing task
	if err == nil {
		t.Error("SpawnDynamicSubagent() did not return error for empty task")
	}
	if result != nil {
		t.Error("SpawnDynamicSubagent() should return nil result for empty task")
	}
	if err != nil && !strings.Contains(err.Error(), "task") {
		t.Errorf("Expected error message to mention 'task', got: %v", err)
	}
}

func TestSpawnDynamicSubagent_AppliesDefaults(t *testing.T) {
	var capturedAgent *entity.Subagent

	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			capturedAgent = agent
			return &SubagentResult{Status: "completed"}, nil
		},
	}

	manager := &MockSubagentManager{}
	uc := NewSubagentUseCase(manager, runner)

	ctx := context.Background()
	config := DynamicSubagentConfig{
		Name:         "test-agent",
		SystemPrompt: "You are an agent",
		// Model and MaxActions omitted - should use defaults
	}

	_, err := uc.SpawnDynamicSubagent(ctx, config, "do something")
	if err != nil {
		t.Fatalf("SpawnDynamicSubagent() returned error: %v", err)
	}

	if capturedAgent == nil {
		t.Fatal("Runner was not called")
	}

	// Verify defaults
	if capturedAgent.Model != "inherit" {
		t.Errorf("Expected default model 'inherit', got '%s'", capturedAgent.Model)
	}
	if capturedAgent.MaxActions != 30 {
		t.Errorf("Expected default max_actions 30, got %d", capturedAgent.MaxActions)
	}
	if capturedAgent.AllowedTools != nil {
		t.Errorf("Expected default allowed_tools to be nil (all tools), got %v", capturedAgent.AllowedTools)
	}
}
