package task

import (
	"context"

	"github.com/compozy/agh/internal/store"
)

// SettleNetworkWake atomically terminalizes one claimed taskless wake and publishes committed hooks.
func (m *Service) SettleNetworkWake(
	ctx context.Context,
	settlement NetworkWakeSettlement,
	actor ActorContext,
) (*NetworkWakeSettlementResult, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	settlement.Actor = actor
	normalized, err := settlement.Normalize(m.now().UTC())
	if err != nil {
		return nil, err
	}
	result, err := m.store.SettleNetworkWake(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if result.Duplicate {
		return &result, nil
	}
	if result.Outcome.State == store.NetworkWakeStateSucceeded {
		if _, err := m.publishCompletedLeaseSettlement(
			ctx,
			&CompletedRunSettlement{Run: result.Run},
			actor,
		); err != nil {
			return nil, err
		}
		return &result, nil
	}
	m.dispatchTaskRunFailed(ctx, result.Run, Task{}, actor)
	return &result, nil
}
