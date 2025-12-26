// Package dto provides Data Transfer Objects for the application layer.
package dto

import "encoding/json"

// ToolExecuteRequest represents a request to execute a tool.
// It contains the tool name and input parameters.
type ToolExecuteRequest struct {
	ToolName string      `json:"tool_name"` // The name of the tool to execute
	Input    interface{} `json:"input"`     // Tool input parameters (can be any JSON-serializable type)
}

// Validate checks if the ToolExecuteRequest is valid.
// Returns an error if tool name is empty or input is nil.
func (r *ToolExecuteRequest) Validate() error {
	if r.ToolName == "" {
		return ErrEmptyToolName
	}
	if r.Input == nil {
		return ErrNilToolInput
	}
	return nil
}

// ToolExecuteBatchRequest represents a request to execute multiple tools.
// This is useful when the AI requests multiple tool calls in a single response.
type ToolExecuteBatchRequest struct {
	SessionID string               `json:"session_id"` // The conversation session ID
	Tools     []ToolExecuteRequest `json:"tools"`      // List of tools to execute
}

// Validate checks if the ToolExecuteBatchRequest is valid.
// Returns an error if session ID is empty or tools list is empty.
func (r *ToolExecuteBatchRequest) Validate() error {
	if r.SessionID == "" {
		return ErrEmptySessionID
	}
	if len(r.Tools) == 0 {
		return ErrEmptyToolList
	}
	for i, tool := range r.Tools {
		if err := tool.Validate(); err != nil {
			return ToolValidationError{Index: i, ToolName: tool.ToolName, Err: err}
		}
	}
	return nil
}

// ToolDefinition represents a tool definition that can be registered.
// It contains the tool's metadata and input schema.
type ToolDefinition struct {
	Name        string                 `json:"name"`                   // Human-readable name of the tool
	Description string                 `json:"description"`            // What the tool does
	ID          string                 `json:"id"`                     // Unique identifier for the tool
	InputSchema map[string]interface{} `json:"input_schema,omitempty"` // JSON schema for validation
}

// Validate checks if the ToolDefinition is valid.
// Returns an error if name, description, or ID is empty.
func (d *ToolDefinition) Validate() error {
	if d.Name == "" {
		return ErrEmptyToolName
	}
	if d.Description == "" {
		return ErrEmptyToolDescription
	}
	if d.ID == "" {
		return ErrEmptyToolID
	}
	return nil
}

// ToJSON converts the ToolDefinition to a JSON string.
func (d *ToolDefinition) ToJSON() (string, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSONToolDefinition parses a ToolDefinition from a JSON string.
func FromJSONToolDefinition(jsonStr string) (*ToolDefinition, error) {
	var def ToolDefinition
	err := json.Unmarshal([]byte(jsonStr), &def)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

// ToolValidationError represents an error in validation of a specific tool in a batch.
type ToolValidationError struct {
	Index    int    // Index of the tool in the batch
	ToolName string // Name of the tool that failed validation
	Err      error  // The underlying validation error
}

// Error implements the error interface.
func (e ToolValidationError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e ToolValidationError) Unwrap() error {
	return e.Err
}
