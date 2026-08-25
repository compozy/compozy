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
	normalized, err := m.normalizeClaimCriteriaForActor(ctx, criteria, actor)
	if err != nil {
		return nil, err
	}
	patched, err := m.dispatchTaskRunPreClaimCriteria(ctx, normalized, actor)
	if err != nil {
		return nil, err
	}

	result, settlement, err := m.claimNextRunSettlement(ctx, patched, actor)
	if err != nil {
		return nil, err
	}
	if result.Run.IsTaskless() {
		m.dispatchTaskRunPostClaim(ctx, result.Run, Task{}, actor)
		return &result, nil
	}

	m.dispatchTaskRunPostClaim(ctx, result.Run, settlement.Task, actor)
	if err := m.bindClaimedRunNetwork(ctx, &result, actor); err != nil {
		return nil, err
	}
	return &result, nil
}
