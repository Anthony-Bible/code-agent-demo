package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	"github.com/anthony-bible/code-agent-demo/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAIProvider is a mock of the port.AIProvider interface.
type MockAIProvider struct {
	mock.Mock
}

func (m *MockAIProvider) SendMessage(ctx context.Context, messages []port.MessageParam, tools []port.ToolParam, _ port.AIRequestOptions) (*entity.Message, []port.ToolCallInfo, error) {
	args := m.Called(ctx, messages, tools)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*entity.Message), args.Get(1).([]port.ToolCallInfo), args.Error(2)
}

func (m *MockAIProvider) SendMessageStreaming(ctx context.Context, messages []port.MessageParam, tools []port.ToolParam, _ port.AIRequestOptions, textCallback port.StreamCallback, thinkingCallback port.ThinkingCallback) (*entity.Message, []port.ToolCallInfo, error) {
	args := m.Called(ctx, messages, tools, textCallback, thinkingCallback)
	return args.Get(0).(*entity.Message), args.Get(1).([]port.ToolCallInfo), args.Error(2)
}

func (m *MockAIProvider) GenerateToolSchema() port.ToolInputSchemaParam {
	return nil
}

func (m *MockAIProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockAIProvider) SetModel(model string) error {
	return nil
}

func (m *MockAIProvider) GetModel() string {
	return "test-model"
}

func (m *MockAIProvider) Clone() port.AIProvider {
	return m
}

func TestRCAService_Correlate(t *testing.T) {
	tests := []struct {
		name         string
		findings     []entity.InvestigationFinding
		mockSetup    func(*MockAIProvider)
		expectedLen  int
		expectedErr  string
		validateFunc func(*testing.T, []entity.RCAFinding)
	}{
		{
			name: "successfully correlates findings into an RCA",
			findings: []entity.InvestigationFinding{
				{
					Type:        "observation",
					Description: "High CPU usage detected on instance web-01",
					Severity:    "high",
				},
				{
					Type:        "discovery",
					Description: "Process 'indexer' is consuming 95% of available CPU",
					Severity:    "critical",
				},
			},
			mockSetup: func(m *MockAIProvider) {
				aiResponseJSON := `{
					"findings": [
						{
							"summary": "CPU exhaustion caused by 'indexer' process",
							"causes": [
								{
									"id": "C1",
									"description": "Process 'indexer' resource leak",
									"confidence_score": 0.95,
									"evidence": ["CPU at 95%", "Process 'indexer' identified"]
								}
							],
							"remedies": [
								{
									"description": "Restart indexer process",
									"actionable_steps": ["systemctl restart indexer"],
									"impact": "High"
								},
								{
									"description": "Scale up CPU resources",
									"actionable_steps": ["Increase instance size"],
									"impact": "Medium"
								}
							]
						}
					]
				}`
				m.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(&entity.Message{
					Role:    entity.RoleAssistant,
					Content: aiResponseJSON,
				}, []port.ToolCallInfo(nil), nil)
			},
			expectedLen: 1,
			validateFunc: func(t *testing.T, findings []entity.RCAFinding) {
				assert.Equal(t, "CPU exhaustion caused by 'indexer' process", findings[0].Summary)
				assert.Equal(t, "C1", findings[0].Causes[0].ID)
				assert.Len(t, findings[0].Remedies, 2)
			},
		},
		{
			name:     "returns nil, nil when findings list is empty",
			findings: nil,
			mockSetup: func(m *MockAIProvider) {
				// No calls expected
			},
			expectedLen: 0,
		},
		{
			name:     "returns error when AI provider fails",
			findings: []entity.InvestigationFinding{{Description: "test"}},
			mockSetup: func(m *MockAIProvider) {
				m.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return((*entity.Message)(nil), []port.ToolCallInfo(nil), errors.New("ai error"))
			},
			expectedErr: "ai error",
		},
		{
			name:     "handles markdown-fenced JSON from AI",
			findings: []entity.InvestigationFinding{{Description: "test"}},
			mockSetup: func(m *MockAIProvider) {
				aiResponseJSON := "```json\n" + `{
					"findings": [
						{
							"summary": "markdown finding",
							"causes": [{"id": "c1", "description": "d1", "confidence_score": 0.9, "evidence": ["e1"]}],
							"remedies": [{"description": "r1", "actionable_steps": ["step"], "impact": "High"}]
						}
					]
				}` + "\n```"
				m.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(&entity.Message{
					Role:    entity.RoleAssistant,
					Content: aiResponseJSON,
				}, []port.ToolCallInfo(nil), nil)
			},
			expectedLen: 1,
			validateFunc: func(t *testing.T, findings []entity.RCAFinding) {
				assert.Equal(t, "markdown finding", findings[0].Summary)
			},
		},
		{
			name:     "returns error when AI returns unparseable content",
			findings: []entity.InvestigationFinding{{Description: "test"}},
			mockSetup: func(m *MockAIProvider) {
				m.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(&entity.Message{
					Role:    entity.RoleAssistant,
					Content: "this is not json",
				}, []port.ToolCallInfo(nil), nil)
			},
			expectedErr: "invalid character",
		},
		{
			name:     "returns error when finding fails Validate (e.g. no causes)",
			findings: []entity.InvestigationFinding{{Description: "test"}},
			mockSetup: func(m *MockAIProvider) {
				aiResponseJSON := `{
					"findings": [
						{
							"summary": "invalid finding",
							"causes": []
						}
					]
				}`
				m.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(&entity.Message{
					Role:    entity.RoleAssistant,
					Content: aiResponseJSON,
				}, []port.ToolCallInfo(nil), nil)
			},
			expectedErr: "invalid RCA finding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockAI := new(MockAIProvider)
			if tt.mockSetup != nil {
				tt.mockSetup(mockAI)
			}
			rcaService := service.NewRCAService(mockAI)

			rcaFindings, err := rcaService.Correlate(ctx, tt.findings)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, rcaFindings)
			} else {
				require.NoError(t, err)
				assert.Len(t, rcaFindings, tt.expectedLen)
				if tt.validateFunc != nil {
					tt.validateFunc(t, rcaFindings)
				}
			}
			mockAI.AssertExpectations(t)
		})
	}
}
