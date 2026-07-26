package task

import (
	"context"
	"time"
)

func (m *Service) schedulerStatus(
	ctx context.Context,
	controlStore schedulerControlStore,
	asOf time.Time,
) (SchedulerStatus, error) {
	state, err := controlStore.GetSchedulerPause(ctx)
	if err != nil {
		return SchedulerStatus{}, err
	}
	active, err := controlStore.CountActiveTaskRunClaims(ctx)
	if err != nil {
		return SchedulerStatus{}, err
	}
	queued, err := controlStore.CountQueuedTaskRuns(ctx, true)
	if err != nil {
		return SchedulerStatus{}, err
	}
	pausedTasks, err := controlStore.CountPausedTasks(ctx)
	if err != nil {
		return SchedulerStatus{}, err
	}
	starved := 0
	if schedulerStarvationWindowOpen(state, asOf, m.starvationAge) {
		starved, err = controlStore.CountEscalatingQueuedTaskRuns(ctx)
		if err != nil {
			return SchedulerStatus{}, err
		}
	}
	needsAttention, err := controlStore.CountNeedsAttentionTaskRuns(ctx)
	if err != nil {
		return SchedulerStatus{}, err
	}
	return SchedulerStatus{
		Paused:                 state.Paused,
		PausedBy:               state.PausedBy,
		PausedAt:               state.PausedAt,
		PausedReason:           state.Reason,
		ActiveClaimCount:       active,
		QueuedRunCount:         queued,
		PausedTaskCount:        pausedTasks,
		StarvedRunCount:        starved,
		NeedsAttentionRunCount: needsAttention,
		AsOf:                   asOf,
	}, nil
}

func schedulerStarvationWindowOpen(
	state SchedulerPauseState,
	asOf time.Time,
	minQueuedAge time.Duration,
) bool {
	if state.Paused || minQueuedAge <= 0 {
		return false
	}
	return state.UpdatedAt.IsZero() || !asOf.Before(state.UpdatedAt.Add(minQueuedAge))
}
