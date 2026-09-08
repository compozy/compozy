package session

import (
	"errors"
	"fmt"
)

// StopVerificationFailedCode is the stable diagnostic for an unverified process exit.
const StopVerificationFailedCode = "stop_verification_failed"

// ErrStopVerificationFailed means the session cannot yet claim terminal state.
var ErrStopVerificationFailed = errors.New("session: stop_verification_failed")

// AgentExitVerifier lets the process owner verify local or remote exit identity.
type AgentExitVerifier interface {
	VerifyExit(proc *AgentProcess) (bool, error)
}

func (m *Manager) verifySessionProcessExit(session *Session) error {
	proc := session.processHandle()
	if proc == nil {
		return nil
	}
	verifyCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	verified, err := m.verifyStopPhase(verifyCtx, proc)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrStopVerificationFailed, session.ID, err)
	}
	if !verified {
		return fmt.Errorf("%w: process for %s is still alive", ErrStopVerificationFailed, session.ID)
	}
	return nil
}
