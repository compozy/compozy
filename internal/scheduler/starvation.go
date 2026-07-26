package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

// runConvergence advances the durable escalation budget for every starved-but-unclaimed run this
// cycle, then reconciles existing budget rows whose run has left the queued state. It never claims:
// it requests a capability-matched spawn (the spawned worker self-claims), emits the canonical
// starved event once, and finally marks the run needs_attention. Workspace/coordination isolation
// is enforced downstream — the spawn inherits the run's scope and the events carry its workspace_id.
func (s *Scheduler) runConvergence(
	ctx context.Context,
	now time.Time,
	starvationNotBefore time.Time,
	candidates []RunSnapshot,
	result *CycleResult,
) []error {
	var errs []error
	processed := make(map[string]struct{}, len(candidates))
	for idx := range candidates {
		work := &candidates[idx]
		runID := strings.TrimSpace(work.Run.ID)
		if runID == "" {
			continue
		}
		processed[runID] = struct{}{}
		if err := s.escalateCandidate(ctx, now, starvationNotBefore, work, result); err != nil {
			errs = append(errs, err)
		}
	}
	return append(errs, s.reconcileStarvationRows(ctx, processed)...)
}

func (s *Scheduler) escalateCandidate(
	ctx context.Context,
	now time.Time,
	starvationNotBefore time.Time,
	work *RunSnapshot,
	result *CycleResult,
) error {
	runID := strings.TrimSpace(work.Run.ID)
	if !isPotentiallyClaimable(work) {
		return nil
	}
	queuedAge, hasQueuedAge := effectiveQueuedAge(now, work.Run.QueuedAt, starvationNotBefore)
	if !hasQueuedAge || queuedAge < s.starveThresholds.MinQueuedAge {
		return nil
	}
	if shouldContinue, err := s.ensureCandidateStillQueued(ctx, runID); err != nil || !shouldContinue {
		return err
	}
	prev, _, err := s.starvation.LoadRunStarvation(ctx, runID)
	if err != nil {
		return fmt.Errorf("scheduler: load run starvation %q: %w", runID, err)
	}
	mutation := advanceStarvation(prev, runID, now, s.starveThresholds)
	var errs []error
	errs = append(errs, s.escalateSpawn(ctx, now, work, &mutation, result)...)
	errs = append(errs, s.escalateEvent(ctx, now, queuedAge, work, &mutation)...)
	cleared, attnErrs := s.escalateNeedsAttention(ctx, runID, queuedAge, mutation, result)
	errs = append(errs, attnErrs...)
	if cleared {
		return errors.Join(errs...)
	}
	if _, err := s.starvation.UpsertRunStarvation(ctx, mutation); err != nil {
		errs = append(errs, fmt.Errorf("scheduler: upsert run starvation %q: %w", runID, err))
	}
	return errors.Join(errs...)
}

func (s *Scheduler) ensureCandidateStillQueued(ctx context.Context, runID string) (bool, error) {
	status, found, err := s.tasks.GetRunStatus(ctx, runID)
	if err != nil {
		return false, fmt.Errorf("scheduler: read run status %q before starvation escalation: %w", runID, err)
	}
	if found && status.Normalize() == taskpkg.TaskRunStatusQueued {
		return true, nil
	}
	if found && status.Normalize() == taskpkg.TaskRunStatusNeedsAttention {
		return false, nil
	}
	if clearErr := s.starvation.ClearRunStarvation(ctx, runID); clearErr != nil {
		return false, fmt.Errorf("scheduler: clear stale run starvation %q: %w", runID, clearErr)
	}
	return false, nil
}

// escalateSpawn requests a capability-matched worker once the spawn tier is reached. The request
// is coalesced via spawn_requested_at; an unresolvable capability is not coalesced so a later
// agent can still be matched, and never blocks the event/needs_attention tiers.
func (s *Scheduler) escalateSpawn(
	ctx context.Context,
	now time.Time,
	work *RunSnapshot,
	mutation *taskpkg.RunStarvationMutation,
	result *CycleResult,
) []error {
	if mutation.WakeCount < s.starveThresholds.SpawnAfter || mutation.SpawnRequestedAt != nil {
		return nil
	}
	switch err := s.escalator.RequestWorkerSpawn(ctx, work); {
	case errors.Is(err, ErrSpawnUnresolvable):
		return nil
	case err != nil:
		return []error{fmt.Errorf("scheduler: request worker spawn %q: %w", work.Run.ID, err)}
	default:
		at := now
		mutation.SpawnRequestedAt = &at
		result.SpawnRequested++
		result.SpawnRequestedRunIDs = append(result.SpawnRequestedRunIDs, mutation.RunID)
		return nil
	}
}

