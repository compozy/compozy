package task

import (
	"context"

	"fmt"
	"strings"
	"sync"
	"time"

	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
)

const forceOpsDocURL = "/docs/autonomy/task-runs-and-leases#force-operations"

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
			fmt.Sprintf("compozy task inspect %s", previous.ID),
			map[string]any{runEvidenceIDKey: previous.ID, leaseStatusKey: previous.Status.Normalize().String()},
			ErrInvalidStatusTransition,
		)
	}
	if err := m.preflightForceReleaseRun(previous, normalized, actor); err != nil {
		return nil, err
	}

	settlement, err := m.forceReleaseRunSettlement(ctx, previous, normalized, actor, m.now().UTC())
	if err != nil {
		return nil, err
	}
	mutation := settlement.mutation
	defer m.restoreTaskRunNetworkBestEffort(ctx, mutation.Previous.SessionID, mutation.Run.ID)
	m.dispatchTaskRunReleased(ctx, mutation.Run, settlement.task, actor, mutation.Previous, normalized.Reason)
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
	if err := m.preflightForceFailRun(previous, normalized, actor); err != nil {
		return nil, err
	}

	settlement, err := m.forceFailRunSettlement(ctx, previous, normalized, actor, m.now().UTC())
	if err != nil {
		return nil, err
	}
	mutation := settlement.mutation
	defer m.restoreTaskRunNetworkBestEffort(ctx, mutation.Previous.SessionID, mutation.Run.ID)
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
			fmt.Sprintf("compozy task inspect %s", source.ID),
			map[string]any{runEvidenceIDKey: source.ID, leaseStatusKey: source.Status.Normalize().String()},
			ErrInvalidStatusTransition,
		)
	}
	if err := m.requireRetryChainDepth(ctx, source); err != nil {
		return nil, err
	}

	newRunID, err := m.newID("run")
	if err != nil {
		return nil, fmt.Errorf("task: generate retry run id: %w", err)
	}
	if err := m.preflightRetryRun(source, newRunID, actor); err != nil {
		return nil, err
	}
	settlement, err := m.retryRunSettlement(
		ctx,
		source,
		newRunID,
		normalized,
		actor,
		m.now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	result := settlement.result
	m.dispatchTaskRunEnqueued(ctx, result.Run, settlement.task, actor, "")
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

	newRunID, err := m.newID("run")
	if err != nil {
		return nil, fmt.Errorf("task: generate recovery run id: %w", err)
	}
	if err := m.preflightRecoverRun(source, newRunID, normalized, actor); err != nil {
		return nil, err
	}
	settlement, err := m.recoverRunSettlement(
		ctx,
		source,
		newRunID,
		normalized,
		normalized.Metadata,
		actor,
		m.now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	result := settlement.result
	defer m.restoreTaskRunNetworkBestEffort(ctx, result.PreviousRun.SessionID, result.Run.ID)
	m.dispatchTaskRunEnqueued(ctx, result.Run, settlement.task, actor, "")
	return &result, nil
}
