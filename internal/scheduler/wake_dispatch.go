package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type wakeKey struct {
	runID     string
	sessionID string
}

func applySelection(result *CycleResult, selection selectionResult) {
	result.WakeAttempts = len(selection.targets)
	result.NoMatchRuns = len(selection.noMatch)
	result.RecentlyNotified = selection.recentlyNotified
	result.UnclaimableRuns = selection.unclaimable
	result.StarvedRuns = len(selection.starved)
	result.CapacityWaitingRuns = len(selection.capacityWaiting)
	result.NoMatchRunIDs = runIDs(selection.noMatch)
	result.StarvedRunIDs = runIDs(selection.starved)
	result.CapacityWaitingRunIDs = runIDs(selection.capacityWaiting)
}

func (s *Scheduler) dispatchWakeTargets(
	ctx context.Context,
	now time.Time,
	targets []WakeTarget,
	result *CycleResult,
) []error {
	for idx := range targets {
		targets[idx].Reason = s.wakeReason
	}
	if batchWaker, ok := s.waker.(BatchWaker); ok {
		return s.dispatchWakeBatch(ctx, now, targets, result, batchWaker)
	}

	var errs []error
	for idx := range targets {
		target := &targets[idx]
		if err := s.waker.Wake(ctx, target); err != nil {
			errs = append(errs, fmt.Errorf(
				"scheduler: wake session %q for run %q: %w",
				target.Session.ID,
				target.Work.Run.ID,
				err,
			))
			s.recordDispatchWakeFailure(result, target, err)
			continue
		}
		s.recordDispatchWakeSuccess(now, result, target)
	}
	return errs
}

func (s *Scheduler) dispatchWakeBatch(
	ctx context.Context,
	now time.Time,
	targets []WakeTarget,
	result *CycleResult,
	batchWaker BatchWaker,
) []error {
	wakeErrs := batchWaker.WakeMany(ctx, append([]WakeTarget(nil), targets...))
	if len(wakeErrs) != len(targets) {
		err := fmt.Errorf(
			"scheduler: batch waker returned %d errors for %d targets",
			len(wakeErrs),
			len(targets),
		)
		for idx := range targets {
			s.recordDispatchWakeFailure(result, &targets[idx], err)
		}
		return []error{err}
	}

	var errs []error
	for idx := range targets {
		target := &targets[idx]
		if err := wakeErrs[idx]; err != nil {
			errs = append(errs, fmt.Errorf(
				"scheduler: wake session %q for run %q: %w",
				target.Session.ID,
				target.Work.Run.ID,
				err,
			))
			s.recordDispatchWakeFailure(result, target, err)
			continue
		}
		s.recordDispatchWakeSuccess(now, result, target)
	}
	return errs
}

func (s *Scheduler) recordDispatchWakeFailure(result *CycleResult, target *WakeTarget, err error) {
	result.WakeFailed++
	s.recordWakeError(err)
	s.logger.Warn(
		"scheduler.wake.error",
		"session_id", target.Session.ID,
		"run_id", target.Work.Run.ID,
		"task_id", target.Work.Task.ID,
		"error", err,
	)
}

func (s *Scheduler) recordDispatchWakeSuccess(now time.Time, result *CycleResult, target *WakeTarget) {
	result.WakeSucceeded++
	result.SelectedRunIDs = append(result.SelectedRunIDs, target.Work.Run.ID)
	s.markWoken(now, target)
	s.logger.Info(
		"scheduler.wake",
		"session_id", target.Session.ID,
		"run_id", target.Work.Run.ID,
		"task_id", target.Work.Task.ID,
	)
}

func (s *Scheduler) wakeStateSnapshot(now time.Time) map[wakeKey]time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := make(map[wakeKey]time.Time, len(s.wakeState))
	for key, last := range s.wakeState {
		if s.wakeCooldown > 0 && now.Sub(last) >= s.wakeCooldown {
			delete(s.wakeState, key)
			continue
		}
		snapshot[key] = last
	}
	return snapshot
}

func (s *Scheduler) markWoken(now time.Time, target *WakeTarget) {
	if target == nil || s.wakeCooldown <= 0 {
		return
	}
	key := wakeKey{
		runID:     strings.TrimSpace(target.Work.Run.ID),
		sessionID: strings.TrimSpace(target.Session.ID),
	}
	if key.runID == "" || key.sessionID == "" {
		return
	}
	s.mu.Lock()
	s.wakeState[key] = now
	s.mu.Unlock()
}

func (s *Scheduler) recordRecovered(count int, now time.Time) {
	s.mu.Lock()
	s.stats.RecoveredLeases += count
	s.stats.LastCycleAt = now
	s.mu.Unlock()
}

func (s *Scheduler) recordExpiredBlocks(count int, now time.Time) {
	s.mu.Lock()
	s.stats.ExpiredBlocks += count
	s.stats.LastCycleAt = now
	s.mu.Unlock()
}

func (s *Scheduler) recordRecoveryError(err error) {
	s.mu.Lock()
	s.stats.RecoveryErrors++
	s.stats.LastRecoveryError = err.Error()
	s.mu.Unlock()
}

func (s *Scheduler) recordExpiryError(err error) {
	s.mu.Lock()
	s.stats.ExpiryErrors++
	s.stats.LastExpiryError = err.Error()
	s.mu.Unlock()
}

func (s *Scheduler) recordWakeError(err error) {
	s.mu.Lock()
	s.stats.LastWakeError = err.Error()
	s.mu.Unlock()
}

func (s *Scheduler) recordCycle(now time.Time, result CycleResult) {
	s.mu.Lock()
	s.stats.Cycles++
	s.stats.WakeAttempts += result.WakeAttempts
	s.stats.WakeSucceeded += result.WakeSucceeded
	s.stats.WakeFailed += result.WakeFailed
	s.stats.NoMatchRuns += result.NoMatchRuns
	s.stats.RecentlyNotified += result.RecentlyNotified
	s.stats.UnclaimableRuns += result.UnclaimableRuns
	s.stats.StarvedRuns += result.StarvedRuns
	s.stats.CapacityWaitingRuns += result.CapacityWaitingRuns
	s.stats.SpawnRequested += result.SpawnRequested
	s.stats.NeedsAttention += result.NeedsAttention
	s.stats.LastCycleAt = now
	s.mu.Unlock()
}
