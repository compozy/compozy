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

var _ agentinstance.SeenStore = (*InstanceRepo)(nil)

func (r *InstanceRepo) LastSeen(ctx context.Context, instance, operator string) (*time.Time, error) {
	if err := r.checkReady(ctx, "read instance seen"); err != nil {
		return nil, err
	}
	if instance == "" || operator == "" {
		return nil, errors.New("instance and operator are required")
	}
	raw, err := r.readQueries.GetAgentInstanceSeen(
		ctx,
		sqlcgen.GetAgentInstanceSeenParams{InstanceID: instance, OperatorID: operator},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, err
	}
	return &at, nil
}

// MarkSeen acknowledges failures and eligible stopped requests without consuming provider reports.
func (r *InstanceRepo) MarkSeen(
	ctx context.Context,
	instance, operator string,
	at time.Time,
) (agentinstance.SeenResult, error) {
	if err := r.checkReady(ctx, "mark instance seen"); err != nil {
		return agentinstance.SeenResult{}, err
	}
	if instance == "" || operator == "" || at.IsZero() {
		return agentinstance.SeenResult{}, errors.New("instance operator and timestamp are required")
	}
	var raw string
	var result agentinstance.SeenResult
	err := store.ExecuteWriteOperation(
		ctx,
		r.db,
		"mark instance seen",
		func(ctx context.Context, tx *store.WriteTx) error {
			result.Interactions = nil
			q := sqlcgen.New(tx)
			var err error
			raw, err = q.MarkAgentInstanceSeen(ctx, sqlcgen.MarkAgentInstanceSeenParams{
				InstanceID: instance, OperatorID: operator, LastSeenAt: store.FormatTimestamp(at),
			})
			if err != nil {
				return err
			}
			if operator == agentinstance.LocalOperator {
				seenAt, err := time.Parse(time.RFC3339Nano, raw)
				if err != nil {
					return err
				}
				ids, err := q.ListSeenStoppedInstanceInteractions(
					ctx,
					sqlcgen.ListSeenStoppedInstanceInteractionsParams{
						InstanceID: instance, SeenAt: raw,
					},
				)
				if err != nil {
					return err
				}
				for _, id := range ids {
					commit, err := transitionPendingInteraction(ctx, tx, store.PendingInteractionTransition{
						InteractionID: id, Status: store.PendingInteractionStatusCanceled,
						At: seenAt, Resolution: "Acknowledged by operator after session stopped", ResolvedBy: operator,
					})
					if err != nil {
						return err
					}
					result.Interactions = append(result.Interactions, commit)
				}
			}
			return q.ResetInstanceUnattended(ctx, sqlcgen.ResetInstanceUnattendedParams{InstanceID: instance, Now: raw})
		},
	)
	if err != nil {
		return agentinstance.SeenResult{}, err
	}
	result.LastSeenAt, err = time.Parse(time.RFC3339Nano, raw)
	return result, err
}
