package port

import (
	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
)

// RCAReporter defines the interface for displaying Root Cause Analysis findings.
// This is a feature-specific port that decouples RCA reporting from the generic
// UserInterface port, following the Interface Segregation Principle.
type RCAReporter interface {
	// DisplayRCAFindings displays structured Root Cause Analysis findings.
	DisplayRCAFindings(findings []entity.RCAFinding) error
}
