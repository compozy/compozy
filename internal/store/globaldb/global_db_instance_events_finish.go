package globaldb

import (
	"context"
	"database/sql"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func consumeFinishedInstanceEvents(
	ctx context.Context,
	q *sqlcgen.Queries,
	req heartbeat.FinishRequest,
	completed int,
) error {
	if completed != 1 {
		return nil
	}
	return q.ConsumeInstanceEventBatch(ctx, sqlcgen.ConsumeInstanceEventBatchParams{
		InstanceID: req.InstanceID, OccurrenceID: sql.NullString{String: req.OccurrenceID, Valid: true},
		Now: sql.NullString{String: store.FormatTimestamp(req.Now), Valid: true},
	})
}
