package globaldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type InstanceRepo struct{ *repoBase }

func (g *GlobalDB) Instances() *InstanceRepo {
	return &InstanceRepo{repoBase: newRepoBase(g.db, g.now, &g.closed)}
}

func (r *InstanceRepo) Get(ctx context.Context, id string) (agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "get instance"); err != nil {
		return agentinstance.Instance{}, err
	}
	row, err := r.queries.GetAgentInstance(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return agentinstance.Instance{}, agentinstance.ErrNotFound
	}
	if err != nil {
		return agentinstance.Instance{}, err
	}
	return instanceFromRow(row)
}

// AdmitCreate serializes identity lookup, cap accounting and insertion in one immediate transaction.
func (r *InstanceRepo) AdmitCreate(ctx context.Context, input agentinstance.Instance, limit int) (agentinstance.Instance, bool, error) {
	if err := r.checkReady(ctx, "admit instance"); err != nil {
		return agentinstance.Instance{}, false, err
	}
	var result agentinstance.Instance
	created := false
	err := store.ExecuteWriteOperation(ctx, r.db, "admit instance", func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		existing, err := q.FindLiveAgentInstance(ctx, sqlcgen.FindLiveAgentInstanceParams{ProfileID: input.ProfileID, AgentName: input.AgentName, WorkspaceID: input.WorkspaceID})
		if err == nil {
			result, err = instanceFromRow(existing)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		count, err := q.CountActiveAgentInstances(ctx, input.ProfileID)
		if err != nil {
			return err
		}
		if count >= int64(limit) {
			return agentinstance.ErrLimit
		}
		id, err := store.NewID("ainst")
		if err != nil {
			return err
		}
		now := store.FormatTimestamp(r.now())
		err = q.InsertAgentInstance(ctx, sqlcgen.InsertAgentInstanceParams{ID: id, ProfileID: input.ProfileID, AgentName: input.AgentName, WorkspaceID: input.WorkspaceID, GrantCeiling: input.GrantCeiling, PermissionCeiling: input.PermissionCeiling, Depth: int64(input.Depth), CreatedByKind: input.CreatedByKind, CreatedByID: input.CreatedByID, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			return err
		}
		if err := q.InsertInstanceWakeState(ctx, id); err != nil {
			return err
		}
		row, err := q.GetAgentInstance(ctx, id)
		if err != nil {
			return err
		}
		result, err = instanceFromRow(row)
		created = err == nil
		return err
	})
	return result, created, err
}

func instanceFromRow(row sqlcgen.AgentInstance) (agentinstance.Instance, error) {
	created, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	updated, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return agentinstance.Instance{}, err
	}
	result := agentinstance.Instance{ID: row.ID, ProfileID: row.ProfileID, AgentName: row.AgentName, WorkspaceID: row.WorkspaceID, State: row.State, PauseReason: row.PauseReason, MainSessionID: row.MainSessionID.String, GrantCeiling: row.GrantCeiling, PermissionCeiling: row.PermissionCeiling, ChecksEnabled: row.ChecksEnabled != 0, Depth: int(row.Depth), CreatedByKind: row.CreatedByKind, CreatedByID: row.CreatedByID, CreatedAt: created, UpdatedAt: updated}
	if row.ArchivedAt.Valid {
		archived, err := store.ParseTimestamp(row.ArchivedAt.String)
		if err != nil {
			return agentinstance.Instance{}, err
		}
		result.ArchivedAt = &archived
	}
	return result, nil
}

func (r *InstanceRepo) InstanceMainAttempt(ctx context.Context, instanceID string) (string, error) {
	if err := r.checkReady(ctx, "read instance main attempt"); err != nil {
		return "", err
	}
	id, err := r.queries.GetInstanceMainAttempt(ctx, sql.NullString{String: instanceID, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
