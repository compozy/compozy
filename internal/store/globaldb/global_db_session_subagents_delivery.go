package globaldb

import (
	"context"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) SetPending(ctx context.Context, ids []string) error {
	if err := g.checkReady(ctx, "set subagents pending"); err != nil {
		return err
	}
	encoded, err := subagentIDsJSON(ids)
	if err != nil {
		return err
	}
	return g.withImmediateTransaction(ctx, "set subagents pending", func(exec globalSQLExecutor) error {
		return sqlcgen.New(exec).SetSubagentsPending(ctx,
			sqlcgen.SetSubagentsPendingParams{Ids: encoded, UpdatedAt: store.FormatTimestamp(g.now())})
	})
}

func (g *SessionRepo) Acknowledge(
	ctx context.Context,
	id, turnID string,
) (out store.SessionSubagent, wake *store.SessionSubagentWake, err error) {
	err = g.withImmediateTransaction(ctx, "acknowledge subagent", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		row, e := readSubagent(ctx, q, id)
		if e != nil {
			return e
		}
		if !store.IsSubagentStatusTerminal(row.Status) || row.Delivery == store.SubagentDeliveryDisposed ||
			row.Delivery == store.SubagentDeliveryAcknowledged {
			out = row
			return nil
		}
		if row.WakeMessageID != nil {
			w, e := readSubagentWake(ctx, q, *row.WakeMessageID)
			if e != nil {
				return e
			}
			if w.State == store.SubagentWakeStateOpen {
				wake = &w
			}
		}
		now := store.FormatTimestamp(g.now())
		if e := q.AcknowledgeSubagent(
			ctx,
			sqlcgen.AcknowledgeSubagentParams{ID: id, TurnID: turnID, RemoveWake: subagentBool(wake != nil), Now: now},
		); e != nil {
			return e
		}
		if wake != nil {
			if e := q.CancelEmptySubagentWake(
				ctx,
				sqlcgen.CancelEmptySubagentWakeParams{WakeID: wake.WakeMessageID, UpdatedAt: now},
			); e != nil {
				return e
			}
			w, e := readSubagentWake(ctx, q, wake.WakeMessageID)
			if e != nil {
				return e
			}
			wake = &w
		}
		out, e = readSubagent(ctx, q, id)
		return e
	})
	return out, wake, err
}

func (g *SessionRepo) Dispose(
	ctx context.Context,
	f store.SubagentDisposeFilter,
) (out []store.SessionSubagent, err error) {
	ids, err := subagentIDsJSON(f.IDs)
	if err != nil {
		return nil, err
	}
	err = g.withImmediateTransaction(ctx, "dispose subagents", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		now := store.FormatTimestamp(g.now())
		rows, e := q.DisposeSubagents(
			ctx,
			sqlcgen.DisposeSubagentsParams{ParentID: f.ParentSessionID, TurnID: f.ParentTurnID, Ids: ids, Now: now},
		)
		if e != nil {
			return e
		}
		if e := q.CancelEmptyParentSubagentWakes(
			ctx,
			sqlcgen.CancelEmptyParentSubagentWakesParams{ParentSessionID: f.ParentSessionID, UpdatedAt: now},
		); e != nil {
			return e
		}
		out, e = subagentsFromSQL(rows)
		return e
	})
	return out, err
}
func (g *SessionRepo) MarkFirstPromptAdmitted(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "mark first subagent prompt admitted"); err != nil {
		return err
	}
	n, err := g.queries.MarkSubagentFirstPromptAdmitted(
		ctx,
		sqlcgen.MarkSubagentFirstPromptAdmittedParams{ID: id, UpdatedAt: store.FormatTimestamp(g.now())},
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrSubagentNotFound
	}
	return nil
}
func (g *SessionRepo) DeleteReserved(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "delete reserved subagent"); err != nil {
		return err
	}
	n, err := g.queries.DeleteReservedSubagent(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrSubagentNotFound
	}
	return nil
}
