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
	rows, err := r.queries.ListInstanceCheckIns(
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
		result = append(
			result,
			agentinstance.CheckIn{ID: row.ID, Result: row.Result, Reason: row.Reason, CreatedAt: at},
		)
	}
	return result, nil
}

func (r *InstanceRepo) CountQuietCheckIns(ctx context.Context, id string, after *time.Time) (int64, error) {
	timestamp := ""
	if after != nil {
		timestamp = store.FormatTimestamp(*after)
	}
	return r.queries.CountInstanceQuietCheckIns(
		ctx,
		sqlcgen.CountInstanceQuietCheckInsParams{InstanceID: id, AfterTime: timestamp},
	)
}
