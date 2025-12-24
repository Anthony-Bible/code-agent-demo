package ai

import (
	"code-editing-agent/internal/domain/port"
	"testing"
)

// TestConvertTools_WithRequiredField verifies that when a tool has a required field,
// it's properly included in the output anthropic.ToolInputSchemaParam.Required field.
func TestConvertTools_WithRequiredField(t *testing.T) {
	// Setup: create an adapter
	adapter := &AnthropicAdapter{}

	// Input: a tool with a standard JSON schema including type, properties, and required
	tools := []port.ToolParam{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"path"},
			},
		},
	}

	// Execute: convert tools
	result := adapter.convertTools(tools)

	// Assert: we get back one tool
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	// Assert: the tool name and description are preserved
	if tool.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got '%s'", tool.Name)
	}
	if tool.Description.Value != "Read the contents of a file" {
		t.Errorf("expected description 'Read the contents of a file', got '%s'", tool.Description.Value)
	}

	// Assert: the input schema type is set correctly
	if tool.InputSchema.Type != "object" {
		t.Errorf("expected input schema type 'object', got '%s'", tool.InputSchema.Type)
	}

	// Assert: the required field is properly set
	if tool.InputSchema.Required == nil {
		t.Fatalf("expected Required field to be non-nil, got nil")
	}
	if len(tool.InputSchema.Required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(tool.InputSchema.Required))
	}
	if tool.InputSchema.Required[0] != "path" {
		t.Errorf("expected required field 'path', got '%s'", tool.InputSchema.Required[0])
	}

	// Assert: the properties contain only the property definitions, not type or required
	properties, ok := tool.InputSchema.Properties.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Properties to be map[string]interface{}, got %T", tool.InputSchema.Properties)
	}
	if len(properties) != 1 {
		t.Errorf("expected 1 property in Properties map, got %d", len(properties))
	}
	pathProperty, exists := properties["path"]
	if !exists {
		t.Fatal("expected property 'path' to exist in Properties map")
	}
	pathPropertyMap, ok := pathProperty.(map[string]interface{})
	if !ok {
		t.Fatalf("expected path property to be a map[string]interface{}, got %T", pathProperty)
	}
	if pathPropertyMap["type"] != "string" {
		t.Errorf("expected path property type 'string', got %v", pathPropertyMap["type"])
	}
}

// TestConvertTools_WithMultipleRequiredFields verifies that when a tool has multiple
// required fields, all of them are included in the output.
func TestConvertTools_WithMultipleRequiredFields(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "write_file",
			Description: "Write content to a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write to the file",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "The write mode",
					},
				},
				"required": []string{"path", "content"},
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	// Assert: the required field contains all required field names
	if tool.InputSchema.Required == nil {
		t.Fatalf("expected Required field to be non-nil, got nil")
	}
	if len(tool.InputSchema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(tool.InputSchema.Required))
	}

	// Check that each required field is present
	requiredSet := make(map[string]bool)
	for _, req := range tool.InputSchema.Required {
		requiredSet[req] = true
	}

	if !requiredSet["path"] {
		t.Error("expected 'path' to be in required fields")
	}
	if !requiredSet["content"] {
		t.Error("expected 'content' to be in required fields")
	}
}

// TestConvertTools_WithNoRequiredField verifies that when a tool has no required
// fields, the output Required field is either nil or empty.
func TestConvertTools_WithNoRequiredField(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	// Assert: the required field is either nil or empty
	if tool.InputSchema.Required != nil && len(tool.InputSchema.Required) > 0 {
		t.Errorf("expected Required field to be nil or empty, got %v", tool.InputSchema.Required)
	}
}

