package globaldb

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ agentinstance.CheckInReader = (*InstanceRepo)(nil)

func (r *InstanceRepo) ListCheckIns(
	ctx context.Context,
	id, cursor string,
	limit int,
) ([]agentinstance.CheckIn, error) {
	rows, err := r.readQueries.ListInstanceCheckIns(
		ctx,
		sqlcgen.ListInstanceCheckInsParams{InstanceID: id, Cursor: cursor, PageLimit: int64(limit)},
	)
	if err != nil {
		return nil, err
	}
	result := make([]agentinstance.CheckIn, 0, len(rows))
	for _, row := range rows {
		at, err := store.ParseTimestamp(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		calls, err := r.CheckInToolCalls(ctx, id, row.OccurrenceID)
		if err != nil {
			return nil, err
		}
		proposals, err := r.readQueries.CountCheckInProposalStates(
			ctx,
			sqlcgen.CountCheckInProposalStatesParams{InstanceID: id, SourceKey: "occ:" + row.OccurrenceID},
		)
		if err != nil {
			return nil, err
		}
		result = append(
			result,
			agentinstance.CheckIn{
				ID: row.ID,
				Proposals: agentinstance.CheckInProposalSummary{
					AutoAccepted: proposals.AutoAccepted,
					Waiting:      proposals.Waiting,
				},
				OccurrenceID: row.OccurrenceID,
				ToolsStatus:  row.ToolsStatus,
				ToolCalls:    calls,
				Result:       row.Result,
				Reason:       row.Reason,
				Detail:       row.Detail,
				CreatedAt:    at,
			},
		)
	}
	return result, nil
}

func (r *InstanceRepo) CountQuietCheckIns(ctx context.Context, id string, after *time.Time) (int64, error) {
	timestamp := ""
	if after != nil {
		timestamp = store.FormatTimestamp(*after)
	}
	return r.readQueries.CountInstanceQuietCheckIns(
		ctx,
		sqlcgen.CountInstanceQuietCheckInsParams{InstanceID: id, AfterTime: timestamp},
	)
}
