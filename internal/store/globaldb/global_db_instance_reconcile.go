package globaldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (r *InstanceRepo) PublishMissingMain(ctx context.Context, id, sessionID string) (agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "publish missing instance main"); err != nil {
		return agentinstance.Instance{}, err
	}
	if sessionID == "" {
		return agentinstance.Instance{}, errors.New("agentinstance: session id required")
	}
	err := store.ExecuteWriteOperation(ctx, r.db, "publish missing instance main", func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		current, err := q.GetAgentInstance(ctx, id)
		if err != nil {
			return err
		}
		if current.ArchivedAt.Valid {
			return agentinstance.ErrNotFound
		}
		if current.MainSessionID.Valid {
			return nil
		}
		stamped, err := q.StampInstanceMainSession(ctx, sqlcgen.StampInstanceMainSessionParams{InstanceID: sql.NullString{String: id, Valid: true}, SessionID: sessionID})
		if err != nil {
			return err
		}
		if stamped != 1 {
			return agentinstance.ErrScopeMismatch
		}
		count, err := q.PublishMissingInstanceMain(ctx, sqlcgen.PublishMissingInstanceMainParams{ID: id, SessionID: sql.NullString{String: sessionID, Valid: true}, UpdatedAt: store.FormatTimestamp(r.now())})
		if err != nil {
			return err
		}
		if count != 1 {
			return agentinstance.ErrStarting
		}
		return nil
	})
	if err != nil {
		return agentinstance.Instance{}, err
	}
	return r.Get(ctx, id)
}

func (r *InstanceRepo) ReconciliationPage(ctx context.Context, after string, limit int) ([]agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "list instance reconciliation page"); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, errors.New("agentinstance: reconciliation page limit must be from 1 to 100")
	}
	rows, err := r.queries.ListInstancesForReconciliation(ctx, sqlcgen.ListInstancesForReconciliationParams{AfterID: after, PageLimit: int64(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]agentinstance.Instance, 0, len(rows))
	for _, row := range rows {
		instance, err := instanceFromRow(row)
		if err != nil {
			return nil, err
		}
		result = append(result, instance)
	}
	return result, nil
}
