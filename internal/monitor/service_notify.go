package monitor

import (
	"context"
	"time"
)

const (
	// noticeBuffer bounds pending observer notices; one per occupying slot is enough
	// because a dropped notice only delays a projection the next change refreshes.
	noticeBuffer = MaxActiveTotal
	// observerTimeout bounds one observer call so a slow reader never stalls the queue.
	observerTimeout = 5 * time.Second
)

// notify queues an observer notice for sessionID without blocking the caller,
// which may hold session-manager locks (a session stop cancels its monitors).
func (s *Service) notify(sessionID string) {
	if s.notices == nil {
		return
	}
	select {
	case s.notices <- sessionID:
	default:
		s.logger.Debug("monitor: observer notice dropped", "session_id", sessionID)
	}
}

// dispatchNotices delivers queued notices one at a time on a service-owned
// goroutine, each bounded by observerTimeout, until the service shuts down.
func (s *Service) dispatchNotices() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case sessionID := <-s.notices:
			callCtx, cancel := context.WithTimeout(s.ctx, observerTimeout)
			s.observer(callCtx, sessionID)
			cancel()
		}
	}
}
