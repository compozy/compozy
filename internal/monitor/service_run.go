package monitor

import (
	"context"
	"errors"
	"time"
)

// run owns one monitor from watch to admitted wake. Every exit path is ctx
// cancellation (cancel, session stop, shutdown) or a recorded terminal status.
func (s *Service) run(ctx context.Context, created Monitor, spec Spec, watch TerminalWatch) {
	outcome, ok := s.observe(ctx, spec, watch)
	if !ok {
		return
	}
	state, ok := s.markFired(created.ID, created.SessionID, outcome)
	if !ok {
		return
	}
	s.notify(created.SessionID)
	s.deliver(ctx, Notification{Monitor: state, Outcome: outcome})
}

func (s *Service) observe(ctx context.Context, spec Spec, watch TerminalWatch) (Outcome, bool) {
	if spec.Kind == KindTimer {
		timer := time.NewTimer(spec.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return Outcome{}, false
		case <-timer.C:
			return Outcome{Reason: ReasonTimerElapsed}, true
		}
	}
	outcome, err := watch(ctx)
	if ctx.Err() != nil {
		return Outcome{}, false
	}
	if err != nil {
		return Outcome{Reason: ReasonTerminalFailed}, true
	}
	return outcome, true
}

// deliver admits the wake, retrying transient refusals with capped backoff
// inside deliveryRetryWindow. The waker's message id is stable per monitor, so
// a retry after an ambiguous failure cannot queue a second wake.
func (s *Service) deliver(ctx context.Context, notification Notification) {
	mon := notification.Monitor
	deadline := s.now().Add(deliveryRetryWindow)
	backoff := deliveryRetryInitial
	for {
		err := s.waker.WakeSession(ctx, notification)
		if err == nil {
			s.settle(mon.SessionID, mon.ID, StatusFired)
			return
		}
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, ErrSessionGone) || !s.now().Add(backoff).Before(deadline) {
			s.settle(mon.SessionID, mon.ID, StatusUndelivered)
			return
		}
		s.logLifecycle("monitor.wake_deferred", mon, "admission_retry")
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, deliveryRetryMax)
	}
}

// markFired records the outcome and moves the monitor to delivering; it still
// holds its cap slots until the wake is admitted or abandoned.
func (s *Service) markFired(id, sessionID string, outcome Outcome) (Monitor, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.sessions[sessionID][id]
	if !ok || item.state.Status != StatusActive {
		return Monitor{}, false
	}
	item.state.Status = StatusDelivering
	item.state.FiredAt = s.now()
	item.state.Reason = outcome.Reason
	item.state.ExitCode = outcome.ExitCode
	s.logLifecycle("monitor.fired", item.state, outcome.Reason)
	return item.state, true
}

func (s *Service) settle(sessionID, id string, status Status) {
	s.mu.Lock()
	item, ok := s.sessions[sessionID][id]
	if !ok || item.state.Status != StatusDelivering {
		s.mu.Unlock()
		return
	}
	s.finishLocked(item, status, s.now())
	event := "monitor.wake_queued"
	if status == StatusUndelivered {
		event = "monitor.wake_failed"
	}
	s.logLifecycle(event, item.state, item.state.Reason)
	s.pruneSessionLocked(sessionID, s.now())
	s.mu.Unlock()
	s.notify(sessionID)
}
