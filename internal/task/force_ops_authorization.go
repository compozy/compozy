package task

import (
	"context"

	"fmt"
	"strings"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
)

// BulkForceReleaseRuns applies force release one row at a time to preserve per-row preconditions.
func (m *Service) BulkForceReleaseRuns(
	ctx context.Context,
	req BulkForceRunRequest,
	actor ActorContext,
) (BulkForceRunResult, error) {
	normalized, err := normalizeBulkForceRunRequest(req, false)
	if err != nil {
		return BulkForceRunResult{}, err
	}
	result := BulkForceRunResult{Items: make([]BulkForceRunItem, 0, len(normalized.RunIDs))}
	for _, id := range normalized.RunIDs {
		run, err := m.ForceReleaseRun(ctx, id, ForceReleaseRun{Reason: normalized.Reason}, actor)
		result.Items = append(result.Items, bulkForceRunItem(id, run, err))
	}
	return result, nil
}

// BulkForceFailRuns applies forced failure one row at a time to preserve per-row preconditions.
func (m *Service) BulkForceFailRuns(
	ctx context.Context,
	req BulkForceRunRequest,
	actor ActorContext,
) (BulkForceRunResult, error) {
	normalized, err := normalizeBulkForceRunRequest(req, true)
	if err != nil {
		return BulkForceRunResult{}, err
	}
	result := BulkForceRunResult{Items: make([]BulkForceRunItem, 0, len(normalized.RunIDs))}
	for _, id := range normalized.RunIDs {
		run, err := m.ForceFailRun(
			ctx,
			id,
			ForceFailRun{Reason: normalized.Reason, Metadata: normalized.Metadata},
			actor,
		)
		result.Items = append(result.Items, bulkForceRunItem(id, run, err))
	}
	return result, nil
}

func bulkForceRunItem(id string, run *Run, err error) BulkForceRunItem {
	item := BulkForceRunItem{RunID: strings.TrimSpace(id), OK: err == nil, Err: err}
	if run != nil {
		runCopy := *run
		item.Run = &runCopy
	}
	return item
}

func (m *Service) requireForceRunAuthority(actor ActorContext) error {
	if err := requireWriteAuthority(actor); err != nil {
		return err
	}
	if m.forceRecovery.AllowAgentForce || actor.Actor.Kind.Normalize() != ActorKindAgentSession {
		return nil
	}
	return forceRunDiagnosticError(
		diagnosticcontract.CodeForbiddenOperatorAction,
		"Force operation is disabled for agents",
		"task.recovery.allow_agent_force is false, so only non-agent operators can run this recovery action.",
		diagnosticcontract.SeverityError,
		"agh config set task.recovery.allow_agent_force true",
		map[string]any{"actor_kind": string(actor.Actor.Kind.Normalize()), "actor_id": actor.Actor.Ref},
		ErrForbiddenOperatorAction,
	)
}

func (m *Service) requireForceRunRate(actor ActorContext, taskID string) error {
	if actor.Actor.Kind.Normalize() != ActorKindAgentSession {
		return nil
	}
	now := m.now().UTC()
	if m.forceRateLimiter.allow(actor.Actor, taskID, now, m.forceRecovery.RateLimitPerMinute) {
		return nil
	}
	return forceRunDiagnosticError(
		diagnosticcontract.CodeForceOpRateLimited,
		"Force operation rate limit exceeded",
		fmt.Sprintf(
			"Actor %s exceeded %d force operations per minute for task %s.",
			actor.Actor.Ref,
			m.forceRecovery.RateLimitPerMinute,
			taskID,
		),
		diagnosticcontract.SeverityWarn,
		"agh task inspect "+taskID,
		map[string]any{
			"actor_kind":      string(actor.Actor.Kind.Normalize()),
			"actor_id":        actor.Actor.Ref,
			taskEvidenceIDKey: taskID,
			"limit":           m.forceRecovery.RateLimitPerMinute,
		},
		ErrForceOpRateLimited,
	)
}

func (m *Service) invalidateForceRunInputs(ctx context.Context, previous Run) (int64, int, error) {
	sessionID := strings.TrimSpace(previous.SessionID)
	if sessionID == "" {
		return 0, 0, nil
	}
	queueStore, ok := m.store.(inputQueueGenerationStore)
	if !ok {
		return 0, 0, nil
	}
	now := m.now().UTC()
	generation, err := queueStore.AdvanceSessionInputGeneration(ctx, sessionID, now)
	if err != nil {
		return 0, 0, fmt.Errorf("task: advance input generation for force operation on session %q: %w", sessionID, err)
	}
	canceled, err := queueStore.CancelPendingSessionInputs(ctx, sessionID, generation, now)
	if err != nil {
		return 0, 0, fmt.Errorf(
			"task: cancel stale input generation for force operation on session %q: %w",
			sessionID,
			err,
		)
	}
	return generation, canceled, nil
}
