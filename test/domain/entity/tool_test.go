package entity

import (
	"encoding/json"
	"testing"
)

func TestTool_NewTool(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		toolName    string
		description string
		want        *Tool
		wantErr     bool
	}{
		{
			name:        "should create valid tool",
			id:          "read_file",
			toolName:    "read_file",
			description: "Reads the contents of a file",
			want:        &Tool{ID: "read_file", Name: "read_file", Description: "Reads the contents of a file"},
			wantErr:     false,
		},
		{
			name:        "should create valid tool with complex description",
			id:          "edit_file",
			toolName:    "edit_file",
			description: "Makes edits to a text file by replacing strings",
			want: &Tool{
				ID:          "edit_file",
				Name:        "edit_file",
				Description: "Makes edits to a text file by replacing strings",
			},
			wantErr: false,
		},
		{
			name:        "should reject tool with empty ID",
			id:          "",
			toolName:    "read_file",
			description: "Reads file",
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "should reject tool with empty name",
			id:          "read_file",
			toolName:    "",
			description: "Reads file",
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "should reject tool with empty description",
			id:          "read_file",
			toolName:    "read_file",
			description: "",
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "should reject tool with whitespace-only ID",
			id:          "   ",
			toolName:    "read_file",
			description: "Reads file",
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "should reject tool with whitespace-only name",
			id:          "read_file",
			toolName:    "   ",
			description: "Reads file",
			want:        nil,
			wantErr:     true,
		},
		{
			name:        "should reject tool with whitespace-only description",
			id:          "read_file",
			toolName:    "read_file",
			description: "   ",
			want:        nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTool(tt.id, tt.toolName, tt.description)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == nil {
					t.Error("NewTool() returned nil tool")
					return
				}
				if got.ID != tt.id {
					t.Errorf("NewTool() ID = %v, want %v", got.ID, tt.id)
				}
				if got.Name != tt.toolName {
					t.Errorf("NewTool() name = %v, want %v", got.Name, tt.toolName)
				}
				if got.Description != tt.description {
					t.Errorf("NewTool() description = %v, want %v", got.Description, tt.description)
				}
			} else {
				if got != nil {
					t.Errorf("NewTool() returned non-nil tool on error: %+v", got)
				}
			}
		})
	}
}

func TestTool_Validate(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		wantErr bool
	}{
		{
			name:    "valid tool should pass validation",
			tool:    Tool{ID: "read_file", Name: "read_file", Description: "Reads file contents"},
			wantErr: false,
		},
		{
			name:    "tool with empty ID should fail validation",
			tool:    Tool{ID: "", Name: "read_file", Description: "Reads file contents"},
			wantErr: true,
		},
		{
			name:    "tool with empty name should fail validation",
			tool:    Tool{ID: "read_file", Name: "", Description: "Reads file contents"},
			wantErr: true,
		},
		{
			name:    "tool with empty description should fail validation",
			tool:    Tool{ID: "read_file", Name: "read_file", Description: ""},
			wantErr: true,
		},
		{
			name:    "tool with whitespace ID should fail validation",
			tool:    Tool{ID: "  ", Name: "read_file", Description: "Reads file contents"},
			wantErr: true,
		},
		{
			name:    "tool with whitespace name should fail validation",
			tool:    Tool{ID: "read_file", Name: "  ", Description: "Reads file contents"},
			wantErr: true,
		},
		{
			name:    "tool with whitespace description should fail validation",
			tool:    Tool{ID: "read_file", Name: "read_file", Description: "  "},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tool.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Tool.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTool_Equals(t *testing.T) {
	tests := []struct {
		name  string
		tool  Tool
		other Tool
		want  bool
	}{
		{
			name:  "tools with same ID should be equal",
			tool:  Tool{ID: "read_file", Name: "read_file", Description: "Reads file"},
			other: Tool{ID: "read_file", Name: "file_reader", Description: "Different description"},
			want:  true,
		},
		{
			name:  "tools with different IDs should not be equal",
			tool:  Tool{ID: "read_file", Name: "read_file", Description: "Reads file"},
			other: Tool{ID: "write_file", Name: "write_file", Description: "Writes file"},
			want:  false,
		},
		{
			name:  "same reference tool should be equal",
			tool:  Tool{ID: "edit_file", Name: "edit_file", Description: "Edits file"},
			other: Tool{ID: "edit_file", Name: "edit_file", Description: "Edits file"},
			want:  true,
		},
		{
			name:  "tool with empty IDs should not be equal",
			tool:  Tool{ID: "", Name: "tool1", Description: "Desc1"},
			other: Tool{ID: "", Name: "tool2", Description: "Desc2"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tool.Equals(tt.other); got != tt.want {
				t.Errorf("Tool.Equals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTool_AddInputSchema(t *testing.T) {
	type fields struct {
		ID             string
		Name           string
		Description    string
		InputSchema    map[string]interface{}
		RequiredFields []string
	}
	type args struct {
		schema   map[string]interface{}
		required []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "should add valid input schema",
			fields: fields{ID: "read_file", Name: "read_file", Description: "Reads file"},
			args: args{
				schema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path to read",
						},
					},
				},
				required: []string{"path"},
			},
			wantErr: false,
		},
		{
			name:   "should add schema without required fields",
			fields: fields{ID: "list_files", Name: "list_files", Description: "Lists files"},
			args: args{
				schema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path",
						},
					},
				},
				required: []string{},
			},
			wantErr: false,
		},
		{
			name:   "should reject nil schema",
			fields: fields{ID: "edit_file", Name: "edit_file", Description: "Edits file"},
			args: args{
				schema:   nil,
				required: []string{"path"},
			},
			wantErr: true,
		},
		{
			name:   "should reject empty schema",
			fields: fields{ID: "edit_file", Name: "edit_file", Description: "Edits file"},
			args: args{
				schema:   map[string]interface{}{},
				required: []string{"path"},
			},
			wantErr: true,
		},
		{
			name:   "should accept nil required fields",
			fields: fields{ID: "read_file", Name: "read_file", Description: "Reads file"},
			args: args{
				schema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path to read",
						},
					},
				},
				required: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := &Tool{
				ID:             tt.fields.ID,
				Name:           tt.fields.Name,
				Description:    tt.fields.Description,
				InputSchema:    tt.fields.InputSchema,
				RequiredFields: tt.fields.RequiredFields,
			}
			err := tl.AddInputSchema(tt.args.schema, tt.args.required)
			if (err != nil) != tt.wantErr {
				t.Errorf("Tool.AddInputSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tl.InputSchema == nil {
					t.Error("Tool.AddInputSchema() input schema should not be nil")
				}
				if len(tt.args.required) > 0 && len(tl.RequiredFields) == 0 {
					t.Error("Tool.AddInputSchema() required fields should not be empty when provided")
				}
				for i, req := range tt.args.required {
					if i >= len(tl.RequiredFields) || tl.RequiredFields[i] != req {
						t.Errorf("Tool.AddInputSchema() required field %d = %v, want %v", i, tl.RequiredFields[i], req)
					}
				}
			}
		})
	}
}

