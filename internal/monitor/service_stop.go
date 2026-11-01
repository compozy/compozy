package monitor

import (
	"fmt"
)

// Cancel stops one active or delivering monitor owned by sessionID without waking the session. The
// monitor is canceled in memory at once and can no longer wake; the call reports an error until the
// cancellation is durable, and the service keeps retrying the write in the background, so a failed
// write never depends on a later caller.
func (s *Service) Cancel(sessionID, id string) (Monitor, error) {
	s.mu.Lock()
	item, ok := s.sessions[sessionID][id]
	if !ok {
		s.mu.Unlock()
		return Monitor{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !item.occupies() {
		state := item.state
		if item.saved >= item.decided {
			s.mu.Unlock()
			return state, nil
		}
		record := s.decideLocked(item)
		s.mu.Unlock()
		return state, s.confirmCancel(item, record)
	}
	s.finishLocked(item, StatusCanceled, s.now())
	s.logLifecycle("monitor.canceled", item.state, "requested")
	item.cancel()
	s.pruneSessionLocked(sessionID, s.now())
	state, record := item.state, s.decideLocked(item)
	s.mu.Unlock()
	s.notify(sessionID)
	return state, s.confirmCancel(item, record)
}

func (s *Service) confirmCancel(item *entry, record Record) error {
	if err := s.persistDecision(item, record, 1); err != nil && !s.confirmed(item) {
		return fmt.Errorf("monitor: persist cancellation: %w", err)
	}
	return nil
}

// CancelSession stops every monitor of a stopped or removed session and drops
// its history; nothing it registered may wake a later incarnation.
func (s *Service) CancelSession(sessionID string) int {
	canceled := s.cancelSession(sessionID)
	if canceled > 0 {
		s.notify(sessionID)
	}
	return canceled
}

func (s *Service) cancelSession(sessionID string) int {
	s.mu.Lock()
	canceled := 0
	now := s.now()
	type decision struct {
		item   *entry
		record Record
	}
	var decisions []decision
	for _, item := range s.sessions[sessionID] {
		if item.occupies() {
			s.finishLocked(item, StatusCanceled, now)
			item.state.Reason = ReasonSessionStopped
			s.logLifecycle("monitor.canceled", item.state, ReasonSessionStopped)
			item.cancel()
			decisions = append(decisions, decision{item, s.decideLocked(item)})
			canceled++
		}
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	for _, d := range decisions {
		// The bucket is gone, so the pending queue (retried in the background and flushed at shutdown)
		// is what keeps a failed write from reviving the monitor after a restart.
		s.writeDecision(d.item, d.record, saveRetries)
	}
	return canceled
}

// SuspendSession stops the in-memory watches of an owner the daemon is
// interrupting for a restart, keeping every monitor durable so Recover can
// restore it. Each monitor keeps the owner binding captured while the owner was live: a stopped
// session no longer carries a complete route, so a snapshot taken at stop time would be degraded.
func (s *Service) SuspendSession(sessionID string) int {
	s.mu.Lock()
	suspended := 0
	for _, item := range s.sessions[sessionID] {
		if !item.occupies() {
			continue
		}
		item.cancel()
		s.releaseLocked(item)
		suspended++
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	if suspended > 0 {
		s.notify(sessionID)
	}
	return suspended
}
