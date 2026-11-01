package monitor

import (
	"context"
	"errors"
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
		Timeout: e.timeout, OwnerBinding: e.binding, Version: e.version,
	}
}

// saveNow persists a record and reports failure to the caller.
func (s *Service) saveNow(record Record) error {
	if s.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), storeTimeout)
	defer cancel()
	return s.store.SaveMonitor(ctx, record)
}

// save persists a state change after the fact. A failure leaves the previous durable
// state, which Recover treats conservatively, so it is logged rather than propagated.
func (s *Service) save(record Record) {
	if err := s.saveNow(record); err != nil {
		s.logger.Log(s.ctx, slog.LevelWarn, "monitor.persist_failed",
			"event", "monitor.persist_failed", "monitor_id", record.Monitor.ID,
			"session_id", record.Monitor.SessionID, "status", record.Monitor.Status,
			"canceled", errors.Is(err, context.Canceled))
	}
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
	var err error
	for attempt := range max(attempts, 1) {
		if err = s.saveNow(record); err == nil {
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
		for s.flushPending() > 0 {
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

// flushPending rewrites every unconfirmed decision once and returns how many remain.
func (s *Service) flushPending() int {
	s.mu.Lock()
	items := make([]*entry, 0, len(s.pending))
	for _, item := range s.pending {
		items = append(items, item)
	}
	s.mu.Unlock()
	for _, item := range items {
		s.mu.Lock()
		record := s.decideLocked(item)
		s.mu.Unlock()
		_ = s.persistDecision(item, record, 1)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// flushDurable is the shutdown flush: it retries unconfirmed decisions a few times within ctx.
func (s *Service) flushDurable(ctx context.Context) {
	for attempt := range saveRetries {
		if s.flushPending() == 0 || ctx.Err() != nil {
			return
		}
		time.Sleep(saveRetryBackoff << attempt)
	}
}
