package monitor

import (
	"context"
	"errors"
	"time"
)

// run owns one monitor from watch to admitted wake. Every exit path is ctx
// cancellation (cancel, session stop, shutdown) or a recorded terminal status.
func (s *Service) run(ctx context.Context, item *entry, created Monitor, spec Spec, watch TerminalWatch) {
	defer func() {
		if ctx.Err() != nil {
			s.logInterrupted(item)
		}
	}()
	outcome, ok := s.observe(ctx, spec, watch)
	if !ok {
		return
	}
	state, record, ok := s.markFired(item, outcome)
	if !ok {
		return
	}
	s.notify(created.SessionID)
	// The fired outcome is durable before the wake is admitted, so a restart re-delivers this exact
	// outcome under the stable message id instead of regenerating a different one.
	if !s.persistFired(ctx, item, record) {
		return
	}
	s.deliver(ctx, item, Notification{Monitor: state, Outcome: outcome})
}

// logInterrupted records a watch that stopped without settling because the daemon is stopping or
// its owner was interrupted by a restart; a monitor settled by Cancel or by an admitted wake is not.
func (s *Service) logInterrupted(item *entry) {
	s.mu.Lock()
	occupying, state := item.occupies(), item.state
	if occupying {
		s.logLifecycle("monitor.interrupted", state, "daemon_shutdown")
	}
	s.mu.Unlock()
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
func (s *Service) deliver(ctx context.Context, item *entry, notification Notification) {
	mon := notification.Monitor
	deadline := s.now().Add(deliveryRetryWindow)
	backoff := deliveryRetryInitial
	for {
		err := s.waker.WakeSession(ctx, notification)
		if err == nil {
			s.settle(item, StatusFired, "")
			return
		}
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, ErrSessionGone) {
			s.settle(item, StatusUndelivered, causeSessionGone)
			return
		}
		if !s.now().Add(backoff).Before(deadline) {
			s.settle(item, StatusUndelivered, causeRetryExhausted)
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
func (s *Service) markFired(item *entry, outcome Outcome) (Monitor, Record, bool) {
	s.mu.Lock()
	if item.state.Status != StatusActive {
		s.mu.Unlock()
		return Monitor{}, Record{}, false
	}
	item.state.Status = StatusDelivering
	item.state.FiredAt = s.now()
	item.state.Reason = outcome.Reason
	item.state.ExitCode = outcome.ExitCode
	s.logLifecycle("monitor.fired", item.state, outcome.Reason)
	state, record := item.state, s.decideLocked(item)
	s.mu.Unlock()
	return state, record, true
}

// persistFired writes the fired outcome with the delivery backoff and reports whether the wake may
// proceed. A store that stays unavailable for the whole retry window leaves the monitor undelivered
// (and queues that decision for the background retry) rather than waking from an outcome a restart
// could not reproduce.
func (s *Service) persistFired(ctx context.Context, item *entry, record Record) bool {
	if s.store == nil {
		return true
	}
	deadline := s.now().Add(deliveryRetryWindow)
	backoff := deliveryRetryInitial
	for {
		err := s.persistDecision(item, record, 1)
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		if !s.now().Add(backoff).Before(deadline) {
			s.settle(item, StatusUndelivered, causePersistUnavailable)
			return false
		}
		s.logLifecycle("monitor.wake_deferred", record.Monitor, "admission_retry")
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
		backoff = min(backoff*2, deliveryRetryMax)
	}
}

// settle records the end of a delivering monitor on the entry itself, so a wake admitted after its
// owner was suspended is still recorded. The durable row follows through the decision queue.
func (s *Service) settle(item *entry, status Status, cause string) {
	s.mu.Lock()
	if item.state.Status != StatusDelivering {
		s.mu.Unlock()
		return
	}
	s.finishLocked(item, status, s.now())
	if status == StatusFired {
		item.admitted = true
	}
	event := "monitor.wake_queued"
	if status == StatusUndelivered {
		event = "monitor.wake_failed"
	}
	s.logLifecycleCause(event, item.state, item.state.Reason, cause)
	record := s.decideLocked(item)
	sessionID := item.state.SessionID
	s.pruneSessionLocked(sessionID, s.now())
	s.mu.Unlock()
	_ = s.persistDecision(item, record, 1)
	s.notify(sessionID)
}
