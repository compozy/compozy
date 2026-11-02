package monitor

import (
	"context"
	"errors"
	"reflect"
	"time"
)

// pullRequestRun is the watch loop's own schedule; only its goroutine touches it.
type pullRequestRun struct {
	target      PullRequestTarget
	deadline    time.Time
	started     time.Time
	nextRead    time.Time
	nextAttempt time.Time
	backoff     time.Duration
}

// pullRequestView is one consistent snapshot of a watch.
type pullRequestView struct {
	state   Monitor
	cursor  prCursor
	pending *prFire
	carry   PullRequestChanges
}

// runPullRequest owns one armed pull_request monitor until it ends. Observing never stops for delivery:
// a fire keeps retrying under its own message id while later changes collect in carry, so a fact cannot
// vanish between being noticed and being told, whatever the forge shows in the meantime.
func (s *Service) runPullRequest(ctx context.Context, item *entry) {
	defer func() {
		if ctx.Err() != nil {
			s.logInterrupted(item)
		}
	}()
	s.mu.Lock()
	run := &pullRequestRun{
		target: item.target(), deadline: item.state.DeadlineAt, started: s.now(), backoff: deliveryRetryInitial,
	}
	lost := item.state.Status == StatusDelivering && item.pullRequest.pending == nil
	s.mu.Unlock()
	if lost {
		s.rearmPullRequest(item, eventWakeFailed, causePersistUnavailable)
	}
	for s.stepPullRequest(ctx, item, run) {
	}
}

// stepPullRequest does the one thing now due and reports whether the watch goes on.
func (s *Service) stepPullRequest(ctx context.Context, item *entry, run *pullRequestRun) bool {
	view := s.viewPullRequest(item)
	if view == nil {
		return false
	}
	now := s.now()
	switch {
	case view.pending != nil && !now.Before(run.nextAttempt):
		return s.attemptPullRequestFire(ctx, item, run, view)
	case view.pending == nil && view.carry.Wake:
		return s.firePullRequest(ctx, item, view.carry, view.cursor, view.carry.reason(), view.carry.Terminal != "")
	case view.pending == nil && !now.Before(run.deadline):
		return s.firePullRequestTimeout(ctx, item, view)
	case view.reading(now, run) && !now.Before(run.nextRead):
		observation, ok := s.readPullRequest(ctx, run.target)
		if !ok {
			return false
		}
		run.nextRead = nextPullRequestTick(run.target, s.now())
		return s.applyPullRequestObservation(ctx, item, observation)
	}
	return s.sleepUntil(ctx, view.wakeAt(now, run))
}

// viewPullRequest returns nil once the monitor has ended.
func (s *Service) viewPullRequest(item *entry) *pullRequestView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !item.occupies() {
		return nil
	}
	watch := item.pullRequest
	return &pullRequestView{state: item.state, cursor: watch.cursor, pending: watch.pending, carry: watch.carry}
}

// reading reports whether another observation could still change what this monitor will say.
func (v *pullRequestView) reading(now time.Time, run *pullRequestRun) bool {
	if !now.Before(run.deadline) || v.carry.Terminal != "" {
		return false
	}
	return v.pending == nil || !v.pending.Final
}

func (v *pullRequestView) wakeAt(now time.Time, run *pullRequestRun) time.Time {
	wake := run.deadline
	if v.reading(now, run) && run.nextRead.Before(wake) {
		wake = run.nextRead
	}
	if v.pending != nil && (!v.reading(now, run) || run.nextAttempt.Before(wake)) {
		wake = run.nextAttempt
	}
	return wake
}

func (s *Service) sleepUntil(ctx context.Context, wake time.Time) bool {
	timer := time.NewTimer(max(wake.Sub(s.now()), 0))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Service) applyPullRequestObservation(ctx context.Context, item *entry, obs PullRequestObservation) bool {
	view := s.viewPullRequest(item)
	if view == nil {
		return false
	}
	changes, next := detectPullRequest(view.cursor, obs, s.now())
	if view.pending == nil && changes.Wake {
		return s.firePullRequest(ctx, item, changes, next, changes.reason(), changes.Terminal != "")
	}
	s.advancePullRequest(item, mergePullRequestChanges(view.carry, changes), next)
	return true
}

