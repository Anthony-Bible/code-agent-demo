package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrEmptyID          = errors.New("tool ID cannot be empty")
	ErrEmptyName        = errors.New("tool name cannot be empty")
	ErrEmptyDescription = errors.New("tool description cannot be empty")
	ErrEmptySchema      = errors.New("input schema cannot be empty")
	ErrNilSchema        = errors.New("input schema cannot be nil")
	ErrInvalidInput     = errors.New("invalid input JSON")
	ErrNilInput         = errors.New("input cannot be nil")
	ErrEmptyInput       = errors.New("input cannot be empty")
)

// Tool represents a computational tool that can be called with input parameters.
// It contains metadata about the tool and validation rules for its inputs.
type Tool struct {
	ID             string                 `json:"id"`                        // Unique identifier for the tool
	Name           string                 `json:"name"`                      // Human-readable name of the tool
	Description    string                 `json:"description"`               // Detailed description of what the tool does
	InputSchema    map[string]interface{} `json:"input_schema,omitempty"`    // JSON schema for validating tool inputs
	RequiredFields []string               `json:"required_fields,omitempty"` // List of required input field names
}

// NewTool creates a new tool with the specified ID, name, and description.
// All provided string fields must be non-empty and contain non-whitespace characters.
func NewTool(id, toolName, description string) (*Tool, error) {
	if id == "" || strings.TrimSpace(id) == "" {
		return nil, ErrEmptyID
	}
	if toolName == "" || strings.TrimSpace(toolName) == "" {
		return nil, ErrEmptyName
	}
	if description == "" || strings.TrimSpace(description) == "" {
		return nil, ErrEmptyDescription
	}

	return &Tool{
		ID:          id,
		Name:        toolName,
		Description: description,
	}, nil
}

// Validate checks if the tool has all required fields properly set.
// Returns an error if ID, name, or description are empty or whitespace-only.
func (t *Tool) Validate() error {
	if t.ID == "" || strings.TrimSpace(t.ID) == "" {
		return ErrEmptyID
	}
	if t.Name == "" || strings.TrimSpace(t.Name) == "" {
		return ErrEmptyName
	}
	if t.Description == "" || strings.TrimSpace(t.Description) == "" {
		return ErrEmptyDescription
	}
	return nil
}

// Equals checks if two tools have the same ID.
// Returns true if both tools have non-empty and matching IDs.
func (t *Tool) Equals(other Tool) bool {
	if t.ID == "" || other.ID == "" {
		return false
	}
	return t.ID == other.ID
}

// AddInputSchema sets the input schema and required fields for the tool.
// The schema must be non-nil and non-empty. The required fields slice is copied defensively.
func (t *Tool) AddInputSchema(schema map[string]interface{}, required []string) error {
	if schema == nil {
		return ErrNilSchema
	}
	if len(schema) == 0 {
		return ErrEmptySchema
	}

	t.InputSchema = schema
	if required != nil {
		t.RequiredFields = make([]string, len(required))
		copy(t.RequiredFields, required)
	}
	return nil
}

// HasRequired checks if a field name is in the required fields list.
// Returns false if the field name is empty or required fields is nil.
func (t *Tool) HasRequired(fieldName string) bool {
	if fieldName == "" || t.RequiredFields == nil {
		return false
	}
	for _, req := range t.RequiredFields {
		if req == fieldName {
			return true
		}
	}
	return false
}

// ValidateInput validates raw JSON input against the tool's required fields.
// The input must be valid JSON and contain all required fields.
func (t *Tool) ValidateInput(input json.RawMessage) error {
	if input == nil {
		return ErrNilInput
	}
	if len(input) == 0 {
		return ErrEmptyInput
	}

	var inputData map[string]interface{}
	if err := json.Unmarshal(input, &inputData); err != nil {
		return ErrInvalidInput
	}

	// Check required fields
	for _, req := range t.RequiredFields {
		if _, exists := inputData[req]; !exists {
			return errors.New("missing required field: " + req)
		}
	}

	return nil
}

// GetDescription returns the description of the tool.
func (t *Tool) GetDescription() string {
	return t.Description
}

// IsValid checks if the tool is valid without returning an error.
// Returns true if the tool passes all validation checks.
func (t *Tool) IsValid() bool {
	return t.Validate() == nil
}

// HasSchema returns true if the tool has an input schema defined.
func (t *Tool) HasSchema() bool {
	return t.InputSchema != nil
}

// GetRequiredFieldsCount returns the number of required fields.
func (t *Tool) GetRequiredFieldsCount() int {
	if t.RequiredFields == nil {
		return 0
	}
	return len(t.RequiredFields)
}

// String returns a string representation of the tool.
func (t *Tool) String() string {
	return fmt.Sprintf("Tool[%s]: %s", t.ID, t.Name)
}
