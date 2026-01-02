package service

import "time"

// NewInvestigationRecordForTest creates an InvestigationRecord for testing purposes.
// This helper sets startedAt to the current time.
func NewInvestigationRecordForTest(id, alertID, sessionID, status string) *InvestigationRecord {
	return &InvestigationRecord{
		id:        id,
		alertID:   alertID,
		sessionID: sessionID,
		status:    status,
		startedAt: time.Now(),
	}
}

// NewInvestigationRecordForTestWithTime creates an InvestigationRecord with a custom start time.
// Use this when testing time-based query filters.
func NewInvestigationRecordForTestWithTime(
	id, alertID, sessionID, status string,
	startedAt time.Time,
) *InvestigationRecord {
	return &InvestigationRecord{
		id:        id,
		alertID:   alertID,
		sessionID: sessionID,
		status:    status,
		startedAt: startedAt,
	}
}
