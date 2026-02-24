package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"encoding/json"
	"testing"
)

// TestAIProviderInterface_Contract validates that AIProvider interface exists with expected methods.
func TestAIProviderInterface_Contract(_ *testing.T) {
	// Verify that AIProvider interface exists
	var _ AIProvider = (*mockAIProvider)(nil)
}

// mockAIProvider is a minimal implementation to validate interface contract.
type mockAIProvider struct{}

func (m *mockAIProvider) SendMessage(
	_ context.Context,
	_ []MessageParam,
	_ []ToolParam,
) (*entity.Message, []ToolCallInfo, error) {
	return nil, nil, nil
}

func (m *mockAIProvider) SendMessageStreaming(
	_ context.Context,
	_ []MessageParam,
	_ []ToolParam,
	_ StreamCallback,
	_ ThinkingCallback,
) (*entity.Message, []ToolCallInfo, error) {
	return nil, nil, nil
}

func (m *mockAIProvider) GenerateToolSchema() ToolInputSchemaParam {
	return make(ToolInputSchemaParam)
}

func (m *mockAIProvider) HealthCheck(_ context.Context) error {
	return nil
}

func (m *mockAIProvider) SetModel(_ string) error {
	return nil
}

func (m *mockAIProvider) GetModel() string {
	return ""
}

// TestAIProviderSendMessage_Exists validates SendMessage method exists.
func TestAIProviderSendMessage_Exists(_ *testing.T) {
	var provider AIProvider = (*mockAIProvider)(nil)

	// This will fail to compile if SendMessage method doesn't exist with correct signature
	_ = provider.SendMessage
}

// TestAIProviderGenerateToolSchema_Exists validates GenerateToolSchema method exists.
func TestAIProviderGenerateToolSchema_Exists(_ *testing.T) {
	var provider AIProvider = (*mockAIProvider)(nil)

	// This will fail to compile if GenerateToolSchema method doesn't exist with correct signature
	_ = provider.GenerateToolSchema
}

// TestAIProviderHealthCheck_Exists validates HealthCheck method exists.
func TestAIProviderHealthCheck_Exists(_ *testing.T) {
	var provider AIProvider = (*mockAIProvider)(nil)

	// This will fail to compile if HealthCheck method doesn't exist with correct signature
	_ = provider.HealthCheck
}

// TestAIProviderSetGetModel_Exists validates SetModel and GetModel methods exist.
func TestAIProviderSetGetModel_Exists(_ *testing.T) {
	var provider AIProvider = (*mockAIProvider)(nil)

	// This will fail to compile if SetModel and GetModel methods don't exist with correct signatures
	_ = provider.SetModel
	_ = provider.GetModel
}

