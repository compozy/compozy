package session

import (
	"errors"
	"fmt"
	"strings"

	"time"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/store"
)

func (s *Session) setFailure(failure *store.SessionFailure) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.failure = store.CloneSessionFailure(failure)
}

func (s *Session) setSandbox(sandbox *store.SessionSandboxMeta, now time.Time) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sandbox = cloneSessionSandboxMeta(sandbox)
	if !now.IsZero() {
		s.UpdatedAt = now
	}
}

func (s *Session) sandboxShouldDestroy() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sandboxDestroyOnStop
}

func (s *Session) activate(now time.Time, preserveStopReason bool) error {
	if err := s.transition(StateActive, now); err != nil {
		return err
	}
	if !preserveStopReason {
		s.clearStopClassification()
	}
	return nil
}

func (s *Session) beginStopping(now time.Time) error {
	return s.transition(StateStopping, now)
}

func (s *Session) markStopped(now time.Time) error {
	return s.transition(StateStopped, now)
}

func (s *Session) markStartFailed(now time.Time) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = StateStopped
	if !now.IsZero() {
		s.UpdatedAt = now
	}
}

func (s *Session) clearStopClassification() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopCause = CauseNone
	if s.stopReason == store.StopAgentCrashed {
		return
	}
	s.stopReason = ""
	s.stopDetail = ""
	s.failure = nil
}

func (s *Session) transition(next State, now time.Time) error {
	if s == nil {
		return errors.New("session: session is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State == next {
		if !now.IsZero() {
			s.UpdatedAt = now
		}
		return nil
	}

	if !canTransition(s.State, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStateTransition, s.State, next)
	}

	s.State = next
	if !now.IsZero() {
		s.UpdatedAt = now
	}
	return nil
}

func (s *Session) applyStopCauseLocked(cause StopCause, detail string) {
	if cause == CauseNone {
		return
	}

	if s.stopCause == CauseNone {
		s.stopCause = cause
		s.stopDetail = strings.TrimSpace(detail)
		return
	}

	if s.stopCause == cause && strings.TrimSpace(detail) != "" {
		s.stopDetail = strings.TrimSpace(detail)
	}
}

func normalizeSessionType(sessionType Type) Type {
	switch Type(strings.TrimSpace(string(sessionType))) {
	case SessionTypeDream:
		return SessionTypeDream
	case SessionTypeSystem:
		return SessionTypeSystem
	case SessionTypeCoordinator:
		return SessionTypeCoordinator
	case SessionTypeSpawned:
		return SessionTypeSpawned
	default:
		return SessionTypeUser
	}
}

func canTransition(current State, next State) bool {
	switch current {
	case StateStarting:
		return next == StateActive || next == StateStopping
	case StateActive:
		return next == StateStopping
	case StateStopping:
		return next == StateStopped
	default:
		return false
	}
}

func cloneCaps(caps acp.Caps) acp.Caps {
	return acp.CloneCaps(caps)
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func stopReasonPointer(value store.StopReason) *store.StopReason {
	if strings.TrimSpace(string(value)) == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func sessionMetaStopReason(meta store.SessionMeta) store.StopReason {
	if meta.StopReason == nil {
		return ""
	}
	return *meta.StopReason
}

func closedSignalChan() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
