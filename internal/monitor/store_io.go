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

// saveRetries and saveRetryBackoff bound the retries of a write that must not be lost.
const (
	saveRetries      = 3
	saveRetryBackoff = 100 * time.Millisecond
)

// saveRetrying persists a decision that nothing can retry later (a stopped owner's cancellation).
func (s *Service) saveRetrying(record Record) {
	if s.store == nil {
		return
	}
	var err error
	for attempt := range saveRetries {
		if err = s.saveNow(record); err == nil {
			return
		}
		time.Sleep(saveRetryBackoff << attempt)
	}
	s.logger.Log(s.ctx, slog.LevelError, "monitor.persist_failed",
		"event", "monitor.persist_failed", "monitor_id", record.Monitor.ID,
		"session_id", record.Monitor.SessionID, "status", record.Monitor.Status, "final", true)
}