// TestThinkingBlockParam_JSONSerialization tests JSON serialization/deserialization of ThinkingBlockParam.
func TestThinkingBlockParam_JSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		block    ThinkingBlockParam
		wantJSON string
		wantErr  bool
	}{
		{
			name: "full thinking block with both fields",
			block: ThinkingBlockParam{
				Thinking:  "This is my reasoning process",
				Signature: "sig_abc123",
			},
			wantJSON: `{"thinking":"This is my reasoning process","signature":"sig_abc123"}`,
			wantErr:  false,
		},
		{
			name: "thinking block with only thinking field",
			block: ThinkingBlockParam{
				Thinking:  "Just thinking without signature",
				Signature: "",
			},
			wantJSON: `{"thinking":"Just thinking without signature","signature":""}`,
			wantErr:  false,
		},
		{
			name: "thinking block with empty thinking",
			block: ThinkingBlockParam{
				Thinking:  "",
				Signature: "sig_xyz789",
			},
			wantJSON: `{"thinking":"","signature":"sig_xyz789"}`,
			wantErr:  false,
		},
		{
			name: "empty thinking block",
			block: ThinkingBlockParam{
				Thinking:  "",
				Signature: "",
			},
			wantJSON: `{"thinking":"","signature":""}`,
			wantErr:  false,
		},
		{
			name: "thinking block with special characters",
			block: ThinkingBlockParam{
				Thinking:  "Thinking with \"quotes\" and\nnewlines",
				Signature: "sig_special_123",
			},
			wantJSON: `{"thinking":"Thinking with \"quotes\" and\nnewlines","signature":"sig_special_123"}`,
			wantErr:  false,
		},
		{
			name: "thinking block with unicode characters",
			block: ThinkingBlockParam{
				Thinking:  "思考过程 with emoji 🤔",
				Signature: "sig_unicode_αβγ",
			},
			wantJSON: `{"thinking":"思考过程 with emoji 🤔","signature":"sig_unicode_αβγ"}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			gotJSON, err := json.Marshal(tt.block)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(gotJSON) != tt.wantJSON {
				t.Errorf("json.Marshal() = %v, want %v", string(gotJSON), tt.wantJSON)
			}

			// Test unmarshaling
			var unmarshaled ThinkingBlockParam
			err = json.Unmarshal([]byte(tt.wantJSON), &unmarshaled)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if unmarshaled.Thinking != tt.block.Thinking {
					t.Errorf("json.Unmarshal() Thinking = %v, want %v", unmarshaled.Thinking, tt.block.Thinking)
				}
				if unmarshaled.Signature != tt.block.Signature {
					t.Errorf("json.Unmarshal() Signature = %v, want %v", unmarshaled.Signature, tt.block.Signature)
				}
			}
		})
	}
}

// TestThinkingBlockParam_JSONDeserialization tests deserializing various JSON formats.
func TestThinkingBlockParam_JSONDeserialization(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    ThinkingBlockParam
		wantErr bool
	}{
		{
			name: "valid JSON with both fields",
			json: `{"thinking":"test thinking","signature":"test_sig"}`,
			want: ThinkingBlockParam{
				Thinking:  "test thinking",
				Signature: "test_sig",
			},
			wantErr: false,
		},
		{
			name: "JSON with extra fields should be ignored",
			json: `{"thinking":"test","signature":"sig","extra":"ignored"}`,
			want: ThinkingBlockParam{
				Thinking:  "test",
				Signature: "sig",
			},
			wantErr: false,
		},
		{
			name: "JSON with missing thinking field",
			json: `{"signature":"sig_only"}`,
			want: ThinkingBlockParam{
				Thinking:  "",
				Signature: "sig_only",
			},
			wantErr: false,
		},
		{
			name: "JSON with missing signature field",
			json: `{"thinking":"thinking_only"}`,
			want: ThinkingBlockParam{
				Thinking:  "thinking_only",
				Signature: "",
			},
			wantErr: false,
		},
		{
			name:    "empty JSON object",
			json:    `{}`,
			want:    ThinkingBlockParam{},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			json:    `{invalid}`,
			want:    ThinkingBlockParam{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ThinkingBlockParam
			err := json.Unmarshal([]byte(tt.json), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Thinking != tt.want.Thinking {
					t.Errorf("json.Unmarshal() Thinking = %v, want %v", got.Thinking, tt.want.Thinking)
				}
				if got.Signature != tt.want.Signature {
					t.Errorf("json.Unmarshal() Signature = %v, want %v", got.Signature, tt.want.Signature)
				}
			}
		})
	}
}

// TestMessageParam_WithThinkingBlocks tests MessageParam with ThinkingBlocks field.
func TestMessageParam_WithThinkingBlocks(t *testing.T) {
	tests := []struct {
		name    string
		msg     MessageParam
		wantErr bool
	}{
		{
			name: "message with single thinking block",
			msg: MessageParam{
				Role:    "assistant",
				Content: "Here's my answer",
				ThinkingBlocks: []ThinkingBlockParam{
					{
						Thinking:  "First I need to analyze the problem",
						Signature: "sig_001",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "message with multiple thinking blocks",
			msg: MessageParam{
				Role:    "assistant",
				Content: "Final response",
				ThinkingBlocks: []ThinkingBlockParam{
					{
						Thinking:  "Step 1: Understand requirements",
						Signature: "sig_001",
					},
					{
						Thinking:  "Step 2: Design solution",
						Signature: "sig_002",
					},
					{
						Thinking:  "Step 3: Implement",
						Signature: "sig_003",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "message with empty thinking blocks array",
			msg: MessageParam{
				Role:           "assistant",
				Content:        "Response without thinking",
				ThinkingBlocks: []ThinkingBlockParam{},
			},
			wantErr: false,
		},
		{
			name: "message with nil thinking blocks",
			msg: MessageParam{
				Role:           "assistant",
				Content:        "Response without thinking",
				ThinkingBlocks: nil,
			},
			wantErr: false,
		},
		{
			name: "message with thinking blocks and tool calls",
			msg: MessageParam{
				Role:    "assistant",
				Content: "",
				ThinkingBlocks: []ThinkingBlockParam{
					{
						Thinking:  "I need to use a tool",
						Signature: "sig_001",
					},
				},
				ToolCalls: []ToolCallParam{
					{
						ToolID:   "call_001",
						ToolName: "bash",
						Input:    map[string]interface{}{"command": "ls"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "message with only thinking blocks and no content",
			msg: MessageParam{
				Role:    "assistant",
				Content: "",
				ThinkingBlocks: []ThinkingBlockParam{
					{
						Thinking:  "Pure thinking without visible content",
						Signature: "sig_001",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the struct can be created
			msg := tt.msg

			// Verify fields are set correctly
			if msg.Role != tt.msg.Role {
				t.Errorf("Role = %v, want %v", msg.Role, tt.msg.Role)
			}
			if msg.Content != tt.msg.Content {
				t.Errorf("Content = %v, want %v", msg.Content, tt.msg.Content)
			}
			if len(msg.ThinkingBlocks) != len(tt.msg.ThinkingBlocks) {
				t.Errorf("ThinkingBlocks length = %v, want %v", len(msg.ThinkingBlocks), len(tt.msg.ThinkingBlocks))
			}
		})
	}
}

// TestMessageParam_ThinkingBlocksJSON tests JSON serialization of MessageParam with ThinkingBlocks.
func TestMessageParam_ThinkingBlocksJSON(t *testing.T) {
	tests := []struct {
		name     string
		msg      MessageParam
		wantJSON string
		wantErr  bool
	}{
		{
			name: "message with thinking blocks serializes correctly",
			msg: MessageParam{
				Role:    "assistant",
				Content: "Response",
				ThinkingBlocks: []ThinkingBlockParam{
					{
						Thinking:  "Reasoning",
						Signature: "sig_1",
					},
				},
			},
			wantJSON: `{"role":"assistant","content":"Response","thinking_blocks":[{"thinking":"Reasoning","signature":"sig_1"}]}`,
			wantErr:  false,
		},
		{
			name: "message with empty thinking blocks omits field",
			msg: MessageParam{
				Role:           "assistant",
				Content:        "Response",
				ThinkingBlocks: []ThinkingBlockParam{},
			},
			wantJSON: `{"role":"assistant","content":"Response"}`,
			wantErr:  false,
		},
		{
			name: "message with nil thinking blocks omits field",
			msg: MessageParam{
				Role:           "assistant",
				Content:        "Response",
				ThinkingBlocks: nil,
			},
			wantJSON: `{"role":"assistant","content":"Response"}`,
			wantErr:  false,
		},
		{
			name: "message with multiple thinking blocks",
			msg: MessageParam{
				Role:    "assistant",
				Content: "Final",
				ThinkingBlocks: []ThinkingBlockParam{
					{Thinking: "Think 1", Signature: "sig_1"},
					{Thinking: "Think 2", Signature: "sig_2"},
				},
			},
			wantJSON: `{"role":"assistant","content":"Final","thinking_blocks":[{"thinking":"Think 1","signature":"sig_1"},{"thinking":"Think 2","signature":"sig_2"}]}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, err := json.Marshal(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(gotJSON) != tt.wantJSON {
				t.Errorf("json.Marshal() = %v, want %v", string(gotJSON), tt.wantJSON)
			}

			// Test round-trip
			if !tt.wantErr {
				var unmarshaled MessageParam
				err = json.Unmarshal(gotJSON, &unmarshaled)
				if err != nil {
					t.Errorf("json.Unmarshal() error = %v", err)
					return
				}
				if unmarshaled.Role != tt.msg.Role {
					t.Errorf("Round-trip Role = %v, want %v", unmarshaled.Role, tt.msg.Role)
				}
				if unmarshaled.Content != tt.msg.Content {
					t.Errorf("Round-trip Content = %v, want %v", unmarshaled.Content, tt.msg.Content)
				}
				if len(unmarshaled.ThinkingBlocks) != len(tt.msg.ThinkingBlocks) {
					t.Errorf(
						"Round-trip ThinkingBlocks length = %v, want %v",
						len(unmarshaled.ThinkingBlocks),
						len(tt.msg.ThinkingBlocks),
					)
				}
			}
		})
	}
}

