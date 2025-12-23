package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"testing"
)

// TestAIProviderInterface_Contract validates that AIProvider interface exists with expected methods.
func TestAIProviderInterface_Contract(t *testing.T) {
	// Verify that AIProvider interface exists
	var _ AIProvider = (*mockAIProvider)(nil)
}

// mockAIProvider is a minimal implementation to validate interface contract
type mockAIProvider struct{}

func (m *mockAIProvider) SendMessage(
	ctx context.Context,
	messages []MessageParam,
	tools []ToolParam,
) (*entity.Message, []ToolCallInfo, error) {
	return nil, nil, nil
}

func (m *mockAIProvider) GenerateToolSchema() ToolInputSchemaParam {
	return make(ToolInputSchemaParam)
}

func (m *mockAIProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockAIProvider) SetModel(model string) error {
	return nil
}

func (m *mockAIProvider) GetModel() string {
	return ""
}

// TestAIProviderSendMessage_Exists validates SendMessage method exists.
func TestAIProviderSendMessage_Exists(t *testing.T) {
	var provider AIProvider = (*mockAIProvider)(nil)

	// This will fail to compile if SendMessage method doesn't exist with correct signature
	_ = provider.SendMessage
}

// TestAIProviderGenerateToolSchema_Exists validates GenerateToolSchema method exists.
func TestAIProviderGenerateToolSchema_Exists(t *testing.T) {
	var provider AIProvider = (*mockAIProvider)(nil)

	// This will fail to compile if GenerateToolSchema method doesn't exist with correct signature
	_ = provider.GenerateToolSchema
}

// TestAIProviderHealthCheck_Exists validates HealthCheck method exists.
func TestAIProviderHealthCheck_Exists(t *testing.T) {
	var provider AIProvider = (*mockAIProvider)(nil)

	// This will fail to compile if HealthCheck method doesn't exist with correct signature
	_ = provider.HealthCheck
}

// TestAIProviderSetGetModel_Exists validates SetModel and GetModel methods exist.
func TestAIProviderSetGetModel_Exists(t *testing.T) {
	var provider AIProvider = (*mockAIProvider)(nil)

	// This will fail to compile if SetModel and GetModel methods don't exist with correct signatures
	_ = provider.SetModel
	_ = provider.GetModel
}
