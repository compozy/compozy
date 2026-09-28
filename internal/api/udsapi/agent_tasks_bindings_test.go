package udsapi

import (
	"testing"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func agentTaskLeaseHandleForTest(
	t *testing.T,
	sessionID string,
	runID string,
	rawToken string,
	leaseUntil time.Time,
) taskpkg.AutonomyLeaseHandle {
	t.Helper()
	if sessionID != "sess-agent" || runID != "run-1" {
		t.Fatalf("LookupActiveRunForSession(session=%q, run=%q), want sess-agent/run-1", sessionID, runID)
	}
	hash, err := taskpkg.ClaimTokenHash(rawToken)
	if err != nil {
		t.Fatalf("ClaimTokenHash() error = %v", err)
	}
	return taskpkg.AutonomyLeaseHandle{
		RunID:          runID,
		TaskID:         "task-1",
		WorkspaceID:    "ws-1",
		SessionID:      sessionID,
		Status:         taskpkg.TaskRunStatusClaimed,
		ClaimToken:     rawToken,
		ClaimTokenHash: hash,
		LeaseUntil:     leaseUntil,
	}
}