// TestConvertEntityThinkingBlockToParam tests conversion from entity.ThinkingBlock to port.ThinkingBlockParam.
func TestConvertEntityThinkingBlockToParam(t *testing.T) {
	tests := []struct {
		name   string
		entity entity.ThinkingBlock
		want   ThinkingBlockParam
	}{
		{
			name: "full thinking block",
			entity: entity.ThinkingBlock{
				Thinking:  "Entity thinking content",
				Signature: "entity_sig_001",
			},
			want: ThinkingBlockParam{
				Thinking:  "Entity thinking content",
				Signature: "entity_sig_001",
			},
		},
		{
			name: "thinking block with only thinking",
			entity: entity.ThinkingBlock{
				Thinking:  "Only thinking",
				Signature: "",
			},
			want: ThinkingBlockParam{
				Thinking:  "Only thinking",
				Signature: "",
			},
		},
		{
			name: "thinking block with only signature",
			entity: entity.ThinkingBlock{
				Thinking:  "",
				Signature: "only_sig",
			},
			want: ThinkingBlockParam{
				Thinking:  "",
				Signature: "only_sig",
			},
		},
		{
			name:   "empty thinking block",
			entity: entity.ThinkingBlock{},
			want:   ThinkingBlockParam{},
		},
		{
			name: "thinking block with special characters",
			entity: entity.ThinkingBlock{
				Thinking:  "Special chars: \n\t\"quotes\"",
				Signature: "sig_special",
			},
			want: ThinkingBlockParam{
				Thinking:  "Special chars: \n\t\"quotes\"",
				Signature: "sig_special",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will define the expected conversion function behavior
			// The actual conversion function doesn't exist yet
			got := ConvertEntityThinkingBlockToParam(tt.entity)
			if got.Thinking != tt.want.Thinking {
				t.Errorf("ConvertEntityThinkingBlockToParam() Thinking = %v, want %v", got.Thinking, tt.want.Thinking)
			}
			if got.Signature != tt.want.Signature {
				t.Errorf(
					"ConvertEntityThinkingBlockToParam() Signature = %v, want %v",
					got.Signature,
					tt.want.Signature,
				)
			}
		})
	}
}

