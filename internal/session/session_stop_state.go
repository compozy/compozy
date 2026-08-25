package session

import (
	"errors"
	"fmt"
	"strings"

	"time"

	"github.com/compozy/compozy/internal/store"
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
	if s.conversationRewindReserved {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if s.worktreeForkReserved {
		return ErrPromptInProgress
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
	return s.beginExclusivePromptSetupLocked()
}

func (s *Session) beginExclusivePromptSetupLocked() (*AgentProcess, error) {
	if s.State != StateActive {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if s.conversationRewindReserved {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if s.worktreeForkReserved {
		return nil, ErrPromptInProgress
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

func (s *Session) reserveConversationRewind() error {
	if s == nil {
		return errors.New("session: session is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if s.conversationRewindReserved || s.worktreeForkReserved || s.promptSetupCount > 0 ||
		s.currentTurnSource != "" || s.currentTurnID != "" {
		return fmt.Errorf("%w: %s", ErrConversationRewindBusy, s.ID)
	}
	if s.process != nil && s.process.HasPendingPermission() {
		return fmt.Errorf("%w: %s", ErrConversationRewindBusy, s.ID)
	}
	s.conversationRewindReserved = true
	return nil
}

func (s *Session) releaseConversationRewind() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.conversationRewindReserved = false
	s.mu.Unlock()
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
	cause = s.resolveSpawnTTLStopCauseLocked(cause)

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

// resolveSpawnTTLStopCauseLocked classifies an expired child at the same
// lifecycle lock that transitions it to stopping. This prevents a new prompt
// from being admitted between a reaper snapshot and the stop decision.
func (s *Session) resolveSpawnTTLStopCauseLocked(cause StopCause) StopCause {
	if cause != CauseSpawnTTLExpired {
		return cause
	}
	if s.promptSetupCount > 0 || s.currentTurnSource != "" || s.currentTurnID != "" {
		return CauseTimeout
	}
	return CauseCompleted
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
	case CauseFailed, CauseUserRequested, CauseShutdown, CauseHookDenied, CauseTimeout, CauseClearConversation,
		CauseConversationRewind:
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
