package task

import (
	"context"

	"fmt"
	"strings"

	eventspkg "github.com/compozy/agh/internal/events"
)

// PauseScheduler marks the daemon scheduler as paused for new dispatch and claims.
func (m *Service) PauseScheduler(
	ctx context.Context,
	req SchedulerPauseRequest,
	actor ActorContext,
) (SchedulerStatus, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return SchedulerStatus{}, err
	}
	controlStore, err := m.requireSchedulerControlStore()
	if err != nil {
		return SchedulerStatus{}, err
	}
	previous, err := controlStore.GetSchedulerPause(ctx)
	if err != nil {
		return SchedulerStatus{}, err
	}
	reason := strings.TrimSpace(req.Reason)
	state, err := controlStore.SetSchedulerPaused(ctx, actorLabel(actor), reason)
	if err != nil {
		return SchedulerStatus{}, err
	}
	status, err := m.schedulerStatus(ctx, controlStore, m.now().UTC())
	if err != nil {
		return SchedulerStatus{}, err
	}
	m.recordSchedulerEventBestEffort(ctx, eventspkg.SchedulerPaused, actor, schedulerEventPayload{
		Manual:         true,
		ActorKind:      actor.Actor.Kind.Normalize(),
		ActorID:        actor.Actor.Ref,
		Reason:         reason,
		PreviousPaused: previous.Paused,
		Paused:         state.Paused,
		ActiveClaims:   status.ActiveClaimCount,
	})
	return status, nil
}

// ResumeScheduler clears the daemon scheduler pause flag.
func (m *Service) ResumeScheduler(
	ctx context.Context,
	_ SchedulerResumeRequest,
	actor ActorContext,
) (SchedulerStatus, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return SchedulerStatus{}, err
	}
	controlStore, err := m.requireSchedulerControlStore()
	if err != nil {
		return SchedulerStatus{}, err
	}
	previous, err := controlStore.GetSchedulerPause(ctx)
	if err != nil {
		return SchedulerStatus{}, err
	}
	state, err := controlStore.SetSchedulerResumed(ctx)
	if err != nil {
		return SchedulerStatus{}, err
	}
	status, err := m.schedulerStatus(ctx, controlStore, m.now().UTC())
	if err != nil {
		return SchedulerStatus{}, err
	}
	m.recordSchedulerEventBestEffort(ctx, eventspkg.SchedulerResumed, actor, schedulerEventPayload{
		Manual:         true,
		ActorKind:      actor.Actor.Kind.Normalize(),
		ActorID:        actor.Actor.Ref,
		PreviousPaused: previous.Paused,
		Paused:         state.Paused,
		ActiveClaims:   status.ActiveClaimCount,
	})
	return status, nil
}

// DrainScheduler pauses the scheduler and waits until active claims reach zero or the timeout expires.
func (m *Service) DrainScheduler(
	ctx context.Context,
	req SchedulerDrainRequest,
	actor ActorContext,
) (SchedulerDrainResult, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return SchedulerDrainResult{}, err
	}
	controlStore, err := m.requireSchedulerControlStore()
	if err != nil {
		return SchedulerDrainResult{}, err
	}
	startedAt := m.now().UTC()
	timeout := req.Timeout
	if timeout < 0 {
		return SchedulerDrainResult{}, fmt.Errorf("%w: scheduler drain timeout must be non-negative", ErrValidation)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "scheduler drain"
	}
	if _, err := controlStore.SetSchedulerPaused(ctx, actorLabel(actor), reason); err != nil {
		return SchedulerDrainResult{}, err
	}
	m.recordSchedulerEventBestEffort(ctx, eventspkg.SchedulerDrainStarted, actor, schedulerEventPayload{
		Manual:    true,
		ActorKind: actor.Actor.Kind.Normalize(),
		ActorID:   actor.Actor.Ref,
		Reason:    reason,
		Paused:    true,
		StartedAt: startedAt,
	})

	drainCtx, cancelDrain := detachedSchedulerDrainContext(ctx, timeout)
	defer cancelDrain()
	result, err := m.waitForSchedulerDrain(drainCtx, controlStore, timeout, startedAt)
	if err != nil {
		return SchedulerDrainResult{}, err
	}
	m.recordSchedulerEventBestEffort(ctx, eventspkg.SchedulerDrainCompleted, actor, schedulerEventPayload{
		Manual:          true,
		ActorKind:       actor.Actor.Kind.Normalize(),
		ActorID:         actor.Actor.Ref,
		Reason:          reason,
		Paused:          result.Status.Paused,
		RemainingClaims: result.RemainingClaims,
		TimedOut:        result.TimedOut,
		StartedAt:       result.StartedAt,
		CompletedAt:     result.CompletedAt,
	})
	return result, nil
}

// SchedulerBacklog returns queued scheduler backlog rows.
func (m *Service) SchedulerBacklog(
	ctx context.Context,
	query SchedulerBacklogQuery,
	actor ActorContext,
) (SchedulerBacklog, error) {
	if err := requireReadAuthority(actor); err != nil {
		return SchedulerBacklog{}, err
	}
	controlStore, err := m.requireSchedulerControlStore()
	if err != nil {
		return SchedulerBacklog{}, err
	}
	return controlStore.SchedulerBacklog(ctx, query)
}