// TestConvertParamThinkingBlockToEntity tests conversion from port.ThinkingBlockParam to entity.ThinkingBlock.
func TestConvertParamThinkingBlockToEntity(t *testing.T) {
	tests := []struct {
		name  string
		param ThinkingBlockParam
		want  entity.ThinkingBlock
	}{
		{
			name: "full thinking block param",
			param: ThinkingBlockParam{
				Thinking:  "Param thinking content",
				Signature: "param_sig_001",
			},
			want: entity.ThinkingBlock{
				Thinking:  "Param thinking content",
				Signature: "param_sig_001",
			},
		},
		{
			name: "param with only thinking",
			param: ThinkingBlockParam{
				Thinking:  "Only thinking",
				Signature: "",
			},
			want: entity.ThinkingBlock{
				Thinking:  "Only thinking",
				Signature: "",
			},
		},
		{
			name: "param with only signature",
			param: ThinkingBlockParam{
				Thinking:  "",
				Signature: "only_sig",
			},
			want: entity.ThinkingBlock{
				Thinking:  "",
				Signature: "only_sig",
			},
		},
		{
			name:  "empty thinking block param",
			param: ThinkingBlockParam{},
			want:  entity.ThinkingBlock{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will define the expected conversion function behavior
			// The actual conversion function doesn't exist yet
			got := ConvertParamThinkingBlockToEntity(tt.param)
			if got.Thinking != tt.want.Thinking {
				t.Errorf("ConvertParamThinkingBlockToEntity() Thinking = %v, want %v", got.Thinking, tt.want.Thinking)
			}
			if got.Signature != tt.want.Signature {
				t.Errorf(
					"ConvertParamThinkingBlockToEntity() Signature = %v, want %v",
					got.Signature,
					tt.want.Signature,
				)
			}
		})
	}
}

// TestConvertEntityThinkingBlocksToParams tests batch conversion from entity to param.
func TestConvertEntityThinkingBlocksToParams(t *testing.T) {
	tests := []struct {
		name     string
		entities []entity.ThinkingBlock
		want     []ThinkingBlockParam
	}{
		{
			name: "multiple thinking blocks",
			entities: []entity.ThinkingBlock{
				{Thinking: "First", Signature: "sig_1"},
				{Thinking: "Second", Signature: "sig_2"},
				{Thinking: "Third", Signature: "sig_3"},
			},
			want: []ThinkingBlockParam{
				{Thinking: "First", Signature: "sig_1"},
				{Thinking: "Second", Signature: "sig_2"},
				{Thinking: "Third", Signature: "sig_3"},
			},
		},
		{
			name: "single thinking block",
			entities: []entity.ThinkingBlock{
				{Thinking: "Only one", Signature: "sig_1"},
			},
			want: []ThinkingBlockParam{
				{Thinking: "Only one", Signature: "sig_1"},
			},
		},
		{
			name:     "empty slice",
			entities: []entity.ThinkingBlock{},
			want:     []ThinkingBlockParam{},
		},
		{
			name:     "nil slice",
			entities: nil,
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertEntityThinkingBlocksToParams(tt.entities)
			if len(got) != len(tt.want) {
				t.Errorf("ConvertEntityThinkingBlocksToParams() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].Thinking != tt.want[i].Thinking {
					t.Errorf(
						"ConvertEntityThinkingBlocksToParams()[%d] Thinking = %v, want %v",
						i,
						got[i].Thinking,
						tt.want[i].Thinking,
					)
				}
				if got[i].Signature != tt.want[i].Signature {
					t.Errorf(
						"ConvertEntityThinkingBlocksToParams()[%d] Signature = %v, want %v",
						i,
						got[i].Signature,
						tt.want[i].Signature,
					)
				}
			}
		})
	}
}

