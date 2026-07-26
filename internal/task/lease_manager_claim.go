package task

import "context"

// ClaimNextRun atomically claims the next eligible run for one session and returns the raw claim token once.
func (m *Service) ClaimNextRun(
	ctx context.Context,
	criteria ClaimCriteria,
	actor ActorContext,
) (*ClaimResult, error) {
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := m.normalizeClaimCriteriaForActor(criteria, actor)
	if err != nil {
		return nil, err
	}
	patched, err := m.dispatchTaskRunPreClaimCriteria(ctx, normalized, actor)
	if err != nil {
		return nil, err
	}

	result, err := m.store.ClaimNextRun(ctx, patched)
	if err != nil {
		return nil, err
	}
	claimResultWithoutRawTokenInMetadata(&result)
	if result.Run.IsNetworkWake() {
		m.dispatchTaskRunPostClaim(ctx, result.Run, Task{}, actor)
		return &result, nil
	}

	reconciledTask, err := m.reconcileTaskCascade(ctx, result.Run.TaskID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(ctx, result.Run.TaskID, result.Run.ID, taskEventRunClaimed, actor, runClaimedPayload{
		Status:         result.Run.Status,
		TaskStatus:     reconciledTask.Status,
		ClaimedBy:      ActorIdentity{Kind: actor.Actor.Kind, Ref: actor.Actor.Ref},
		ClaimTokenHash: result.Run.ClaimTokenHash,
		LeaseUntil:     result.Run.LeaseUntil,
	}); err != nil {
		return nil, err
	}
	m.dispatchTaskRunPostClaim(ctx, result.Run, reconciledTask, actor)
	result.Task = &reconciledTask
	return &result, nil
}