// advancePullRequest saves progress that needs no wake right now, such as a baseline or changes waiting
// behind a fire, so a restart does not forget it.
func (s *Service) advancePullRequest(item *entry, carry PullRequestChanges, next prCursor) {
	s.mu.Lock()
	watch := item.pullRequest
	if !item.occupies() || (reflect.DeepEqual(watch.cursor, next) && reflect.DeepEqual(watch.carry, carry)) {
		s.mu.Unlock()
		return
	}
	watch.cursor, watch.carry = next, carry
	record := s.decideLocked(item)
	s.mu.Unlock()
	s.writeDecision(item, record, 1)
}

func (s *Service) firePullRequestTimeout(ctx context.Context, item *entry, view *pullRequestView) bool {
	changes, next := flushBurst(PullRequestChanges{
		Wake: true, TimedOut: true, Number: view.cursor.Number, Head: view.cursor.Head,
	}, view.cursor)
	if view.cursor.Number == 0 {
		changes.EndedNumber = view.cursor.EndedNumber
	}
	return s.firePullRequest(ctx, item, changes, next, ReasonTimeout, true)
}

// firePullRequest records the fire durably, sequence included, before any admission attempt, so a restart
// replays this exact wake under the same message id. Facts the fire carries leave the cursor in that same
// write. It reports whether the watch goes on.
func (s *Service) firePullRequest(
	ctx context.Context, item *entry, changes PullRequestChanges, next prCursor, reason string, final bool,
) bool {
	s.mu.Lock()
	if item.state.Status != StatusActive {
		s.mu.Unlock()
		return false
	}
	seq := item.state.FireCount + 1
	if seq >= MaxPullRequestFires && !final {
		changes.FireCapHit, final, reason = true, true, ReasonFireCapReached
	}
	watch := item.pullRequest
	watch.pending, watch.cursor, watch.carry = &prFire{Changes: changes, Final: final}, next, PullRequestChanges{}
	item.state.Status = StatusDelivering
	item.state.FireCount = seq
	item.state.LastFiredAt = s.now()
	item.state.Reason = reason
	s.logLifecycle("monitor.fired", item.state, reason)
	state, record := item.state, s.decideLocked(item)
	s.mu.Unlock()
	s.notify(state.SessionID)
	return s.persistFired(ctx, item, record)
}

// attemptPullRequestFire tries to admit the pending fire once. The message id stays the same across
// attempts, so one that was admitted without saying so cannot queue a second wake.
func (s *Service) attemptPullRequestFire(
	ctx context.Context, item *entry, run *pullRequestRun, view *pullRequestView,
) bool {
	err := s.waker.WakeSession(ctx, Notification{
		Monitor: view.state, Outcome: Outcome{Reason: view.state.Reason}, PullRequest: &view.pending.Changes,
	})
	switch {
	case err == nil:
		run.nextAttempt, run.backoff = time.Time{}, deliveryRetryInitial
		if view.pending.Final {
			s.settle(item, StatusFired, "")
			return false
		}
		s.rearmPullRequest(item, "monitor.wake_queued", "")
		return true
	case ctx.Err() != nil:
		return false
	case errors.Is(err, ErrSessionGone):
		s.settle(item, StatusUndelivered, causeSessionGone)
		return false
	}
	now := s.now()
	if !now.Add(run.backoff).Before(run.horizon(view)) {
		s.settle(item, StatusUndelivered, causeRetryExhausted)
		return false
	}
	s.logLifecycle("monitor.wake_deferred", view.state, "admission_retry")
	run.nextAttempt = now.Add(run.backoff)
	run.backoff = min(run.backoff*2, deliveryRetryMax)
	return true
}

// horizon is when a fire stops retrying. A final wake gets the usual window. Any other fire keeps its
// place for as long as the monitor lives, plus that window, since the watch is still armed behind it.
func (r *pullRequestRun) horizon(view *pullRequestView) time.Time {
	base := r.deadline
	if view.pending.Final {
		base = view.state.LastFiredAt
	}
	if r.started.After(base) {
		base = r.started
	}
	return base.Add(deliveryRetryWindow)
}

// rearmPullRequest returns a delivering monitor to active with its slots once its fire is resolved.
func (s *Service) rearmPullRequest(item *entry, event, cause string) {
	s.mu.Lock()
	if item.state.Status != StatusDelivering {
		s.mu.Unlock()
		return
	}
	item.state.Status = StatusActive
	item.pullRequest.pending = nil
	s.logLifecycleCause(event, item.state, item.state.Reason, cause)
	record, sessionID := s.decideLocked(item), item.state.SessionID
	s.mu.Unlock()
	s.writeDecision(item, record, saveRetries)
	s.notify(sessionID)
}
