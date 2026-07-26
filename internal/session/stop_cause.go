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
)
