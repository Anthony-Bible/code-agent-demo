package entity_test

import (
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCause_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cause   entity.Cause
		wantErr string
	}{
		{
			name: "valid cause",
			cause: entity.Cause{
				ID:              "C1",
				Description:     "High CPU usage",
				ConfidenceScore: 0.8,
				Evidence:        []string{"Logs"},
			},
			wantErr: "",
		},
		{
			name: "missing id",
			cause: entity.Cause{
				Description:     "High CPU usage",
				ConfidenceScore: 0.8,
			},
			wantErr: "id cannot be empty",
		},
		{
			name: "missing description",
			cause: entity.Cause{
				ID:              "C1",
				ConfidenceScore: 0.8,
			},
			wantErr: "description cannot be empty",
		},
		{
			name: "invalid confidence score > 1",
			cause: entity.Cause{
				ID:              "C1",
				Description:     "Desc",
				ConfidenceScore: 1.1,
			},
			wantErr: "confidence_score must be between 0 and 1",
		},
		{
			name: "invalid confidence score < 0",
			cause: entity.Cause{
				ID:              "C1",
				Description:     "Desc",
				ConfidenceScore: -0.1,
			},
			wantErr: "confidence_score must be between 0 and 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cause.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRemedy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		remedy  entity.Remedy
		wantErr string
	}{
		{
			name: "valid remedy",
			remedy: entity.Remedy{
				Description:     "Restart service",
				ActionableSteps: []string{"systemctl restart svc"},
				Impact:          "High",
			},
			wantErr: "",
		},
		{
			name: "missing description",
			remedy: entity.Remedy{
				ActionableSteps: []string{"systemctl restart svc"},
				Impact:          "High",
			},
			wantErr: "description cannot be empty",
		},
		{
			name: "missing impact",
			remedy: entity.Remedy{
				Description:     "Restart service",
				ActionableSteps: []string{"systemctl restart svc"},
			},
			wantErr: "impact cannot be empty",
		},
		{
			name: "missing actionable steps",
			remedy: entity.Remedy{
				Description: "Restart service",
				Impact:      "High",
			},
			wantErr: "actionable_steps at least one step is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.remedy.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRCAFinding_Validate(t *testing.T) {
	validCause := entity.Cause{
		ID:              "C1",
		Description:     "Desc",
		ConfidenceScore: 0.8,
	}
	validRemedy := entity.Remedy{
		Description:     "Rem",
		Impact:          "High",
		ActionableSteps: []string{"Step"},
	}

	tests := []struct {
		name    string
		finding entity.RCAFinding
		wantErr string
	}{
		{
			name: "valid finding",
			finding: entity.RCAFinding{
				Summary:  "Test Summary",
				Causes:   []entity.Cause{validCause},
				Remedies: []entity.Remedy{validRemedy},
			},
			wantErr: "",
		},
		{
			name: "missing summary",
			finding: entity.RCAFinding{
				Causes:   []entity.Cause{validCause},
				Remedies: []entity.Remedy{validRemedy},
			},
			wantErr: "summary is required",
		},
		{
			name: "missing causes",
			finding: entity.RCAFinding{
				Summary:  "Test Summary",
				Remedies: []entity.Remedy{validRemedy},
			},
			wantErr: "causes at least one cause is required",
		},
		{
			name: "invalid cause in causes",
			finding: entity.RCAFinding{
				Summary:  "Test Summary",
				Causes:   []entity.Cause{{}}, // invalid
				Remedies: []entity.Remedy{validRemedy},
			},
			wantErr: "cause at index 0 is invalid",
		},
		{
			name: "invalid remedy in remedies",
			finding: entity.RCAFinding{
				Summary:  "Test Summary",
				Causes:   []entity.Cause{validCause},
				Remedies: []entity.Remedy{{}}, // invalid
			},
			wantErr: "remedy at index 0 is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.finding.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRCAFinding_AddMethods(t *testing.T) {
	finding := entity.RCAFinding{}
	cause := entity.Cause{ID: "C1", Description: "Desc", ConfidenceScore: 0.8}
	remedy := entity.Remedy{Description: "Rem", Impact: "High", ActionableSteps: []string{"Step"}}

	finding.AddCause(cause)
	finding.AddRemedy(remedy)

	require.Len(t, finding.Causes, 1)
	require.Len(t, finding.Remedies, 1)
	assert.Equal(t, cause, finding.Causes[0])
	assert.Equal(t, remedy, finding.Remedies[0])
}
