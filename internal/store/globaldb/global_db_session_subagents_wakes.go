package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func readSubagentWake(ctx context.Context, q *sqlcgen.Queries, id string) (store.SessionSubagentWake, error) {
	row, err := q.GetSubagentWake(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionSubagentWake{}, store.ErrSubagentWakeNotFound
	}
	if err != nil {
		return store.SessionSubagentWake{}, err
	}
	return subagentWakeFromSQL(row)
}

func (g *SessionRepo) OpenOrJoinWake(
	ctx context.Context,
	parentID string,
	ids []string,
	newWakeID string,
) (out store.SessionSubagentWake, err error) {
	encoded, err := subagentIDsJSON(ids)
	if err != nil {
		return out, err
	}
	err = g.withImmediateTransaction(ctx, "open or join subagent wake", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		row, e := q.GetOpenSubagentWake(ctx, parentID)
		now := store.FormatTimestamp(g.now())
		if errors.Is(e, sql.ErrNoRows) {
			if e := q.InsertSubagentWake(
				ctx,
				sqlcgen.InsertSubagentWakeParams{WakeID: newWakeID, ParentID: parentID, Now: now},
			); e != nil {
				return e
			}
			row, e = q.GetSubagentWake(ctx, newWakeID)
		}
		if e != nil {
			return e
		}
		if e := q.InheritSubagentWakeAttempts(ctx, sqlcgen.InheritSubagentWakeAttemptsParams{
			WakeID: row.WakeMessageID, ParentID: parentID, Ids: encoded,
		}); e != nil {
			return e
		}
		if e := q.ClaimSubagentWakeRows(
			ctx,
			sqlcgen.ClaimSubagentWakeRowsParams{
				WakeID:   sql.NullString{String: row.WakeMessageID, Valid: true},
				ParentID: parentID,
				Ids:      encoded,
				Now:      now,
			},
		); e != nil {
			return e
		}
		out, e = readSubagentWake(ctx, q, row.WakeMessageID)
		return e
	})
	return out, err
}

func (g *SessionRepo) GetWake(
	ctx context.Context,
	id string,
) (out store.SessionSubagentWake, items []store.SessionSubagent, err error) {
	err = g.withImmediateTransaction(ctx, "get subagent wake", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		var e error
		out, e = readSubagentWake(ctx, q, id)
		if e != nil {
			return e
		}
		rows, e := q.ListSubagentWakeRows(ctx, sql.NullString{String: id, Valid: true})
		if e != nil {
			return e
		}
		items, e = subagentsFromSQL(rows)
		return e
	})
	return out, items, err
}
func (g *SessionRepo) SetWakeInput(ctx context.Context, id, route, inputID string) error {
	return g.withImmediateTransaction(ctx, "set subagent wake input", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if _, err := readSubagentWake(ctx, q, id); err != nil {
			return err
		}
		_, err := q.SetSubagentWakeInput(
			ctx,
			sqlcgen.SetSubagentWakeInputParams{
				WakeMessageID: id,
				Route:         route,
				InputEntryID:  inputID,
				UpdatedAt:     store.FormatTimestamp(g.now()),
			},
		)
		return err
	})
}
func (g *SessionRepo) MarkWakeSteerRequeued(ctx context.Context, id string) error {
	return g.withImmediateTransaction(ctx, "mark subagent wake steer requeued", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if _, err := readSubagentWake(ctx, q, id); err != nil {
			return err
		}
		_, err := q.MarkSubagentWakeSteerRequeued(
			ctx,
			sqlcgen.MarkSubagentWakeSteerRequeuedParams{WakeMessageID: id, UpdatedAt: store.FormatTimestamp(g.now())},
		)
		return err
	})
}
func (g *SessionRepo) MarkWakeDispatched(ctx context.Context, id string) error {
	return g.withImmediateTransaction(ctx, "dispatch subagent wake", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if _, err := readSubagentWake(ctx, q, id); err != nil {
			return err
		}
		_, err := q.MarkSubagentWakeDispatched(
			ctx,
			sqlcgen.MarkSubagentWakeDispatchedParams{WakeMessageID: id, UpdatedAt: store.FormatTimestamp(g.now())},
		)
		return err
	})
}

