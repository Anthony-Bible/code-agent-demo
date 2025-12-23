// Package dto provides Data Transfer Objects for the application layer.
package dto

// Validation errors for DTOs

var (
	// ErrEmptySessionID is returned when a request has an empty session ID.
	ErrEmptySessionID = NewValidationError("session ID cannot be empty")

	// ErrEmptyMessage is returned when a request has an empty message.
	ErrEmptyMessage = NewValidationError("message cannot be empty")

	// ErrInvalidSession is returned when a session is not found or is invalid.
	ErrInvalidSession = NewValidationError("invalid session")

	// ErrEmptyToolName is returned when a tool name is empty.
	ErrEmptyToolName = NewValidationError("tool name cannot be empty")

	// ErrEmptyToolDescription is returned when a tool description is empty.
	ErrEmptyToolDescription = NewValidationError("tool description cannot be empty")

	// ErrEmptyToolID is returned when a tool ID is empty.
	ErrEmptyToolID = NewValidationError("tool ID cannot be empty")

	// ErrNilToolInput is returned when tool input is nil.
	ErrNilToolInput = NewValidationError("tool input cannot be nil")

	// ErrEmptyToolList is returned when a tool list is empty.
	ErrEmptyToolList = NewValidationError("tool list cannot be empty")
)

// ValidationError represents a validation error in the application layer.
type ValidationError struct {
	Message string // The validation error message
}

// NewValidationError creates a new ValidationError.
func NewValidationError(msg string) error {
	return &ValidationError{Message: msg}
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return "validation error: " + e.Message
}