// escalateEvent emits the canonical task.run_starved event exactly once per episode, gated by the
// durable starved_event_at so the emit-once guard survives daemon restart.
func (s *Scheduler) escalateEvent(
	ctx context.Context,
	now time.Time,
	queuedAge time.Duration,
	work *RunSnapshot,
	mutation *taskpkg.RunStarvationMutation,
) []error {
	if mutation.WakeCount < s.starveThresholds.EventAfter || mutation.StarvedEventAt != nil {
		return nil
	}
	if err := s.escalator.EmitRunStarved(ctx, work, queuedAge); err != nil {
		return []error{fmt.Errorf("scheduler: emit run starved %q: %w", work.Run.ID, err)}
	}
	at := now
	mutation.StarvedEventAt = &at
	return nil
}

// escalateNeedsAttention parks an unconverged run as the terminal escalation tier and clears its
// budget. A run that left the queued state between selection and now fails the CAS guard with
// ErrInvalidStatusTransition; that is benign — the budget is cleared and tracking stops.
func (s *Scheduler) escalateNeedsAttention(
	ctx context.Context,
	runID string,
	queuedAge time.Duration,
	mutation taskpkg.RunStarvationMutation,
	result *CycleResult,
) (bool, []error) {
	if mutation.WakeCount < s.starveThresholds.NeedsAttentionAfter {
		return false, nil
	}
	_, err := s.escalator.MarkRunNeedsAttention(ctx, runID, starvationDiagnostic(queuedAge, mutation))
	if err != nil && !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
		return false, []error{fmt.Errorf("scheduler: mark run needs attention %q: %w", runID, err)}
	}
	var errs []error
	if err == nil {
		result.NeedsAttention++
		result.NeedsAttentionRunIDs = append(result.NeedsAttentionRunIDs, runID)
	}
	if clearErr := s.starvation.ClearRunStarvation(ctx, runID); clearErr != nil {
		errs = append(errs, fmt.Errorf("scheduler: clear run starvation %q: %w", runID, clearErr))
	}
	return true, errs
}

// reconcileStarvationRows clears the budget for runs that left the queued set since they were last
// escalated. It re-reads the durable status rather than diffing the candidate set, so a paused
// (still-queued) run holds its clock instead of resetting it.
func (s *Scheduler) reconcileStarvationRows(ctx context.Context, processed map[string]struct{}) []error {
	rows, err := s.starvation.ListRunStarvation(ctx)
	if err != nil {
		return []error{fmt.Errorf("scheduler: list run starvation: %w", err)}
	}
	var errs []error
	for idx := range rows {
		runID := strings.TrimSpace(rows[idx].RunID)
		if runID == "" {
			continue
		}
		if _, ok := processed[runID]; ok {
			continue
		}
		status, found, err := s.tasks.GetRunStatus(ctx, runID)
		if err != nil {
			errs = append(errs, fmt.Errorf("scheduler: read run status %q: %w", runID, err))
			continue
		}
		if found && starvationStatusHolds(status) {
			continue
		}
		if clearErr := s.starvation.ClearRunStarvation(ctx, runID); clearErr != nil {
			errs = append(errs, fmt.Errorf("scheduler: clear run starvation %q: %w", runID, clearErr))
		}
	}
	return errs
}

func advanceStarvation(
	prev taskpkg.RunStarvation,
	runID string,
	now time.Time,
	thresholds StarvationThresholds,
) taskpkg.RunStarvationMutation {
	mutation := taskpkg.RunStarvationMutation{
		RunID:            runID,
		WakeCount:        prev.WakeCount + 1,
		FirstStarvedAt:   prev.FirstStarvedAt,
		LastWakeAt:       now,
		SpawnRequestedAt: prev.SpawnRequestedAt,
		StarvedEventAt:   prev.StarvedEventAt,
		UpdatedAt:        now,
	}
	if mutation.FirstStarvedAt.IsZero() {
		mutation.FirstStarvedAt = now
	}
	mutation.EscalationTier = starvationTier(mutation.WakeCount, thresholds)
	return mutation
}

func starvationTier(wakeCount int, thresholds StarvationThresholds) int {
	switch {
	case wakeCount >= thresholds.NeedsAttentionAfter:
		return 4
	case wakeCount >= thresholds.EventAfter:
		return 3
	case wakeCount >= thresholds.SpawnAfter:
		return 2
	case wakeCount >= thresholds.FanOutAfter:
		return 1
	default:
		return 0
	}
}

func starvationStatusHolds(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusQueued, taskpkg.TaskRunStatusNeedsAttention:
		return true
	default:
		return false
	}
}

func starvationDiagnostic(queuedAge time.Duration, mutation taskpkg.RunStarvationMutation) string {
	return fmt.Sprintf(
		"run queued %s without a claim after %d escalation cycles",
		max(queuedAge, 0).Round(time.Second),
		mutation.WakeCount,
	)
}

func effectiveQueuedAge(now time.Time, queuedAt time.Time, starvationNotBefore time.Time) (time.Duration, bool) {
	if queuedAt.IsZero() {
		return 0, false
	}
	effectiveQueuedAt := queuedAt
	if starvationNotBefore.After(effectiveQueuedAt) {
		effectiveQueuedAt = starvationNotBefore
	}
	return max(now.Sub(effectiveQueuedAt), 0), true
}