func (g *SessionRepo) SettleWake(
	ctx context.Context,
	id string,
	canceled bool,
) (out []store.SessionSubagent, err error) {
	out = []store.SessionSubagent{}
	err = g.withImmediateTransaction(ctx, "settle subagent wake", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if _, e := readSubagentWake(ctx, q, id); e != nil {
			return e
		}
		state, delivery := store.SubagentWakeStateSettled, store.SubagentDeliveryDelivered
		if canceled {
			state, delivery = store.SubagentWakeStateCanceled, store.SubagentDeliveryPending
		}
		now := store.FormatTimestamp(g.now())
		n, e := q.SettleSubagentWake(
			ctx,
			sqlcgen.SettleSubagentWakeParams{State: state, UpdatedAt: now, WakeMessageID: id},
		)
		if e != nil || n == 0 {
			return e
		}
		rows, e := q.SettleSubagentWakeRows(
			ctx,
			sqlcgen.SettleSubagentWakeRowsParams{
				Delivery: delivery,
				Now:      now,
				WakeID:   id,
			},
		)
		if e != nil {
			return e
		}
		out, e = subagentsFromSQL(rows)
		return e
	})
	return out, err
}
func (g *SessionRepo) ListOpenWakes(ctx context.Context) ([]store.SessionSubagentWake, error) {
	if err := g.checkReady(ctx, "list open subagent wakes"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListOpenSubagentWakes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]store.SessionSubagentWake, 0, len(rows))
	for _, row := range rows {
		v, e := subagentWakeFromSQL(row)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

func (g *SessionRepo) RewriteSubagentWakeInput(
	ctx context.Context, wakeID, text string, metadata json.RawMessage,
) error {
	if !json.Valid(metadata) {
		return errors.New("store: valid synthetic wake metadata is required")
	}
	return g.withImmediateTransaction(ctx, "rewrite subagent wake input", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		wake, err := readSubagentWake(ctx, q, wakeID)
		if err != nil {
			return err
		}
		if wake.State != store.SubagentWakeStateOpen {
			return fmt.Errorf("store: subagent wake %q is not open", wakeID)
		}
		affected, err := q.RewriteSubagentWakeInput(ctx, sqlcgen.RewriteSubagentWakeInputParams{
			Text: text, Metadata: string(metadata), Now: store.FormatTimestamp(g.now()),
			InputID: wake.InputEntryID, ParentID: wake.ParentSessionID,
		})
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotQueued, wake.InputEntryID)
		}
		return nil
	})
}

func (g *SessionRepo) FailWake(
	ctx context.Context,
	id string,
) (wake store.SessionSubagentWake, rows []store.SessionSubagent, err error) {
	err = g.withImmediateTransaction(ctx, "fail subagent wake", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		now := store.FormatTimestamp(g.now())
		changed, e := q.FailSubagentWake(ctx, sqlcgen.FailSubagentWakeParams{UpdatedAt: now, WakeMessageID: id})
		if e != nil {
			return e
		}
		wake, e = readSubagentWake(ctx, q, id)
		if e != nil || changed == 0 {
			return e
		}
		delivery := store.SubagentDeliveryPending
		if wake.Attempts >= 3 {
			delivery = store.SubagentDeliveryDisposed
		}
		updated, e := q.SettleSubagentWakeRows(ctx, sqlcgen.SettleSubagentWakeRowsParams{
			Delivery: delivery, Now: now, WakeID: id,
		})
		if e != nil {
			return e
		}
		rows, e = subagentsFromSQL(updated)
		return e
	})
	return wake, rows, err
}
