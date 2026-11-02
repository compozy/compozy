package globaldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (r *InstanceRepo) Adopt(
	ctx context.Context,
	input agentinstance.Instance,
	sessionID string,
	limit int,
) (agentinstance.Instance, error) {
	if err := r.checkReady(ctx, "adopt instance"); err != nil {
		return agentinstance.Instance{}, err
	}
	var result agentinstance.Instance
	err := store.ExecuteWriteOperation(ctx, r.db, "adopt instance", func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		if err := validateInstanceAdoption(ctx, q, &input, limit); err != nil {
			return err
		}
		id, err := store.NewID("ainst")
		if err != nil {
			return err
		}
		now := store.FormatTimestamp(r.now())
		err = q.InsertAgentInstance(
			ctx,
			sqlcgen.InsertAgentInstanceParams{
				ID:                id,
				ProfileID:         input.ProfileID,
				AgentName:         input.AgentName,
				WorkspaceID:       input.WorkspaceID,
				GrantCeiling:      input.GrantCeiling,
				PermissionCeiling: input.PermissionCeiling,
				Depth:             int64(input.Depth),
				CreatedByKind:     input.CreatedByKind,
				CreatedByID:       input.CreatedByID,
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		)
		if err != nil {
			return err
		}
		if err := q.InsertInstanceWakeState(ctx, id); err != nil {
			return err
		}
		stamped, err := q.StampInstanceMainSession(
			ctx,
			sqlcgen.StampInstanceMainSessionParams{
				InstanceID: sql.NullString{String: id, Valid: true},
				SessionID:  sessionID,
			},
		)
		if err != nil {
			return err
		}
		if stamped != 1 {
			return agentinstance.ErrScopeMismatch
		}
		bound, err := q.BindAgentInstanceMain(
			ctx,
			sqlcgen.BindAgentInstanceMainParams{
				ID:        id,
				SessionID: sql.NullString{String: sessionID, Valid: true},
				UpdatedAt: now,
			},
		)
		if err != nil {
			return err
		}
		if bound != 1 {
			return agentinstance.ErrStarting
		}
		row, err := q.GetAgentInstance(ctx, id)
		if err != nil {
			return err
		}
		result, err = instanceFromRow(row)
		return err
	})
	return result, err
}

func validateInstanceAdoption(ctx context.Context, q *sqlcgen.Queries, input *agentinstance.Instance, limit int) error {
	_, err := q.FindLiveAgentInstance(
		ctx,
		sqlcgen.FindLiveAgentInstanceParams{
			ProfileID:   input.ProfileID,
			AgentName:   input.AgentName,
			WorkspaceID: input.WorkspaceID,
		},
	)
	if err == nil {
		return agentinstance.ErrExists
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
	return nil
}
