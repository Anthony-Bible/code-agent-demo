package entity

import "fmt"

// RCAFinding represents the result of a Root Cause Analysis investigation.
type RCAFinding struct {
	Summary  string   `json:"summary"`
	Causes   []Cause  `json:"causes"`
	Remedies []Remedy `json:"remedies"`
}

// Cause represents a potential reason for an alert.
type Cause struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	ConfidenceScore float64  `json:"confidence_score"`
	Evidence        []string `json:"evidence"`
}

// Impact levels for suggested remedies.
const (
	ImpactHigh   = "High"
	ImpactMedium = "Medium"
	ImpactLow    = "Low"
)

// Remedy represents a suggested action to resolve a cause.
type Remedy struct {
	Description     string   `json:"description"`
	ActionableSteps []string `json:"actionable_steps"`
	Impact          string   `json:"impact"`
}

// AddCause adds a cause to the RCAFinding.
func (r *RCAFinding) AddCause(cause Cause) {
	r.Causes = append(r.Causes, cause)
}

// AddRemedy adds a remedy to the RCAFinding.
func (r *RCAFinding) AddRemedy(remedy Remedy) {
	r.Remedies = append(r.Remedies, remedy)
}

// Validate ensures the RCAFinding is valid.
func (r *RCAFinding) Validate() error {
	if r.Summary == "" {
		return NewValidationError("summary", "summary is required")
	}

	if len(r.Causes) == 0 {
		return NewValidationError("causes", "at least one cause is required")
	}

	for i, cause := range r.Causes {
		if err := cause.Validate(); err != nil {
			return fmt.Errorf("cause at index %d is invalid: %w", i, err)
		}
	}

	for i, remedy := range r.Remedies {
		if err := remedy.Validate(); err != nil {
			return fmt.Errorf("remedy at index %d is invalid: %w", i, err)
		}
	}

	return nil
}

// Validate ensures the Cause is valid.
func (c *Cause) Validate() error {
	v := NewValidationHelper()

	if err := v.ValidateNotEmpty(c.ID, "id"); err != nil {
		return err
	}

	if err := v.ValidateNotEmpty(c.Description, "description"); err != nil {
		return err
	}

	if c.ConfidenceScore < 0 || c.ConfidenceScore > 1 {
		return NewValidationError("confidence_score", "must be between 0 and 1")
	}

	return nil
}

// Validate ensures the Remedy is valid.
func (r *Remedy) Validate() error {
	v := NewValidationHelper()

	if err := v.ValidateNotEmpty(r.Description, "description"); err != nil {
		return err
	}

	if err := v.ValidateNotEmpty(r.Impact, "impact"); err != nil {
		return err
	}

	switch r.Impact {
	case ImpactHigh, ImpactMedium, ImpactLow:
		// Valid
	default:
		return NewValidationError("impact", fmt.Sprintf("invalid impact level: %s", r.Impact))
	}

	if len(r.ActionableSteps) == 0 {
		return NewValidationError("actionable_steps", "at least one step is required")
	}

	return nil
}
