package globaldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ agentinstance.InstanceStore = (*InstanceRepo)(nil)

func (r *InstanceRepo) BindMain(ctx context.Context, id, sessionID string) (agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "bind instance main"); err != nil {
		return agentinstance.Instance{}, err
	}
	if sessionID == "" {
		return agentinstance.Instance{}, errors.New("agentinstance: main session id required")
	}
	err := store.ExecuteWriteOperation(ctx, r.db, "bind instance main", func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		stamped, err := q.StampInstanceMainSession(ctx, sqlcgen.StampInstanceMainSessionParams{InstanceID: sql.NullString{String: id, Valid: true}, SessionID: sessionID})
		if err != nil {
			return err
		}
		if stamped != 1 {
			return agentinstance.ErrScopeMismatch
		}
		count, err := q.BindAgentInstanceMain(ctx, sqlcgen.BindAgentInstanceMainParams{ID: id, SessionID: sql.NullString{String: sessionID, Valid: true}, UpdatedAt: store.FormatTimestamp(r.now())})
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

func (r *InstanceRepo) DeleteUnstarted(ctx context.Context, id string) error {
	if err := r.checkReady(ctx, "delete unstarted instance"); err != nil {
		return err
	}
	return store.ExecuteWriteOperation(ctx, r.db, "delete unstarted instance", func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		if err := q.DetachFailedInstanceAttempts(ctx, sql.NullString{String: id, Valid: true}); err != nil {
			return err
		}
		_, err := q.DeleteUnstartedAgentInstance(ctx, id)
		return err
	})
}
func (r *InstanceRepo) Pause(ctx context.Context, id, reason string) (agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "pause instance"); err != nil {
		return agentinstance.Instance{}, err
	}
	_, err := r.queries.PauseAgentInstance(ctx, sqlcgen.PauseAgentInstanceParams{ID: id, PauseReason: reason, UpdatedAt: store.FormatTimestamp(r.now())})
	if err != nil {
		return agentinstance.Instance{}, err
	}
	return r.Get(ctx, id)
}
func (r *InstanceRepo) Resume(ctx context.Context, id string, limit int) (agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "resume instance"); err != nil {
		return agentinstance.Instance{}, err
	}
	var result agentinstance.Instance
	err := store.ExecuteWriteOperation(ctx, r.db, "resume instance", func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		row, err := q.GetAgentInstance(ctx, id)
		if err != nil {
			return err
		}
		if row.ArchivedAt.Valid {
			return agentinstance.ErrNotFound
		}
		if row.State == "starting" {
			return agentinstance.ErrStarting
		}
		if row.State == "paused" {
			count, err := q.CountActiveAgentInstances(ctx, row.ProfileID)
			if err != nil {
				return err
			}
			if count >= int64(limit) {
				return agentinstance.ErrLimit
			}
			if _, err := q.ResumeAgentInstance(ctx, sqlcgen.ResumeAgentInstanceParams{ID: id, UpdatedAt: store.FormatTimestamp(r.now())}); err != nil {
				return err
			}
			row, err = q.GetAgentInstance(ctx, id)
			if err != nil {
				return err
			}
		}
		result, err = instanceFromRow(row)
		return err
	})
	return result, err
}
func (r *InstanceRepo) Archive(ctx context.Context, id string) (agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "archive instance"); err != nil {
		return agentinstance.Instance{}, err
	}
	now := store.FormatTimestamp(r.now())
	_, err := r.queries.ArchiveAgentInstance(ctx, sqlcgen.ArchiveAgentInstanceParams{ID: id, ArchivedAt: sql.NullString{String: now, Valid: true}, UpdatedAt: now})
	if err != nil {
		return agentinstance.Instance{}, err
	}
	return r.Get(ctx, id)
}
func (r *InstanceRepo) List(ctx context.Context, query agentinstance.ListQuery) ([]agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "list instances"); err != nil {
		return nil, err
	}
	if query.ProfileID == "" || query.Limit < 1 || query.Limit > 100 {
		return nil, errors.New("instance list requires profile and limit from 1 to 100")
	}
	all := 0
	if query.AllWorkspaces {
		all = 1
	}
	rows, err := r.queries.ListAgentInstances(ctx, sqlcgen.ListAgentInstancesParams{ProfileID: query.ProfileID, WorkspaceID: query.WorkspaceID, AllWorkspaces: all, AfterID: query.AfterID, PageLimit: int64(query.Limit)})
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
