package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (r *InstanceRepo) SetChecks(ctx context.Context, id string, enabled bool, cadence time.Duration) (agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "set instance checks"); err != nil {
		return agentinstance.Instance{}, err
	}
	if cadence <= 0 {
		return agentinstance.Instance{}, errors.New("check cadence must be positive")
	}
	value := int64(0)
	if enabled {
		value = 1
	}
	now := r.now()
	err := store.ExecuteWriteOperation(ctx, r.db, "set instance checks", func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		row, err := q.GetAgentInstance(ctx, id)
		if err != nil {
			return err
		}
		if row.ArchivedAt.Valid {
			return agentinstance.ErrNotFound
		}
		changed, err := q.SetInstanceChecks(ctx, sqlcgen.SetInstanceChecksParams{InstanceID: id, Enabled: value, Now: store.FormatTimestamp(now)})
		if err != nil {
			return err
		}
		if enabled && changed == 1 {
			return q.ScheduleInstanceChecks(ctx, sqlcgen.ScheduleInstanceChecksParams{InstanceID: id, NextWakeAt: store.FormatTimestamp(now.Add(cadence))})
		}
		return nil
	})
	if err != nil {
		return agentinstance.Instance{}, err
	}
	return r.Get(ctx, id)
}

func (r *InstanceRepo) Occurrence(ctx context.Context, instanceID, occurrenceID string) (agentinstance.Occurrence, error) {
	if err := r.checkReady(ctx, "read instance occurrence"); err != nil {
		return agentinstance.Occurrence{}, err
	}
	row, err := r.queries.GetInstanceOccurrence(ctx, sqlcgen.GetInstanceOccurrenceParams{InstanceID: instanceID, OccurrenceID: occurrenceID})
	if errors.Is(err, sql.ErrNoRows) {
		return agentinstance.Occurrence{}, agentinstance.ErrNotFound
	}
	if err != nil {
		return agentinstance.Occurrence{}, err
	}
	return agentinstance.Occurrence{State: row.State, Result: row.Result, Reason: row.Reason, ReportID: row.ReportID}, nil
}
