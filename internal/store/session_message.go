package store

import (
	"context"
	"time"
)

// SessionMessageWindow summarizes one sender's admitted messages to a target inside a rate window.
type SessionMessageWindow struct {
	Count    int
	OldestAt time.Time
}

// SessionMessageLedger reads durable session-message admissions for the agent messaging guards.
type SessionMessageLedger interface {
	// SessionMessageWindow counts the sender's admissions into the target session created at or after since.
	SessionMessageWindow(
		ctx context.Context,
		targetWorkspaceID, targetSessionID, senderSessionID string,
		since time.Time,
	) (SessionMessageWindow, error)
	// LookupSessionPromptAdmissionByMessageID reads one admission by its message identity.
	LookupSessionPromptAdmissionByMessageID(
		ctx context.Context,
		workspaceID, sessionID, messageID string,
	) (SessionPromptAdmission, bool, error)
}
