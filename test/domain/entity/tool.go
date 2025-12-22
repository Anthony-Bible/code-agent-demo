package entity

import (
	"encoding/json"
	"errors"
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

type Tool struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	InputSchema    map[string]interface{} `json:"input_schema,omitempty"`
	RequiredFields []string               `json:"required_fields,omitempty"`
}

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

func (t *Tool) Equals(other Tool) bool {
	if t.ID == "" || other.ID == "" {
		return false
	}
	return t.ID == other.ID
}

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

func (t *Tool) GetDescription() string {
	return t.Description
}
