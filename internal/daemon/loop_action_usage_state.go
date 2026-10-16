package daemon

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type loopActionUsageState struct {
	tokensUsed       atomic.Int64
	mu               sync.RWMutex
	bindingMu        sync.Mutex
	sessionID        string
	pendingSessionID string
	// livenessSessionID names a session whose activity proves progress without owning the run's session link.
	livenessSessionID string
	progressAt        time.Time
	now               func() time.Time
	onSessionBound    func(sessionID string) bool
}

type loopActionProgressSnapshot struct {
	tokensUsed        int64
	sessionID         string
	livenessSessionID string
	progressAt        time.Time
}

// activitySessionID prefers the bound run session and falls back to a liveness-only session.
func (s loopActionProgressSnapshot) activitySessionID() string {
	if s.sessionID != "" {
		return s.sessionID
	}
	return s.livenessSessionID
}

func newLoopActionUsageState(now func() time.Time, onSessionBound func(sessionID string) bool) *loopActionUsageState {
	return &loopActionUsageState{now: now, progressAt: now().UTC(), onSessionBound: onSessionBound}
}

func (s *loopActionUsageState) ReportActionTokensUsed(tokensUsed int64) {
	if s == nil || tokensUsed <= 0 {
		return
	}
	for {
		current := s.tokensUsed.Load()
		if tokensUsed <= current {
			return
		}
		if s.tokensUsed.CompareAndSwap(current, tokensUsed) {
			s.recordProgress()
			return
		}
	}
}

func (s *loopActionUsageState) ReportActionSessionBound(sessionID string) {
	if s == nil {
		return
	}
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return
	}
	s.mu.Lock()
	changed := trimmed != s.sessionID
	if changed {
		s.sessionID = trimmed
		s.pendingSessionID = trimmed
	}
	s.progressAt = s.now().UTC()
	s.mu.Unlock()
	if changed && s.onSessionBound != nil {
		s.retryPendingSessionBinding()
	}
}

// ReportActionLivenessSession records the session that proves progress for executors owning their own binding.
func (s *loopActionUsageState) ReportActionLivenessSession(sessionID string) {
	if s == nil {
		return
	}
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return
	}
	s.mu.Lock()
	s.livenessSessionID = trimmed
	s.progressAt = s.now().UTC()
	s.mu.Unlock()
}

func (s *loopActionUsageState) retryPendingSessionBinding() {
	if s == nil {
		return
	}
	s.bindingMu.Lock()
	defer s.bindingMu.Unlock()
	for {
		s.mu.RLock()
		pending := s.pendingSessionID
		bind := s.onSessionBound
		s.mu.RUnlock()
		if pending == "" {
			return
		}
		if bind != nil && !bind(pending) {
			return
		}
		s.mu.Lock()
		if s.pendingSessionID == pending {
			s.pendingSessionID = ""
		}
		hasPending := s.pendingSessionID != ""
		s.mu.Unlock()
		if !hasPending {
			return
		}
	}
}

func (s *loopActionUsageState) recordProgress() {
	s.mu.Lock()
	s.progressAt = s.now().UTC()
	s.mu.Unlock()
}

func (s *loopActionUsageState) snapshot() loopActionProgressSnapshot {
	if s == nil {
		return loopActionProgressSnapshot{}
	}
	s.mu.RLock()
	snapshot := loopActionProgressSnapshot{
		tokensUsed:        s.tokensUsed.Load(),
		sessionID:         s.sessionID,
		livenessSessionID: s.livenessSessionID,
		progressAt:        s.progressAt,
	}
	s.mu.RUnlock()
	return snapshot
}

func (s *loopActionUsageState) TokensUsed() int64 {
	if s == nil {
		return 0
	}
	return s.tokensUsed.Load()
}
