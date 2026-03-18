package integration

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/application/usecase"
	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	"github.com/anthony-bible/code-agent-demo/internal/domain/service"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockAIProviderForRCA is a specialized mock for the integration test.
type mockAIProviderForRCA struct {
	mock.Mock
}

func (m *mockAIProviderForRCA) SendMessage(ctx context.Context, messages []port.MessageParam, tools []port.ToolParam) (*entity.Message, []port.ToolCallInfo, error) {
	args := m.Called(ctx, messages, tools)
	msg := args.Get(0).(*entity.Message)
	var toolCalls []port.ToolCallInfo
	if args.Get(1) != nil {
		toolCalls = args.Get(1).([]port.ToolCallInfo)
	}
	return msg, toolCalls, args.Error(2)
}

func (m *mockAIProviderForRCA) SendMessageStreaming(ctx context.Context, messages []port.MessageParam, tools []port.ToolParam, textCallback port.StreamCallback, thinkingCallback port.ThinkingCallback) (*entity.Message, []port.ToolCallInfo, error) {
	return m.SendMessage(ctx, messages, tools)
}

func (m *mockAIProviderForRCA) GenerateToolSchema() port.ToolInputSchemaParam { return nil }
func (m *mockAIProviderForRCA) HealthCheck(ctx context.Context) error         { return nil }
func (m *mockAIProviderForRCA) SetModel(model string) error                   { return nil }
func (m *mockAIProviderForRCA) GetModel() string                              { return "test-model" }

// mockToolExecutorForRCA implements port.ToolExecutor.
type mockToolExecutorForRCA struct {
	port.ToolExecutor
}

func (m *mockToolExecutorForRCA) ListTools() ([]entity.Tool, error) {
	return []entity.Tool{}, nil
}

// mockPromptBuilderRegistryForRCA implements usecase.PromptBuilderRegistry.
type mockPromptBuilderRegistryForRCA struct{}

func (m *mockPromptBuilderRegistryForRCA) Register(builder usecase.InvestigationPromptBuilder) error {
	return nil
}

func (m *mockPromptBuilderRegistryForRCA) Get(alertType string) (usecase.InvestigationPromptBuilder, error) {
	return nil, nil
}

func (m *mockPromptBuilderRegistryForRCA) BuildPromptForAlert(alert *usecase.AlertView, tools []entity.Tool, skills []port.SkillInfo) (string, error) {
	return "Test prompt", nil
}

func (m *mockPromptBuilderRegistryForRCA) ListAlertTypes() []string {
	return []string{}
}

// mockSafetyEnforcerForRCA implements usecase.SafetyEnforcer.
type mockSafetyEnforcerForRCA struct{}

func (m *mockSafetyEnforcerForRCA) CheckToolAllowed(tool string) error         { return nil }
func (m *mockSafetyEnforcerForRCA) CheckCommandAllowed(cmd string) error       { return nil }
func (m *mockSafetyEnforcerForRCA) CheckActionBudget(currentActions int) error { return nil }
func (m *mockSafetyEnforcerForRCA) CheckTimeout(ctx context.Context) error     { return nil }
func (m *mockSafetyEnforcerForRCA) GetMaxActions() int                         { return 10 }

