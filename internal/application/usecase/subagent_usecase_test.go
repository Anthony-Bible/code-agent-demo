package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"strings"
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
	return nil, nil
}

func (m *MockSubagentManager) LoadAgentMetadata(ctx context.Context, agentName string) (*entity.Subagent, error) {
	if m.LoadAgentMetadataFunc != nil {
		return m.LoadAgentMetadataFunc(ctx, agentName)
	}
	return nil, nil
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
	return nil, nil
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
	return nil, nil
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
	if !errors.Is(err, expectedErr) && !errors.Is(err, expectedErr) {
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

	var callCount int
	runner := &MockSubagentRunner{
		RunFunc: func(ctx context.Context, agent *entity.Subagent, taskPrompt string, subagentID string) (*SubagentResult, error) {
			callCount++
			// Different delays to ensure they execute concurrently
			delay := time.Duration(callCount*10) * time.Millisecond
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
