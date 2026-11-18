package globaldb

import (
	"context"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (r *InstanceRepo) Metrics(ctx context.Context, profileID, workspaceID string) (agentinstance.Metrics, error) {
	result := agentinstance.Metrics{PauseCounts: map[string]int64{}}
	if err := r.checkReady(ctx, "read instance metrics"); err != nil {
		return result, err
	}
	rows, err := r.readQueries.CountInstancePausesByReason(ctx, sqlcgen.CountInstancePausesByReasonParams{
		ProfileID: profileID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.PauseCounts[row.PauseReason] = row.Count
	}
	result.HeldInputCount, err = r.readQueries.CountScopedInstanceHeldInputs(
		ctx,
		sqlcgen.CountScopedInstanceHeldInputsParams{
			ProfileID: profileID, WorkspaceID: workspaceID,
		},
	)
	if err != nil {
		return result, err
	}
	result.CheckInToolCalls = map[string]map[string]int64{}
	calls, err := r.readQueries.CountScopedCheckInCalls(
		ctx,
		sqlcgen.CountScopedCheckInCallsParams{ProfileID: profileID, WorkspaceID: workspaceID},
	)
	if err != nil {
		return result, err
	}
	for _, row := range calls {
		if result.CheckInToolCalls[row.Access] == nil {
			result.CheckInToolCalls[row.Access] = map[string]int64{}
		}
		result.CheckInToolCalls[row.Access][row.Decision] = row.Count
	}
	result.Picks = map[string]int64{}
	picks, err := r.readQueries.CountScopedInstancePicks(
		ctx,
		sqlcgen.CountScopedInstancePicksParams{ProfileID: profileID, WorkspaceID: workspaceID},
	)
	if err != nil {
		return result, err
	}
	for _, row := range picks {
		result.Picks[row.Actor] = row.Count
	}
	return result, nil
}