func TestEndToEndRCALogic(t *testing.T) {
	// 1. Setup Dependencies
	mockAI := new(mockAIProviderForRCA)
	mockTools := new(mockToolExecutorForRCA)

	convService, err := service.NewConversationService(mockAI, mockTools)
	require.NoError(t, err)

	// UI Adapter with captured output
	output := &strings.Builder{}
	uiAdapter := ui.NewCLIAdapterWithIO(strings.NewReader(""), output)

	// RCA Service
	rcaService := service.NewRCAService(mockAI)

	// Investigation Use Case
	invConfig := usecase.AlertInvestigationUseCaseConfig{
		MaxConcurrent: 5,
	}
	invUseCase := usecase.NewAlertInvestigationUseCaseWithConfig(invConfig)

	// Use reflection to set private fields for the integration test
	var promptRegistry usecase.PromptBuilderRegistry = &mockPromptBuilderRegistryForRCA{}
	var safetyEnforcer usecase.SafetyEnforcer = &mockSafetyEnforcerForRCA{}

	setPrivateField(invUseCase, "convService", convService)
	setPrivateField(invUseCase, "rcaService", rcaService)
	setPrivateField(invUseCase, "uiAdapter", uiAdapter)
	setPrivateField(invUseCase, "rcaReporter", port.RCAReporter(uiAdapter))
	setPrivateField(invUseCase, "toolExecutor", port.ToolExecutor(mockTools))
	setPrivateField(invUseCase, "promptBuilderRegistry", promptRegistry)
	setPrivateField(invUseCase, "safetyEnforcer", safetyEnforcer)

	// 2. Configure Mock AI Expectations

	// First call: Investigation loop starts, AI decides to complete immediately with findings
	completionToolCall := port.ToolCallInfo{
		ToolID:   "call-completion",
		ToolName: "complete_investigation",
		Input: map[string]interface{}{
			"confidence": 0.9,
			"findings": []interface{}{
				"High CPU detected on server-01",
				"Process 'bad-actor' is using 99% CPU",
			},
		},
	}

	// Use mock.Anything for context as ConversationService wraps it
	mockAI.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(
		&entity.Message{Role: entity.RoleAssistant, Content: "Investigation finished."},
		[]port.ToolCallInfo{completionToolCall},
		nil,
	).Once()

	// Second call: RCAService calls AI to correlate findings
	rcaResponseJSON := `{
		"findings": [
			{
				"summary": "CPU exhaustion on server-01 due to 'bad-actor' process",
				"causes": [
					{
						"id": "C1",
						"description": "Runaway 'bad-actor' process",
						"confidence_score": 0.95,
						"evidence": ["CPU at 99%", "Process name identified"]
					}
				],
				"remedies": [
					{
						"description": "Kill 'bad-actor' process",
						"actionable_steps": ["killall -9 bad-actor"],
						"impact": "High"
					},
					{
						"description": "Investigate 'bad-actor' logs",
						"actionable_steps": ["tail -f /var/log/bad-actor.log"],
						"impact": "Medium"
					}
				]
			}
		]
	}`

	mockAI.On("SendMessage", mock.Anything, mock.MatchedBy(func(msgs []port.MessageParam) bool {
		return len(msgs) > 0 && strings.Contains(msgs[0].Content, "As an expert SRE")
	}), mock.Anything).Return(
		&entity.Message{Role: entity.RoleAssistant, Content: rcaResponseJSON},
		nil,
		nil,
	).Once()

	// 3. Execute Alert Handler
	handlerConfig := usecase.AlertHandlerConfig{
		AutoInvestigateCritical: true,
	}
	handler := usecase.NewAlertHandler(invUseCase, handlerConfig)

	alert, err := entity.NewAlert("alert-123", "prometheus", entity.SeverityCritical, "High CPU Usage")
	require.NoError(t, err)

	err = handler.RunEntityAlertInvestigation(context.Background(), alert, "inv-123")
	require.NoError(t, err)

	// 4. Assert Results
	out := output.String()

	// Verify RCA header is present
	assert.Contains(t, out, "ROOT CAUSE ANALYSIS")
	assert.Contains(t, out, "SUMMARY: CPU exhaustion on server-01 due to 'bad-actor' process")
	assert.Contains(t, out, "IDENTIFIED CAUSES:")
	assert.Contains(t, out, "[C1] Runaway 'bad-actor' process")
	assert.Contains(t, out, "SUGGESTED REMEDIES:")
	assert.Contains(t, out, "Kill 'bad-actor' process")
	assert.Contains(t, out, "killall -9 bad-actor")

	mockAI.AssertExpectations(t)
}

func setPrivateField(obj interface{}, name string, value interface{}) {
	v := reflect.ValueOf(obj).Elem()
	f := v.FieldByName(name)
	if !f.IsValid() {
		return
	}

	// Use reflect.NewAt to set unexported field
	ptr := reflect.NewAt(f.Type(), f.Addr().UnsafePointer())
	ptr.Elem().Set(reflect.ValueOf(value))
}
