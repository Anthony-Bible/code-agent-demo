package entity

import (
	"errors"
	"strings"
	"time"
)

// Validator defines the interface for entity validation.
type Validator interface {
	Validate() error
}

// ValidationHelper provides common validation functions used across entities.
type ValidationHelper struct{}

// NewValidationHelper creates a new validation helper instance.
func NewValidationHelper() *ValidationHelper {
	return &ValidationHelper{}
}

// ValidateNotEmpty checks if a string is not empty after trimming whitespace.
func (v *ValidationHelper) ValidateNotEmpty(value, fieldName string) error {
	if value == "" {
		return NewValidationError(fieldName, "cannot be empty")
	}
	if strings.TrimSpace(value) == "" {
		return NewValidationError(fieldName, "cannot be whitespace only")
	}
	return nil
}

// ValidateEnum checks if a string value is one of the allowed enum values.
func (v *ValidationHelper) ValidateEnum(value, fieldName string, allowedValues []string) error {
	if err := v.ValidateNotEmpty(value, fieldName); err != nil {
		return err
	}

	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}

	return NewValidationError(fieldName,
		"must be one of: "+strings.Join(allowedValues, ", "))
}

// ValidateNotNil checks if a pointer or interface is not nil.
func (v *ValidationHelper) ValidateNotNil(value interface{}, fieldName string) error {
	if value == nil {
		return NewValidationError(fieldName, "cannot be nil")
	}
	return nil
}

// ValidateNotEmptyMap checks if a map is not empty.
func (v *ValidationHelper) ValidateNotEmptyMap(value map[string]interface{}, fieldName string) error {
	if value == nil {
		return NewValidationError(fieldName, "cannot be nil")
	}
	if len(value) == 0 {
		return NewValidationError(fieldName, "cannot be empty")
	}
	return nil
}

// ValidateTimestamp checks if a timestamp is not zero.
func (v *ValidationHelper) ValidateTimestamp(ts time.Time, fieldName string) error {
	if ts.IsZero() {
		return NewValidationError(fieldName, "cannot be zero")
	}
	return nil
}

// ValidationError represents a validation error with context.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return e.Field + " " + e.Message
}

// NewValidationError creates a new validation error.
func NewValidationError(field, message string) error {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	validationError := &ValidationError{}
	ok := errors.As(err, &validationError)
	return ok
}
