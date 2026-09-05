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

// String returns the stable cause token used by public stop diagnostics.
func (cause StopCause) String() string {
	switch cause {
	case CauseCompleted:
		return "completed"
	case CauseFailed:
		return "failed"
	case CauseUserRequested:
		return "user_requested"
	case CauseShutdown:
		return "shutdown"
	case CauseHookDenied:
		return "hook_denied"
	case CauseProcessExited:
		return "process_exited"
	case CauseTimeout:
		return "timeout"
	case CauseClearConversation:
		return "clear_conversation"
	case CauseConversationRewind:
		return "conversation_rewind"
	case CauseSpawnTTLExpired:
		return "spawn_ttl_expired"
	default:
		return ""
	}
}