// TestConvertTools_PropertiesFieldIsCorrectlySet verifies that the properties field
// is correctly set and doesn't include type or required at the top level.
func TestConvertTools_PropertiesFieldIsCorrectlySet(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "complex_tool",
			Description: "A tool with complex schema",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"line_number": map[string]interface{}{
						"type":        "integer",
						"description": "Line number to edit",
					},
				},
				"required": []string{"file_path"},
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	properties, ok := tool.InputSchema.Properties.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Properties to be map[string]interface{}, got %T", tool.InputSchema.Properties)
	}

	// Assert: type and required are NOT in the Properties map
	if _, exists := properties["type"]; exists {
		t.Error("'type' should not be in the Properties map - it should be a separate field")
	}
	if _, exists := properties["required"]; exists {
		t.Error("'required' should not be in the Properties map - it should be a separate field")
	}

	// Assert: only property definitions are in Properties
	if len(properties) != 2 {
		t.Errorf("expected 2 properties in Properties map, got %d", len(properties))
	}

	// Verify each property exists and has correct structure
	filePathProp, exists := properties["file_path"]
	if !exists {
		t.Fatal("expected 'file_path' property to exist")
	}
	filePathMap, ok := filePathProp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected file_path property to be a map, got %T", filePathProp)
	}
	if filePathMap["type"] != "string" {
		t.Errorf("expected file_path type 'string', got %v", filePathMap["type"])
	}

	lineNumberProp, exists := properties["line_number"]
	if !exists {
		t.Fatal("expected 'line_number' property to exist")
	}
	lineNumberMap, ok := lineNumberProp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected line_number property to be a map, got %T", lineNumberProp)
	}
	if lineNumberMap["type"] != "integer" {
		t.Errorf("expected line_number type 'integer', got %v", lineNumberMap["type"])
	}
}

// TestConvertTools_EmptyInputSchema verifies that when a tool has an empty or nil
// input schema, the tool is still converted correctly.
func TestConvertTools_EmptyInputSchema(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "simple_tool",
			Description: "A tool with no input schema",
			InputSchema: nil,
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	if tool.Name != "simple_tool" {
		t.Errorf("expected tool name 'simple_tool', got '%s'", tool.Name)
	}

	// Properties should be nil since InputSchema was nil
	if tool.InputSchema.Properties != nil {
		t.Errorf("expected nil Properties, got %v", tool.InputSchema.Properties)
	}

	// Required should be nil
	if tool.InputSchema.Required != nil {
		t.Errorf("expected nil Required, got %v", tool.InputSchema.Required)
	}
}

// TestConvertTools_MultipleTools verifies that multiple tools are converted correctly,
// each preserving its own required fields and properties.
func TestConvertTools_MultipleTools(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "read_file",
			Description: "Read file contents",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write file contents",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "list_files",
			Description: "List directory contents",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
				// No required fields
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result))
	}

	// Verify first tool
	tool0 := result[0].OfTool
	if tool0.Name != "read_file" {
		t.Errorf("tool 0: expected name 'read_file', got '%s'", tool0.Name)
	}
	if len(tool0.InputSchema.Required) != 1 || tool0.InputSchema.Required[0] != "path" {
		t.Errorf("tool 0: expected Required [path], got %v", tool0.InputSchema.Required)
	}

	// Verify second tool
	tool1 := result[1].OfTool
	if tool1.Name != "write_file" {
		t.Errorf("tool 1: expected name 'write_file', got '%s'", tool1.Name)
	}
	if len(tool1.InputSchema.Required) != 2 {
		t.Errorf("tool 1: expected 2 required fields, got %d", len(tool1.InputSchema.Required))
	}

	// Verify third tool
	tool2 := result[2].OfTool
	if tool2.Name != "list_files" {
		t.Errorf("tool 2: expected name 'list_files', got '%s'", tool2.Name)
	}
	if tool2.InputSchema.Required != nil && len(tool2.InputSchema.Required) > 0 {
		t.Errorf("tool 2: expected no required fields, got %v", tool2.InputSchema.Required)
	}
}

// TestConvertTools_EmptySlice verifies that an empty input slice returns an empty output slice.
func TestConvertTools_EmptySlice(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{}

	result := adapter.convertTools(tools)

	if len(result) != 0 {
		t.Errorf("expected 0 tools, got %d", len(result))
	}
}

// TestConvertTools_RequiredPreservesOrder verifies that the order of required fields
// is preserved in the output.
func TestConvertTools_RequiredPreservesOrder(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"alpha":   map[string]interface{}{"type": "string"},
					"beta":    map[string]interface{}{"type": "string"},
					"gamma":   map[string]interface{}{"type": "string"},
					"delta":   map[string]interface{}{"type": "string"},
					"epsilon": map[string]interface{}{"type": "string"},
				},
				"required": []string{"beta", "alpha", "delta", "epsilon", "gamma"},
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	expectedOrder := []string{"beta", "alpha", "delta", "epsilon", "gamma"}

	if len(tool.InputSchema.Required) != len(expectedOrder) {
		t.Fatalf("expected %d required fields, got %d", len(expectedOrder), len(tool.InputSchema.Required))
	}

	for i, expected := range expectedOrder {
		if tool.InputSchema.Required[i] != expected {
			t.Errorf("required field at index %d: expected '%s', got '%s'", i, expected, tool.InputSchema.Required[i])
		}
	}
}
