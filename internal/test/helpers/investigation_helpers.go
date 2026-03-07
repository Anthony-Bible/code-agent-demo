package helpers

import (
	"code-editing-agent/internal/application/service"
	"time"
)

// NewInvestigationRecordForTest creates an InvestigationRecord for testing purposes.
// This helper sets startedAt to the current time.
func NewInvestigationRecordForTest(id, alertID, sessionID, status string) *service.InvestigationRecord {
	return service.NewInvestigationRecord(id, alertID, sessionID, status, time.Now())
}

// NewInvestigationRecordForTestWithTime creates an InvestigationRecord with a custom start time.
// Use this when testing time-based query filters.
func NewInvestigationRecordForTestWithTime(
	id, alertID, sessionID, status string,
	startedAt time.Time,
) *service.InvestigationRecord {
	return service.NewInvestigationRecord(id, alertID, sessionID, status, startedAt)
}
