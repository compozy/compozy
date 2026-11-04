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

func (r *InstanceRepo) RotationAdmissionCount(ctx context.Context, sessionID string) (int64, error) {
	return r.readQueries.InstanceRotationAdmissionCount(ctx, sessionID)
}

// PublishRotation checks persisted dependencies while the caller holds admission against monitors and turns.
func (r *InstanceRepo) PublishRotation(ctx context.Context, req agentinstance.RotationPublication) error {
	if req.PredecessorID == "" || req.SuccessorID == "" || req.PredecessorID == req.SuccessorID {
		return agentinstance.ErrRotationChanged
	}
	return store.ExecuteWriteOperation(
		ctx,
		r.db,
		"publish instance rotation",
		func(ctx context.Context, tx *store.WriteTx) error {
			q := sqlcgen.New(tx)
			ready, err := q.InstanceRotationPersistenceReady(ctx, req.PredecessorID)
			if err != nil {
				return err
			}
			if !ready.Valid || !ready.Bool {
				return agentinstance.ErrRotationChanged
			}
			count, err := q.InstanceRotationAdmissionCount(ctx, req.PredecessorID)
			if err != nil {
				return err
			}
			if count != req.AdmissionCount {
				return agentinstance.ErrRotationChanged
			}
			stamped, err := q.StampInstanceMainSession(
				ctx,
				sqlcgen.StampInstanceMainSessionParams{
					InstanceID: sql.NullString{String: req.InstanceID, Valid: true},
					SessionID:  req.SuccessorID,
				},
			)
			if err != nil {
				return err
			}
			if stamped != 1 {
				return agentinstance.ErrScopeMismatch
			}
			updated, err := q.PublishRotatedInstanceMain(
				ctx,
				sqlcgen.PublishRotatedInstanceMainParams{
					InstanceID:    req.InstanceID,
					PredecessorID: sql.NullString{String: req.PredecessorID, Valid: true},
					SuccessorID:   sql.NullString{String: req.SuccessorID, Valid: true},
					UpdatedAt:     store.FormatTimestamp(r.now()),
				},
			)
			if err != nil {
				return err
			}
			if updated != 1 {
				return agentinstance.ErrRotationChanged
			}
			return q.ClearInstanceRotationDue(
				ctx,
				sqlcgen.ClearInstanceRotationDueParams{InstanceID: req.InstanceID, SessionID: req.PredecessorID},
			)
		},
	)
}

func (r *InstanceRepo) RotationPersistenceReady(ctx context.Context, sessionID string) (bool, error) {
	ready, err := r.readQueries.InstanceRotationPersistenceReady(ctx, sessionID)
	return ready.Valid && ready.Bool, err
}

func (r *InstanceRepo) SupersededSessions(ctx context.Context, instanceID string) ([]string, error) {
	return r.readQueries.ListSupersededInstanceSessions(ctx, instanceID)
}

func (r *InstanceRepo) RotationWaitingSince(
	ctx context.Context,
	instanceID, sessionID string,
	now time.Time,
) (time.Time, error) {
	value, err := executeWriteQueryResult(
		ctx,
		r.repoBase,
		"globaldb.BeginInstanceRotationWait",
		func(ctx context.Context, queries *sqlcgen.Queries) (string, error) {
			return queries.BeginInstanceRotationWait(
				ctx,
				sqlcgen.BeginInstanceRotationWaitParams{
					InstanceID:    instanceID,
					MainSessionID: sessionID,
					StartedAt:     store.FormatTimestamp(now),
				},
			)
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, agentinstance.ErrRotationChanged
	}
	if err != nil {
		return time.Time{}, err
	}
	return store.ParseTimestamp(value)
}

func (r *InstanceRepo) ClearRotationWait(ctx context.Context, instanceID, sessionID string) error {
	return r.executeWriteQuery(
		ctx,
		"globaldb.ClearInstanceRotationWait",
		func(ctx context.Context, queries *sqlcgen.Queries) error {
			return queries.ClearInstanceRotationWait(
				ctx,
				sqlcgen.ClearInstanceRotationWaitParams{InstanceID: instanceID, MainSessionID: sessionID},
			)
		},
	)
}

func (r *InstanceRepo) RotationDueSession(ctx context.Context, id string) (string, error) {
	state, err := r.WakeState(ctx, id)
	return state.RotationDueSession, err
}

func (r *InstanceRepo) RotationBlockers(ctx context.Context, sessionID string) ([]string, error) {
	return r.readQueries.InstanceRotationBlockers(ctx, sql.NullString{String: sessionID, Valid: true})
}
