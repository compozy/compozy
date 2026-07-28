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
	run, err := m.store.HeartbeatRunLease(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if run.IsNetworkWake() {
		m.dispatchTaskRunLeaseExtended(ctx, run, Task{}, actor)
		return &run, nil
	}
	taskRecord, err := m.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunLeaseExtended, actor, leaseExtendedPayload{
		Status:         run.Status,
		TaskStatus:     taskRecord.Status,
		LeaseUntil:     run.LeaseUntil,
		HeartbeatAt:    run.HeartbeatAt,
		ClaimTokenHash: run.ClaimTokenHash,
	}); err != nil {
		return nil, err
	}
	m.dispatchTaskRunLeaseExtended(ctx, run, taskRecord, actor)
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
	previous, err := m.store.GetTaskRun(ctx, normalized.RunID)
	if err != nil {
		return nil, err
	}
	run, err := m.store.ReleaseRunLease(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if run.IsNetworkWake() {
		m.dispatchTaskRunReleased(ctx, run, Task{}, actor, previous, normalized.Reason)
		return &run, nil
	}
	taskRecord, err := m.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return nil, err
	}
	reconciledTask, err := m.reconcileTaskCascade(ctx, taskRecord.ID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunReleased, actor, releasedRunPayload{
		PreviousStatus: previous.Status,
		Status:         run.Status,
		TaskStatus:     reconciledTask.Status,
		Reason:         normalized.Reason,
		SessionID:      previous.SessionID,
	}); err != nil {
		return nil, err
	}
	m.dispatchTaskRunReleased(ctx, run, reconciledTask, actor, previous, normalized.Reason)
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
	normalized, err := release.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	runs, err := m.activeSessionRunLeases(ctx, normalized.SessionID)
	if err != nil {
		return nil, err
	}

	results := make([]SessionLeaseReleaseResult, 0, len(runs))
	for _, previous := range runs {
		run := requeueSessionRunLease(previous)
		if err := m.store.UpdateTaskRun(ctx, run); err != nil {
			return nil, err
		}
		if run.IsNetworkWake() {
			m.dispatchTaskRunReleased(ctx, run, Task{}, actor, previous, normalized.Reason)
			results = append(results, SessionLeaseReleaseResult{
				Run: run, PreviousRunStatus: previous.Status,
				PreviousSessionID: previous.SessionID, PreviousLeaseUntil: previous.LeaseUntil,
				PreviousClaimTokenHash: previous.ClaimTokenHash, Reason: normalized.Reason,
			})
			continue
		}
		reconciledTask, err := m.reconcileTaskCascade(ctx, run.TaskID, actor)
		if err != nil {
			return nil, err
		}
		if err := m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunReleased, actor, releasedRunPayload{
			PreviousStatus: previous.Status,
			Status:         run.Status,
			TaskStatus:     reconciledTask.Status,
			Reason:         normalized.Reason,
			SessionID:      previous.SessionID,
		}); err != nil {
			return nil, err
		}
		m.dispatchTaskRunReleased(ctx, run, reconciledTask, actor, previous, normalized.Reason)
		results = append(results, SessionLeaseReleaseResult{
			Run:                    run,
			PreviousRunStatus:      previous.Status,
			PreviousSessionID:      previous.SessionID,
			PreviousLeaseUntil:     previous.LeaseUntil,
			PreviousClaimTokenHash: previous.ClaimTokenHash,
			Reason:                 normalized.Reason,
		})
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
	normalized.Actor = actor
	settlement, err := m.store.CompleteRunLeaseSettlement(ctx, normalized)
	if err != nil {
		if hallucinated, ok := errors.AsType[*HallucinatedTaskRefsError](err); ok {
			if eventErr := m.recordCompletionHallucinationBlocked(ctx, hallucinated, actor); eventErr != nil {
				return nil, errors.Join(err, eventErr)
			}
		}
		return nil, err
	}
	return m.publishCompletedLeaseSettlement(ctx, &settlement, actor)
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

func (m *Service) recordCompletionHallucinationSuspected(ctx context.Context, run Run, actor ActorContext) {
	suspectedTaskIDs := m.suspectedCompletionTaskIDs(ctx, run)
	if len(suspectedTaskIDs) == 0 {
		return
	}
	if err := m.recordTaskEvent(
		ctx,
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
	candidates := canonicalTaskIDTokens(run.Result)
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
