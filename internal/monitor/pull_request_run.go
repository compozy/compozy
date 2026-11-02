package monitor

import (
	"context"
	"reflect"
	"time"
)

// runPullRequest owns one armed pull_request monitor until it ends. A fire stops the loop only for its
// delivery, so whatever changes meanwhile is still in the next observation's diff against the cursor.
func (s *Service) runPullRequest(ctx context.Context, item *entry) {
	defer func() {
		if ctx.Err() != nil {
			s.logInterrupted(item)
		}
	}()
	if !s.replayPullRequestFire(ctx, item) {
		return
	}
	s.mu.Lock()
	target, deadline := item.target(), item.state.DeadlineAt
	s.mu.Unlock()
	for {
		if !s.now().Before(deadline) {
			s.firePullRequestTimeout(ctx, item)
			return
		}
		observation, ok := s.readPullRequest(ctx, target)
		if !ok || !s.applyPullRequestObservation(ctx, item, observation) {
			return
		}
		if !s.sleepUntil(ctx, nextPullRequestTick(target, s.now()), deadline) {
			return
		}
	}
}

func (s *Service) sleepUntil(ctx context.Context, tick, deadline time.Time) bool {
	wake := tick
	if deadline.Before(wake) {
		wake = deadline
	}
	timer := time.NewTimer(max(wake.Sub(s.now()), 0))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// replayPullRequestFire re-delivers the fire a restart interrupted, with the same id and the same facts.
func (s *Service) replayPullRequestFire(ctx context.Context, item *entry) bool {
	s.mu.Lock()
	state, pending := item.state, item.pullRequest.pending
	s.mu.Unlock()
	if state.Status != StatusDelivering {
		return state.Status == StatusActive
	}
	if pending == nil {
		s.rearmPullRequest(item, false, causePersistUnavailable)
		return true
	}
	return s.deliverPullRequestFire(ctx, item, state, pending)
}

func (s *Service) applyPullRequestObservation(ctx context.Context, item *entry, obs PullRequestObservation) bool {
	s.mu.Lock()
	active, cursor := item.state.Status == StatusActive, item.pullRequest.cursor
	s.mu.Unlock()
	if !active {
		return false
	}
	changes, next := detectPullRequest(cursor, obs, s.now())
	if !changes.Wake {
		s.advancePullRequestCursor(item, next)
		return true
	}
	return s.firePullRequest(ctx, item, changes, next, changes.reason(), changes.Terminal != "")
}

// advancePullRequestCursor saves quiet progress, such as a baseline, so a restart does not forget it.
func (s *Service) advancePullRequestCursor(item *entry, next prCursor) {
	s.mu.Lock()
	if item.state.Status != StatusActive || reflect.DeepEqual(item.pullRequest.cursor, next) {
		s.mu.Unlock()
		return
	}
	item.pullRequest.cursor = next
	record := s.decideLocked(item)
	s.mu.Unlock()
	s.writeDecision(item, record, 1)
}

func (s *Service) firePullRequestTimeout(ctx context.Context, item *entry) {
	s.mu.Lock()
	cursor := item.pullRequest.cursor
	s.mu.Unlock()
	changes, next := flushBurst(PullRequestChanges{
		Wake: true, TimedOut: true, Number: cursor.Number, Head: cursor.Head,
	}, cursor)
	s.firePullRequest(ctx, item, changes, next, ReasonTimeout, true)
}

// firePullRequest records the fire durably, sequence included, before admitting its wake, so a restart
// replays this exact wake under the same message id. It reports whether the watch goes on.
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
	fire := &prFire{Changes: changes, Next: next, Final: final}
	item.pullRequest.pending = fire
	item.state.Status = StatusDelivering
	item.state.FireCount = seq
	item.state.LastFiredAt = s.now()
	item.state.Reason = reason
	s.logLifecycle("monitor.fired", item.state, reason)
	state, record := item.state, s.decideLocked(item)
	s.mu.Unlock()
	s.notify(state.SessionID)
	if !s.persistFired(ctx, item, record) {
		return false
	}
	return s.deliverPullRequestFire(ctx, item, state, fire)
}

func (s *Service) deliverPullRequestFire(ctx context.Context, item *entry, state Monitor, fire *prFire) bool {
	switch s.admitWake(ctx, Notification{
		Monitor: state, Outcome: Outcome{Reason: state.Reason}, PullRequest: &fire.Changes,
	}) {
	case wakeAdmitted:
		if fire.Final {
			s.settle(item, StatusFired, "")
			return false
		}
		s.rearmPullRequest(item, true, "")
		return true
	case wakeSessionGone:
		s.settle(item, StatusUndelivered, causeSessionGone)
		return false
	case wakeRetryExhausted:
		if fire.Final {
			s.settle(item, StatusUndelivered, causeRetryExhausted)
			return false
		}
		s.rearmPullRequest(item, false, causeRetryExhausted)
		return true
	default:
		return false
	}
}

// rearmPullRequest returns a delivering monitor to active with its slots. Only an admitted wake advances
// the cursor; otherwise the same facts are reported again by the next fire, under a new message id.
func (s *Service) rearmPullRequest(item *entry, admitted bool, cause string) {
	s.mu.Lock()
	if item.state.Status != StatusDelivering {
		s.mu.Unlock()
		return
	}
	item.state.Status = StatusActive
	event := eventWakeFailed
	if admitted {
		item.pullRequest.cursor = item.pullRequest.pending.Next
		event = "monitor.wake_queued"
	}
	item.pullRequest.pending = nil
	s.logLifecycleCause(event, item.state, item.state.Reason, cause)
	record, sessionID := s.decideLocked(item), item.state.SessionID
	s.mu.Unlock()
	s.writeDecision(item, record, saveRetries)
	s.notify(sessionID)
}
