package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// HeartbeatRunLease extends one active task-run lease after token verification.
func (m *Service) HeartbeatRunLease(
	ctx context.Context,
	heartbeat LeaseHeartbeat,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := heartbeat.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	normalized.Actor = actor
	settlement, err := m.heartbeatRunLeaseSettlement(ctx, normalized, actor)
	if err != nil {
		return nil, err
	}
	run := settlement.Run
	if run.IsNetworkWake() {
		m.dispatchTaskRunLeaseExtended(ctx, run, Task{}, actor)
		return &run, nil
	}
	m.dispatchTaskRunLeaseExtended(ctx, run, settlement.Task, actor)
	return &run, nil
}

// BindLeasedRunSession records the executing session for one active leased run after token verification.
func (m *Service) BindLeasedRunSession(
	ctx context.Context,
	binding LeaseSessionBinding,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := binding.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	normalized.Actor = actor
	settlement, err := m.bindLeasedRunSessionSettlement(ctx, normalized, actor)
	if err != nil {
		return nil, err
	}
	run := settlement.Run
	return &run, nil
}

// ReleaseRunLease releases one active task-run lease after token verification and requeues the run.
func (m *Service) ReleaseRunLease(
	ctx context.Context,
	release LeaseRelease,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := release.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	normalized.Actor = actor
	settlement, err := m.releaseRunLeaseSettlement(ctx, normalized, actor)
	if err != nil {
		return nil, err
	}
	run := settlement.Run
	previous := settlement.PreviousRun
	defer m.restoreTaskRunNetworkBestEffort(ctx, previous.SessionID, run.ID)
	if run.IsNetworkWake() {
		m.dispatchTaskRunReleased(ctx, run, Task{}, actor, previous, normalized.Reason)
		return &run, nil
	}
	m.dispatchTaskRunReleased(ctx, run, settlement.Task, actor, previous, normalized.Reason)
	return &run, nil
}

// ReleaseSessionRunLeases structurally releases every active task-run lease
// bound to one session without requiring the raw claim token. This is reserved
// for daemon-owned runtime cleanup paths such as safe-spawn reaping.
func (m *Service) ReleaseSessionRunLeases(
	ctx context.Context,
	release SessionLeaseRelease,
	actor ActorContext,
) ([]SessionLeaseReleaseResult, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	if actor.Actor.Kind.Normalize() != ActorKindDaemon {
		return nil, ErrPermissionDenied
	}
	normalized, err := release.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	settlement, err := m.releaseSessionRunLeasesSettlement(ctx, normalized, actor)
	if err != nil {
		return nil, err
	}

	results := make([]SessionLeaseReleaseResult, 0, len(settlement.outcomes))
	for index := range settlement.outcomes {
		outcome := &settlement.outcomes[index]
		m.dispatchTaskRunReleased(ctx, outcome.result.Run, outcome.task, actor, outcome.previous, normalized.Reason)
		m.restoreTaskRunNetworkBestEffort(ctx, outcome.previous.SessionID, outcome.result.Run.ID)
		results = append(results, outcome.result)
	}
	return results, nil
}

// CompleteRunLease marks one active task-run lease complete after token verification.
func (m *Service) CompleteRunLease(
	ctx context.Context,
	completion LeaseCompletion,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := completion.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	storedResult, err := normalized.Result.StoredValue()
	if err != nil {
		return nil, err
	}
	if err := m.enforceResultBudget(storedResult); err != nil {
		return nil, err
	}
	normalized.Actor = actor
	advisoryEventID, err := m.reserveTaskEventID()
	if err != nil {
		return nil, err
	}
	settlement, err := m.store.CompleteRunLeaseSettlement(ctx, normalized)
	if err != nil {
		if hallucinated, ok := errors.AsType[*HallucinatedTaskRefsError](err); ok {
			if eventErr := m.recordCompletionHallucinationBlocked(ctx, hallucinated, actor); eventErr != nil {
				return nil, errors.Join(err, eventErr)
			}
		}
		return nil, err
	}
	return m.publishCompletedLeaseSettlement(ctx, &settlement, advisoryEventID, actor)
}

func (m *Service) recordCompletionHallucinationBlocked(
	ctx context.Context,
	err *HallucinatedTaskRefsError,
	actor ActorContext,
) error {
	if err == nil {
		return nil
	}
	if recordErr := m.recordTaskEvent(
		ctx,
		err.TaskID,
		err.RunID,
		taskEventCompletionHallucinationBlocked,
		actor,
		completionHallucinationBlockedPayload{
			Status:         err.RunStatus,
			SessionID:      err.SessionID,
			ClaimedTaskIDs: err.ClaimedTaskIDs,
			InvalidTaskIDs: err.InvalidTaskIDs,
			ClaimTokenHash: err.ClaimTokenHash,
		},
	); recordErr != nil {
		return fmt.Errorf("task: record completion hallucination block event: %w", recordErr)
	}
	return nil
}

func (m *Service) recordCompletionHallucinationSuspected(
	ctx context.Context,
	eventID string,
	run Run,
	actor ActorContext,
) {
	suspectedTaskIDs := m.suspectedCompletionTaskIDs(ctx, run)
	if len(suspectedTaskIDs) == 0 {
		return
	}
	if err := m.recordTaskEventWithID(
		ctx,
		eventID,
		run.TaskID,
		run.ID,
		taskEventCompletionHallucinationSuspected,
		actor,
		completionHallucinationSuspectedPayload{
			Status:           run.Status,
			SuspectedTaskIDs: suspectedTaskIDs,
			ClaimTokenHash:   run.ClaimTokenHash,
		},
	); err != nil {
		slog.Warn(
			"task: completion hallucination advisory event failed",
			"task_id", run.TaskID,
			"run_id", run.ID,
			"error", err,
		)
	}
}

func (m *Service) suspectedCompletionTaskIDs(ctx context.Context, run Run) []string {
	candidates := canonicalTaskIDTokens(rawJSONValue(run.Result))
	if len(candidates) == 0 {
		return nil
	}
	suspected := make([]string, 0)
	for _, candidate := range candidates {
		if _, err := m.store.GetTask(ctx, candidate); err != nil {
			if errors.Is(err, ErrTaskNotFound) {
				suspected = append(suspected, candidate)
				continue
			}
			slog.Warn(
				"task: completion hallucination advisory lookup failed",
				"task_id", run.TaskID,
				"run_id", run.ID,
				"candidate_task_id", candidate,
				"error", err,
			)
		}
	}
	return suspected
}
