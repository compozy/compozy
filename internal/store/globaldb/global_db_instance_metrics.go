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
	return result, err
}
