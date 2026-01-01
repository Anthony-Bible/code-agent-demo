package service

import "time"

// NewInvestigationStubForTest creates an InvestigationStub for testing purposes.
// This helper sets startedAt to the current time.
func NewInvestigationStubForTest(id, alertID, sessionID, status string) *InvestigationStub {
	return &InvestigationStub{
		id:        id,
		alertID:   alertID,
		sessionID: sessionID,
		status:    status,
		startedAt: time.Now(),
	}
}

// NewInvestigationStubForTestWithTime creates an InvestigationStub with a custom start time.
// Use this when testing time-based query filters.
func NewInvestigationStubForTestWithTime(
	id, alertID, sessionID, status string,
	startedAt time.Time,
) *InvestigationStub {
	return &InvestigationStub{
		id:        id,
		alertID:   alertID,
		sessionID: sessionID,
		status:    status,
		startedAt: startedAt,
	}
}
