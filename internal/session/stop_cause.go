package session

// StopCause records why a session stop was initiated.
type StopCause int

const (
	CauseNone StopCause = iota
	CauseCompleted
	CauseFailed
	CauseUserRequested
	CauseShutdown
	CauseHookDenied
	CauseProcessExited
	CauseTimeout
	CauseClearConversation
	CauseConversationRewind
	// CauseSpawnTTLExpired asks the session manager to classify the stop while
	// holding the session lifecycle lock, so prompt admission cannot race the
	// TTL decision.
	CauseSpawnTTLExpired
)
