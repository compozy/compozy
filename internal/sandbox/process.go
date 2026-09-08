package sandbox

import "context"

// ProcessSignal identifies one action in the session owner's termination ladder.
type ProcessSignal string

const (
	ProcessSignalCloseInput ProcessSignal = "close-input"
	ProcessSignalTerminate  ProcessSignal = "terminate"
	ProcessSignalKill       ProcessSignal = "kill"
)

// ProcessController recovers an agent through its persisted provider identity.
// Missing identities and transport failures must never count as exit proof.
type ProcessController interface {
	ProcessExitVerified(context.Context, SessionState) (bool, error)
	SignalProcess(context.Context, SessionState, ProcessSignal) error
}
