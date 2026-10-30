package globaldb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionProcessRepo) LoadProcessObservations(ctx context.Context) ([]sessionprocess.Observation, error) {
	if err := g.checkReady(ctx, "load process observations"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListSessionProcessObservations(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: load process observations: %w", err)
	}
	result := make([]sessionprocess.Observation, 0, len(rows))
	for _, row := range rows {
		start, err := time.Parse(time.RFC3339Nano, row.StartedAt)
		if err != nil {
			return nil, fmt.Errorf("store: parse process start: %w", err)
		}
		first, err := time.Parse(time.RFC3339Nano, row.FirstSeen)
		if err != nil {
			return nil, fmt.Errorf("store: parse process observation: %w", err)
		}
		group, err := time.Parse(time.RFC3339Nano, row.GroupObservedAt)
		if err != nil {
			return nil, fmt.Errorf("store: parse process group observation: %w", err)
		}
		result = append(result, sessionprocess.Observation{
			PID: int(row.Pid), StartedAt: start,
			Owner: sessionprocess.Owner{
				SessionID:   row.SessionID,
				WorkspaceID: row.WorkspaceID,
				ProfileID:   row.ProfileID,
				Name:        row.SessionName,
			},
			PGID:            int(row.Pgid),
			FirstSeen:       first,
			GroupObservedAt: group,
			Attribution:     row.Attribution,
			KernelToken:     row.KernelToken,
		})
	}
	return result, nil
}

func (g *SessionProcessRepo) SaveProcessObservations(
	ctx context.Context,
	updates []sessionprocess.Observation,
	deletes []sessionprocess.Identity,
) (err error) {
	if err := g.checkReady(ctx, "save process observations"); err != nil {
		return err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin process observations: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	q := g.queries.WithTx(tx)
	for _, id := range deletes {
		if err := q.DeleteSessionProcessObservation(
			ctx,
			sqlcgen.DeleteSessionProcessObservationParams{
				Pid:       int64(id.PID),
				StartedAt: id.StartedAt.UTC().Format(time.RFC3339Nano),
			},
		); err != nil {
			return err
		}
	}
	for _, o := range updates {
		if o.PID <= 1 || o.StartedAt.IsZero() || o.Owner.SessionID == "" || o.Owner.ProfileID == "" ||
			o.FirstSeen.IsZero() {
			return errors.New("store: incomplete process observation")
		}
		err = q.UpsertSessionProcessObservation(ctx, sqlcgen.UpsertSessionProcessObservationParams{
			Pid:         int64(o.PID),
			StartedAt:   o.StartedAt.UTC().Format(time.RFC3339Nano),
			SessionID:   o.Owner.SessionID,
			WorkspaceID: o.Owner.WorkspaceID,
			ProfileID:   o.Owner.ProfileID,
			SessionName: o.Owner.Name,
			Pgid:        int64(o.PGID),
			FirstSeen: o.FirstSeen.UTC().
				Format(time.RFC3339Nano),
			GroupObservedAt: o.GroupObservedAt.UTC().Format(time.RFC3339Nano),
			Attribution:     o.Attribution,
			KernelToken:     o.KernelToken,
		})
		if err != nil {
			return fmt.Errorf("store: save process observation: %w", err)
		}
	}
	return tx.Commit()
}
