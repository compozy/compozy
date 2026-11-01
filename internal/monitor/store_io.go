package monitor

import (
	"context"
	"log/slog"
	"time"
)

// storeTimeout bounds one persistence call so a slow database never stalls a monitor path.
const storeTimeout = 5 * time.Second

// recordLocked snapshots the entry for persistence and advances its write version.
func (e *entry) recordLocked() Record {
	e.version++
	return Record{
		Monitor: e.state, WorkspaceID: e.workspaceID, ProfileID: e.profileID,
		Timeout: e.timeout, OwnerBinding: e.binding, OwnerCheckFailedAt: e.ownerCheckFailedAt, Version: e.version,
	}
}

// saveNow persists a record and reports failure to the caller.
func (s *Service) saveNow(record Record) error {
	return s.saveWithin(context.WithoutCancel(s.ctx), record)
}

// saveWithin persists a record under ctx, bounded by storeTimeout.
func (s *Service) saveWithin(ctx context.Context, record Record) error {
	if s.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()
	return s.store.SaveMonitor(ctx, record)
}

// pruneStore drops durable finished monitors past retention, at most once per sweepInterval.
func (s *Service) pruneStore() {
	if s.store == nil {
		return
	}
	now := s.now()
	s.mu.Lock()
	due := now.Sub(s.lastPrune) >= sweepInterval
	if due {
		s.lastPrune = now
	}
	s.mu.Unlock()
	if !due {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), storeTimeout)
	defer cancel()
	if err := s.store.PruneFinishedMonitors(ctx, now.Add(-finishedRetention)); err != nil {
		s.logger.Log(s.ctx, slog.LevelWarn, "monitor.prune_failed", "event", "monitor.prune_failed")
	}
}

// saveRetries and saveRetryBackoff bound the immediate retries of a write that must not be lost.
const (
	saveRetries      = 3
	saveRetryBackoff = 100 * time.Millisecond
	// durableRetryInterval spaces the background retries of unconfirmed decisions.
	durableRetryInterval = 5 * time.Second
)

// decideLocked snapshots the entry as a terminal decision that must reach the store.
func (s *Service) decideLocked(item *entry) Record {
	record := item.recordLocked()
	item.decided = record.Version
	return record
}

// persistDecision writes a terminal decision (cancel, fired outcome, undelivered). Confirmation is
// monotonic: the decision is durable once any write at or above its version landed, whatever order
// overlapping writes finish in. A decision that is still unconfirmed is queued for background retries
// and a final flush at shutdown, so it never depends on a later caller to land.
func (s *Service) persistDecision(item *entry, record Record, attempts int) error {
	return s.persistDecisionWithin(context.WithoutCancel(s.ctx), item, record, attempts)
}

// persistDecisionWithin is persistDecision under ctx, so a shutdown flush honors its deadline.
func (s *Service) persistDecisionWithin(ctx context.Context, item *entry, record Record, attempts int) error {
	var err error
	for attempt := range max(attempts, 1) {
		if err = s.saveWithin(ctx, record); err == nil {
			break
		}
		if attempt+1 < attempts {
			time.Sleep(saveRetryBackoff << attempt)
		}
	}
	s.mu.Lock()
	if err == nil && record.Version > item.saved {
		item.saved = record.Version
	}
	unconfirmed := item.saved < item.decided
	if unconfirmed {
		s.pending[item.state.ID] = item
	} else {
		delete(s.pending, item.state.ID)
	}
	s.mu.Unlock()
	if unconfirmed {
		s.kickRetry()
		if err != nil {
			s.logger.Log(s.ctx, slog.LevelError, "monitor.persist_failed",
				"event", "monitor.persist_failed", "monitor_id", record.Monitor.ID,
				"session_id", record.Monitor.SessionID, "status", record.Monitor.Status)
		}
	}
	return err
}

// confirmed reports whether the entry's latest terminal decision is durable.
func (s *Service) confirmed(item *entry) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return item.saved >= item.decided
}

func (s *Service) kickRetry() {
	select {
	case s.retryKick <- struct{}{}:
	default:
	}
}

// retryDurable keeps retrying unconfirmed decisions until each lands or the service stops.
func (s *Service) retryDurable() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.retryKick:
		}
		for s.flushPending(context.WithoutCancel(s.ctx)) > 0 {
			timer := time.NewTimer(durableRetryInterval)
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

// flushPending rewrites every unconfirmed decision once, stopping at ctx, and returns how many remain.
func (s *Service) flushPending(ctx context.Context) int {
	s.mu.Lock()
	items := make([]*entry, 0, len(s.pending))
	for _, item := range s.pending {
		items = append(items, item)
	}
	s.mu.Unlock()
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		s.mu.Lock()
		record := s.decideLocked(item)
		s.mu.Unlock()
		if err := s.persistDecisionWithin(ctx, item, record, 1); err != nil {
			s.logger.Log(s.ctx, slog.LevelDebug, "monitor.decision_unconfirmed", "monitor_id", record.Monitor.ID)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// flushDurable is the shutdown flush: it retries unconfirmed decisions a few times within ctx.
func (s *Service) flushDurable(ctx context.Context) {
	for attempt := range saveRetries {
		if s.flushPending(ctx) == 0 || ctx.Err() != nil {
			return
		}
		timer := time.NewTimer(saveRetryBackoff << attempt)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// writeDecision persists a decision whose failure needs no caller reaction: persistDecision already
// logs it and keeps the decision queued for the background retry and the shutdown flush.
func (s *Service) writeDecision(item *entry, record Record, attempts int) {
	if err := s.persistDecision(item, record, attempts); err != nil {
		s.logger.Log(s.ctx, slog.LevelDebug, "monitor.decision_unconfirmed",
			"monitor_id", record.Monitor.ID, "queued", !s.confirmed(item))
	}
}
