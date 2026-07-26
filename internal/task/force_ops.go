package task

import (
	"context"

	"fmt"
	"strings"
	"sync"
	"time"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
)

const forceOpsDocURL = "/runtime/core/autonomy/task-runs-and-leases#force-operations"

type inputQueueGenerationStore interface {
	AdvanceSessionInputGeneration(ctx context.Context, sessionID string, now time.Time) (int64, error)
	CancelPendingSessionInputs(ctx context.Context, sessionID string, generation int64, now time.Time) (int, error)
}

type forceRunRateLimiter struct {
	mu      sync.Mutex
	windows map[string]forceRunRateWindow
}

type forceRunRateWindow struct {
	start time.Time
	count int
}

func newForceRunRateLimiter() *forceRunRateLimiter {
	return &forceRunRateLimiter{windows: make(map[string]forceRunRateWindow)}
}

func normalizeForceRecoveryOptions(options ForceRecoveryOptions) ForceRecoveryOptions {
	if options.RateLimitPerMinute <= 0 {
		options.RateLimitPerMinute = DefaultForceRunRateLimitPerMinute
	}
	return options
}

func (l *forceRunRateLimiter) allow(actor ActorIdentity, taskID string, now time.Time, limit int) bool {
	if l == nil || limit <= 0 {
		return true
	}
	key := string(actor.Kind.Normalize()) + ":" + strings.TrimSpace(actor.Ref) + ":" + strings.TrimSpace(taskID)
	l.mu.Lock()
	defer l.mu.Unlock()

	window := l.windows[key]
	if window.start.IsZero() || now.Sub(window.start) >= time.Minute {
		l.windows[key] = forceRunRateWindow{start: now, count: 1}
		return true
	}
	if window.count >= limit {
		return false
	}
	window.count++
	l.windows[key] = window
	return true
}