// TestConvertParamThinkingBlocksToEntities tests batch conversion from param to entity.
func TestConvertParamThinkingBlocksToEntities(t *testing.T) {
	tests := []struct {
		name   string
		params []ThinkingBlockParam
		want   []entity.ThinkingBlock
	}{
		{
			name: "multiple thinking block params",
			params: []ThinkingBlockParam{
				{Thinking: "First", Signature: "sig_1"},
				{Thinking: "Second", Signature: "sig_2"},
			},
			want: []entity.ThinkingBlock{
				{Thinking: "First", Signature: "sig_1"},
				{Thinking: "Second", Signature: "sig_2"},
			},
		},
		{
			name: "single thinking block param",
			params: []ThinkingBlockParam{
				{Thinking: "Only one", Signature: "sig_1"},
			},
			want: []entity.ThinkingBlock{
				{Thinking: "Only one", Signature: "sig_1"},
			},
		},
		{
			name:   "empty slice",
			params: []ThinkingBlockParam{},
			want:   []entity.ThinkingBlock{},
		},
		{
			name:   "nil slice",
			params: nil,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertParamThinkingBlocksToEntities(tt.params)
			if len(got) != len(tt.want) {
				t.Errorf("ConvertParamThinkingBlocksToEntities() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].Thinking != tt.want[i].Thinking {
					t.Errorf(
						"ConvertParamThinkingBlocksToEntities()[%d] Thinking = %v, want %v",
						i,
						got[i].Thinking,
						tt.want[i].Thinking,
					)
				}
				if got[i].Signature != tt.want[i].Signature {
					t.Errorf(
						"ConvertParamThinkingBlocksToEntities()[%d] Signature = %v, want %v",
						i,
						got[i].Signature,
						tt.want[i].Signature,
					)
				}
			}
		})
	}
}

// TestThinkingBlockParam_NilHandling tests that nil thinking blocks are handled correctly.
func TestThinkingBlockParam_NilHandling(t *testing.T) {
	tests := []struct {
		name string
		msg  MessageParam
	}{
		{
			name: "message with nil thinking blocks",
			msg: MessageParam{
				Role:           "assistant",
				Content:        "Content",
				ThinkingBlocks: nil,
			},
		},
		{
			name: "message with explicitly empty thinking blocks",
			msg: MessageParam{
				Role:           "assistant",
				Content:        "Content",
				ThinkingBlocks: []ThinkingBlockParam{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that nil and empty slices behave consistently in JSON
			jsonData, err := json.Marshal(tt.msg)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
				return
			}

			var unmarshaled MessageParam
			err = json.Unmarshal(jsonData, &unmarshaled)
			if err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
				return
			}

			// After unmarshaling, both nil and empty slices should be consistent
			if tt.msg.ThinkingBlocks == nil && unmarshaled.ThinkingBlocks != nil &&
				len(unmarshaled.ThinkingBlocks) > 0 {
				t.Errorf("nil ThinkingBlocks should remain nil or empty after round-trip")
			}
		})
	}
}

// TestThinkingBlockParam_EmptyArrayHandling tests that empty arrays are handled correctly.
func TestThinkingBlockParam_EmptyArrayHandling(t *testing.T) {
	tests := []struct {
		name string
		json string
		want int // expected length of ThinkingBlocks
	}{
		{
			name: "JSON with empty thinking_blocks array",
			json: `{"role":"assistant","content":"test","thinking_blocks":[]}`,
			want: 0,
		},
		{
			name: "JSON without thinking_blocks field",
			json: `{"role":"assistant","content":"test"}`,
			want: 0,
		},
		{
			name: "JSON with null thinking_blocks",
			json: `{"role":"assistant","content":"test","thinking_blocks":null}`,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg MessageParam
			err := json.Unmarshal([]byte(tt.json), &msg)
			if err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
				return
			}

			gotLen := 0
			if msg.ThinkingBlocks != nil {
				gotLen = len(msg.ThinkingBlocks)
			}

			if gotLen != tt.want {
				t.Errorf("ThinkingBlocks length = %v, want %v", gotLen, tt.want)
			}
		})
	}
}
