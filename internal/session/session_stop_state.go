package session

import (
	"errors"
	"fmt"
	"strings"

	"time"

	"github.com/compozy/agh/internal/store"
)

func (s *Session) beginPromptSetup() error {
	if s == nil {
		return errors.New("session: session is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != StateActive {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if s.process == nil {
		return errors.New("session: agent process is not available")
	}
	if s.promptSetupDone == nil {
		s.promptSetupDone = closedSignalChan()
	}
	if s.promptSetupCount == 0 {
		s.promptSetupDone = make(chan struct{})
	}
	s.promptSetupCount++
	return nil
}

func (s *Session) beginExclusivePromptSetup() (*AgentProcess, error) {
	if s == nil {
		return nil, errors.New("session: session is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != StateActive {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if s.process == nil {
		return nil, errors.New("session: agent process is not available")
	}
	if s.promptSetupCount > 0 || s.currentTurnSource != "" {
		return nil, ErrPromptInProgress
	}
	if s.promptSetupDone == nil {
		s.promptSetupDone = closedSignalChan()
	}
	if s.promptSetupCount == 0 {
		s.promptSetupDone = make(chan struct{})
	}
	s.promptSetupCount++
	return s.process, nil
}

func (s *Session) finishPromptSetup() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.promptSetupCount == 0 {
		return
	}
	s.promptSetupCount--
	if s.promptSetupCount == 0 {
		close(s.promptSetupDone)
	}
}

func (s *Session) prepareStop(now time.Time, cause StopCause, detail string) (bool, <-chan struct{}, error) {
	if s == nil {
		return false, nil, errors.New("session: session is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.promptSetupDone == nil {
		s.promptSetupDone = closedSignalChan()
	}

	switch s.State {
	case StateStopped:
		s.applyStopCauseLocked(cause, detail)
		return false, s.promptSetupDone, nil
	case StateStopping:
		s.applyStopCauseLocked(cause, detail)
		return false, s.promptSetupDone, nil
	case StateStarting, StateActive:
		if !canTransition(s.State, StateStopping) {
			return false, nil, fmt.Errorf("%w: %s -> %s", ErrInvalidStateTransition, s.State, StateStopping)
		}
		s.applyStopCauseLocked(cause, detail)
		s.State = StateStopping
		if !now.IsZero() {
			s.UpdatedAt = now
		}
		return true, s.promptSetupDone, nil
	default:
		return false, nil, fmt.Errorf("%w: %s -> %s", ErrInvalidStateTransition, s.State, StateStopping)
	}
}

func (s *Session) setStopCause(cause StopCause) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyStopCauseLocked(cause, "")
}

func (s *Session) stopCauseDetail() (StopCause, string) {
	if s == nil {
		return CauseNone, ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopCause, s.stopDetail
}

func (s *Session) stopWasRequested() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	switch s.stopCause {
	case CauseFailed, CauseUserRequested, CauseShutdown, CauseHookDenied, CauseTimeout, CauseClearConversation:
		return true
	default:
		return false
	}
}

func (s *Session) setStopClassification(reason store.StopReason, detail string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopReason = reason
	s.stopDetail = strings.TrimSpace(detail)
}
