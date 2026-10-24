package session

import (
	"errors"
	"fmt"
	"strings"
)

// StopVerificationFailedCode is the stable diagnostic for an unverified process exit.
const StopVerificationFailedCode = "stop_verification_failed"

// ErrStopVerificationFailed means the session cannot yet claim terminal state.
var ErrStopVerificationFailed = errors.New("session: stop_verification_failed")

type stopVerificationError struct {
	error
}

func (e stopVerificationError) Error() string {
	seen := make(map[string]bool)
	var lines []string
	var collect func(error)
	collect = func(err error) {
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			for _, cause := range joined.Unwrap() {
				collect(cause)
			}
			return
		}
		line := err.Error()
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	collect(e.error)
	return strings.Join(lines, "\n")
}

func (e stopVerificationError) Unwrap() error { return e.error }

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