// ForceReleaseRun releases one claimed run without requiring the raw claim token.
func (m *Service) ForceReleaseRun(
	ctx context.Context,
	runID string,
	release ForceReleaseRun,
	actor ActorContext,
) (*Run, error) {
	if err := m.requireForceRunAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := normalizeForceReleaseRun(release)
	if err != nil {
		return nil, err
	}
	previous, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.requireForceRunRate(actor, taskRecord.ID); err != nil {
		return nil, err
	}
	if previous.Status.Normalize() != TaskRunStatusClaimed {
		return nil, forceRunDiagnosticError(
			diagnosticcontract.CodeTaskRunNotReleasable,
			"Task run cannot be force released",
			fmt.Sprintf(
				"Run %s is %s; only claimed runs can be force released.",
				previous.ID,
				previous.Status.Normalize(),
			),
			diagnosticcontract.SeverityError,
			fmt.Sprintf("agh task inspect %s", previous.ID),
			map[string]any{runEvidenceIDKey: previous.ID, leaseStatusKey: previous.Status.Normalize().String()},
			ErrInvalidStatusTransition,
		)
	}

	mutation, err := m.store.ForceReleaseTaskRun(ctx, ForceReleaseRunMutation{
		RunID: previous.ID,
		Now:   m.now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	queueGeneration, canceledInputs, err := m.invalidateForceRunInputs(ctx, mutation.Previous)
	if err != nil {
		return nil, err
	}
	reconciledTask, err := m.reconcileTaskCascade(ctx, taskRecord.ID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(
		ctx,
		mutation.Run.TaskID,
		mutation.Run.ID,
		taskEventRunReleased,
		actor,
		releasedRunPayload{
			Manual:                          true,
			ActorKind:                       actor.Actor.Kind.Normalize(),
			ActorID:                         actor.Actor.Ref,
			PreviousStatus:                  mutation.Previous.Status,
			Status:                          mutation.Run.Status,
			TaskStatus:                      reconciledTask.Status,
			Reason:                          normalized.Reason,
			SessionID:                       mutation.Previous.SessionID,
			PreviousSessionID:               mutation.Previous.SessionID,
			PreviousClaimTokenHashTruncated: truncateClaimTokenHash(mutation.Previous.ClaimTokenHash),
			PreviousLeaseUntil:              optionalPayloadTime(mutation.Previous.LeaseUntil),
			QueueGeneration:                 queueGeneration,
			CanceledQueuedInputs:            canceledInputs,
		},
	); err != nil {
		return nil, err
	}
	m.dispatchTaskRunReleased(ctx, mutation.Run, reconciledTask, actor, mutation.Previous, normalized.Reason)
	return &mutation.Run, nil
}

// ForceFailRun marks one queued or claimed run failed without requiring the raw claim token.
func (m *Service) ForceFailRun(
	ctx context.Context,
	runID string,
	failure ForceFailRun,
	actor ActorContext,
) (*Run, error) {
	if err := m.requireForceRunAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := normalizeForceFailRun(failure)
	if err != nil {
		return nil, err
	}
	previous, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.requireForceRunRate(actor, taskRecord.ID); err != nil {
		return nil, err
	}
	if err := requireForceFailStatus(previous); err != nil {
		return nil, err
	}

	mutation, err := m.store.ForceFailTaskRun(ctx, ForceFailRunMutation{
		RunID:  previous.ID,
		Reason: normalized.Reason,
		Now:    m.now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	queueGeneration, canceledInputs, err := m.invalidateForceRunInputs(ctx, mutation.Previous)
	if err != nil {
		return nil, err
	}
	reconciledTask, err := m.reconcileTaskCascade(ctx, taskRecord.ID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(
		ctx,
		mutation.Run.TaskID,
		mutation.Run.ID,
		taskEventRunOperatorForcedFail,
		actor,
		operatorForcedFailPayload{
			Manual:               true,
			ActorKind:            actor.Actor.Kind.Normalize(),
			ActorID:              actor.Actor.Ref,
			PreviousStatus:       mutation.Previous.Status,
			Status:               mutation.Run.Status,
			TaskStatus:           reconciledTask.Status,
			Reason:               normalized.Reason,
			SessionID:            mutation.Previous.SessionID,
			QueueGeneration:      queueGeneration,
			CanceledQueuedInputs: canceledInputs,
			Metadata:             cloneRawJSON(normalized.Metadata),
		},
	); err != nil {
		return nil, err
	}
	return &mutation.Run, nil
}

// RetryRun creates one new queued run linked to a failed source run.
func (m *Service) RetryRun(
	ctx context.Context,
	runID string,
	retry RetryRunRequest,
	actor ActorContext,
) (*RetryRunResult, error) {
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	if err := m.requireForceRunAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := normalizeRetryRunRequest(retry)
	if err != nil {
		return nil, err
	}
	source, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.requireForceRunRate(actor, taskRecord.ID); err != nil {
		return nil, err
	}
	if source.Status.Normalize() != TaskRunStatusFailed {
		return nil, forceRunDiagnosticError(
			diagnosticcontract.CodeTaskRunStillActive,
			"Task run cannot be retried",
			fmt.Sprintf("Run %s is %s; only failed runs can be retried.", source.ID, source.Status.Normalize()),
			diagnosticcontract.SeverityError,
			fmt.Sprintf("agh task inspect %s", source.ID),
			map[string]any{runEvidenceIDKey: source.ID, leaseStatusKey: source.Status.Normalize().String()},
			ErrInvalidStatusTransition,
		)
	}
	if err := m.requireRetryChainDepth(ctx, source); err != nil {
		return nil, err
	}

	result, err := m.store.RetryTaskRun(ctx, RetryRunMutation{
		SourceRunID: source.ID,
		NewRunID:    m.newID("run"),
		Origin:      actor.Origin,
		Metadata:    normalized.Metadata,
		QueuedAt:    m.now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	reconciledTask, err := m.reconcileTaskCascade(ctx, taskRecord.ID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(
		ctx,
		result.Run.TaskID,
		result.Run.ID,
		taskEventRunOperatorRetry,
		actor,
		operatorRetryPayload{
			Manual:      true,
			ActorKind:   actor.Actor.Kind.Normalize(),
			ActorID:     actor.Actor.Ref,
			SourceRunID: result.PreviousRun.ID,
			NewRunID:    result.Run.ID,
			TaskStatus:  reconciledTask.Status,
		},
	); err != nil {
		return nil, err
	}
	m.dispatchTaskRunEnqueued(ctx, result.Run, reconciledTask, actor, "")
	return &result, nil
}

// RecoverRun terminalizes a needs_attention run and queues one fresh child to resume work.
func (m *Service) RecoverRun(
	ctx context.Context,
	runID string,
	req RecoverRunRequest,
	actor ActorContext,
) (*RetryRunResult, error) {
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	if err := m.requireForceRunAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := normalizeRecoverRunRequest(req)
	if err != nil {
		return nil, err
	}
	source, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.requireForceRunRate(actor, taskRecord.ID); err != nil {
		return nil, err
	}
	if err := validateRecoverRunStatus(&source); err != nil {
		return nil, err
	}
	if err := m.requireRetryChainDepth(ctx, source); err != nil {
		return nil, err
	}

	result, err := m.store.RecoverTaskRun(ctx, RecoverRunMutation{
		SourceRunID: source.ID,
		NewRunID:    m.newID("run"),
		Origin:      actor.Origin,
		Reason:      normalized.Reason,
		Metadata:    normalized.Metadata,
		QueuedAt:    m.now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	reconciledTask, err := m.reconcileTaskCascade(ctx, taskRecord.ID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(
		ctx,
		result.Run.TaskID,
		result.Run.ID,
		taskEventRunRecoveredFromAttention,
		actor,
		recoveredFromAttentionPayload{
			Manual:         true,
			ActorKind:      actor.Actor.Kind.Normalize(),
			ActorID:        actor.Actor.Ref,
			SourceRunID:    result.PreviousRun.ID,
			NewRunID:       result.Run.ID,
			PreviousStatus: TaskRunStatusNeedsAttention,
			Status:         result.Run.Status.Normalize(),
			TaskStatus:     reconciledTask.Status,
			Reason:         normalized.Reason,
		},
	); err != nil {
		return nil, err
	}
	m.dispatchTaskRunEnqueued(ctx, result.Run, reconciledTask, actor, "")
	return &result, nil
}
