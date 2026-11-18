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
		proposals, err := r.checkInProposals(ctx, id, row.OccurrenceID)
		if err != nil {
			return nil, err
		}
		result = append(
			result,
			agentinstance.CheckIn{
				ID:           row.ID,
				Proposals:    proposals,
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

func (r *InstanceRepo) checkInProposals(
	ctx context.Context,
	instance, occurrence string,
) (agentinstance.CheckInProposalSummary, error) {
	proposals, err := r.readQueries.CountCheckInProposalStates(
		ctx,
		sqlcgen.CountCheckInProposalStatesParams{InstanceID: instance, SourceKey: "occ:" + occurrence},
	)
	if err != nil {
		return agentinstance.CheckInProposalSummary{}, err
	}
	return agentinstance.CheckInProposalSummary{
		AutoAccepted: proposals.AutoAccepted,
		Waiting:      proposals.Waiting,
	}, nil
}
