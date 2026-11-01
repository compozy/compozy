package monitor

import (
	"fmt"
)

// Cancel stops one active or delivering monitor owned by sessionID without waking the session.
func (s *Service) Cancel(sessionID, id string) (Monitor, error) {
	s.mu.Lock()
	item, ok := s.sessions[sessionID][id]
	if !ok {
		s.mu.Unlock()
		return Monitor{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !item.occupies() {
		state := item.state
		if !item.unsaved {
			s.mu.Unlock()
			return state, nil
		}
		record := item.recordLocked()
		s.mu.Unlock()
		return state, s.confirmCancel(item, record)
	}
	s.finishLocked(item, StatusCanceled, s.now())
	s.logLifecycle("monitor.canceled", item.state, "requested")
	item.cancel()
	s.pruneSessionLocked(sessionID, s.now())
	// Until the write is confirmed a concurrent Cancel must retry it, not report success.
	item.unsaved = true
	state, record := item.state, item.recordLocked()
	s.mu.Unlock()
	s.notify(sessionID)
	return state, s.confirmCancel(item, record)
}

// confirmCancel makes a cancellation durable. The monitor is already canceled in memory and can no
// longer wake, but until the write lands a restart would restore it as active, so a failure is
// reported to the caller and the next Cancel retries the write.
func (s *Service) confirmCancel(item *entry, record Record) error {
	err := s.saveNow(record)
	s.mu.Lock()
	item.unsaved = err != nil
	s.mu.Unlock()
	if err != nil {
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
	var records []Record
	for _, item := range s.sessions[sessionID] {
		if item.occupies() {
			s.finishLocked(item, StatusCanceled, now)
			item.state.Reason = ReasonSessionStopped
			s.logLifecycle("monitor.canceled", item.state, ReasonSessionStopped)
			item.cancel()
			records = append(records, item.recordLocked())
			canceled++
		}
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	for _, record := range records {
		s.saveRetrying(record)
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
	var records []Record
	for _, item := range s.sessions[sessionID] {
		if !item.occupies() {
			continue
		}
		item.cancel()
		s.releaseLocked(item)
		records = append(records, item.recordLocked())
		suspended++
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	for _, record := range records {
		s.save(record)
	}
	if suspended > 0 {
		s.notify(sessionID)
	}
	return suspended
}