func TestTool_HasRequired(t *testing.T) {
	type fields struct {
		RequiredFields []string
	}
	type args struct {
		fieldName string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "should return true for required field",
			fields: fields{RequiredFields: []string{"path", "content"}},
			args:   args{fieldName: "path"},
			want:   true,
		},
		{
			name:   "should return false for non-required field",
			fields: fields{RequiredFields: []string{"path", "content"}},
			args:   args{fieldName: "optional"},
			want:   false,
		},
		{
			name:   "should return false for empty required fields",
			fields: fields{RequiredFields: []string{}},
			args:   args{fieldName: "path"},
			want:   false,
		},
		{
			name:   "should return false for nil required fields",
			fields: fields{RequiredFields: nil},
			args:   args{fieldName: "path"},
			want:   false,
		},
		{
			name:   "should handle empty field name",
			fields: fields{RequiredFields: []string{"path"}},
			args:   args{fieldName: ""},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := &Tool{
				RequiredFields: tt.fields.RequiredFields,
			}
			got := tl.HasRequired(tt.args.fieldName)
			if got != tt.want {
				t.Errorf("Tool.HasRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTool_ValidateInput(t *testing.T) {
	type fields struct {
		InputSchema    map[string]interface{}
		RequiredFields []string
	}
	type args struct {
		input json.RawMessage
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "should validate valid JSON input",
			fields: fields{
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type": "string",
						},
					},
				},
				RequiredFields: []string{"path"},
			},
			args:    args{input: json.RawMessage(`{"path": "/tmp/file.txt"}`)},
			wantErr: false,
		},
		{
			name: "should reject missing required field",
			fields: fields{
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type": "string",
						},
					},
				},
				RequiredFields: []string{"path"},
			},
			args:    args{input: json.RawMessage(`{"other": "value"}`)},
			wantErr: true,
		},
		{
			name: "should accept missing optional field",
			fields: fields{
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type": "string",
						},
						"optional": map[string]interface{}{
							"type": "string",
						},
					},
				},
				RequiredFields: []string{"path"},
			},
			args:    args{input: json.RawMessage(`{"path": "/tmp/file.txt"}`)},
			wantErr: false,
		},
		{
			name: "should reject invalid JSON",
			fields: fields{
				InputSchema: map[string]interface{}{
					"type": "object",
				},
				RequiredFields: []string{},
			},
			args:    args{input: json.RawMessage(`invalid json`)},
			wantErr: true,
		},
		{
			name:    "should reject nil input",
			fields:  fields{RequiredFields: []string{}},
			args:    args{input: nil},
			wantErr: true,
		},
		{
			name:    "should reject empty input",
			fields:  fields{RequiredFields: []string{}},
			args:    args{input: json.RawMessage{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := &Tool{
				InputSchema:    tt.fields.InputSchema,
				RequiredFields: tt.fields.RequiredFields,
			}
			err := tl.ValidateInput(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Tool.ValidateInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTool_GetDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "should return tool description",
			description: "Reads the contents of a file from the workspace",
			want:        "Reads the contents of a file from the workspace",
		},
		{
			name:        "should return empty description",
			description: "",
			want:        "",
		},
		{
			name:        "should return description with special characters",
			description: "Edits a file. Replaces 'old_str' with 'new_str'.",
			want:        "Edits a file. Replaces 'old_str' with 'new_str'.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := &Tool{Description: tt.description}
			if got := tl.GetDescription(); got != tt.want {
				t.Errorf("Tool.GetDescription() = %v, want %v", got, tt.want)
			}
		})
	}
}
