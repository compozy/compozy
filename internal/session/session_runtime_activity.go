package session

import (
	"strings"

	"time"

	"github.com/compozy/agh/internal/store"
)

func (s *Session) observeRuntimeEventActivity(activity store.SessionActivityMeta, now time.Time) {
	_, _ = s.observeRuntimeActivityState(activity, now, false)
}

func (s *Session) observeRuntimeActivityState(
	activity store.SessionActivityMeta,
	now time.Time,
	clearStall bool,
) (string, string) {
	if s == nil || now.IsZero() {
		return "", ""
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Liveness == nil {
		s.Liveness = &store.SessionLivenessMeta{}
	}
	cloned := store.CloneSessionActivityMeta(&activity)
	s.Liveness.Activity = cloned
	lastUpdateAt := now.UTC()
	if cloned != nil && cloned.LastActivityAt != nil && !cloned.LastActivityAt.IsZero() {
		lastUpdateAt = cloned.LastActivityAt.UTC()
	}
	s.Liveness.LastUpdateAt = &lastUpdateAt
	previousStallState := ""
	previousStallReason := ""
	if clearStall {
		previousStallState = strings.TrimSpace(s.Liveness.StallState)
		previousStallReason = strings.TrimSpace(s.Liveness.StallReason)
		s.Liveness.StallState = ""
		s.Liveness.StallReason = ""
	}
	s.UpdatedAt = now.UTC()
	return previousStallState, previousStallReason
}

func (s *Session) clearRuntimeActivity(now time.Time) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Liveness != nil {
		s.Liveness.Activity = nil
		s.Liveness.StallState = ""
		s.Liveness.StallReason = ""
	}
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
}

func (s *Session) markRuntimeStalled(reason string, now time.Time) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Liveness == nil {
		s.Liveness = &store.SessionLivenessMeta{}
	}
	if strings.TrimSpace(reason) == "" {
		reason = store.SessionStallReasonActivityTimeout
	}
	s.Liveness.StallState = store.SessionStallStateDetected
	s.Liveness.StallReason = strings.TrimSpace(reason)
	if !now.IsZero() {
		lastUpdateAt := now.UTC()
		s.Liveness.LastUpdateAt = &lastUpdateAt
		s.UpdatedAt = now.UTC()
	}
}

func (s *Session) markExited(now time.Time) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Liveness != nil {
		s.Liveness.SubprocessPID = 0
		s.Liveness.SubprocessStartedAt = nil
		s.Liveness.StallState = ""
		s.Liveness.StallReason = ""
		s.Liveness.Activity = nil
	}
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
}

func (s *Session) setRecorder(recorder EventRecorder) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorder = recorder
}
