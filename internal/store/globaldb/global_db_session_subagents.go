package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.SubagentStore = (*SessionRepo)(nil)

func (g *SessionRepo) ReserveSubagent(
	ctx context.Context,
	row store.SessionSubagent,
) (out store.SessionSubagent, created bool, err error) {
	if row.CreatedAt.IsZero() {
		row.CreatedAt = g.now()
	}
	if row.UpdatedAt.IsZero() {
		row.UpdatedAt = row.CreatedAt
	}
	if row.Status == "" {
		row.Status = store.SubagentStatusQueued
	}
	if row.WorkState == "" {
		row.WorkState = store.SubagentWorkStateWorking
	}
	if row.Delivery == "" {
		row.Delivery = store.SubagentDeliveryNone
	}
	if row.Role == "" {
		row.Role = "general"
	}
	err = g.withImmediateTransaction(ctx, "reserve subagent", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		n, e := q.ReserveSubagent(ctx, subagentInsertParams(row))
		if e != nil {
			return e
		}
		existing, e := q.GetSubagentByKey(
			ctx,
			sqlcgen.GetSubagentByKeyParams{ParentSessionID: row.ParentSessionID, IdempotencyKey: row.IdempotencyKey},
		)
		if e != nil {
			return e
		}
		if existing.RequestFingerprint != row.RequestFingerprint {
			return store.ErrSubagentIdempotencyConflict
		}
		out, e = subagentFromSQL(&existing)
		created = n == 1
		return e
	})
	return out, created, err
}
func readSubagent(ctx context.Context, q *sqlcgen.Queries, id string) (store.SessionSubagent, error) {
	row, err := q.GetSubagent(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionSubagent{}, store.ErrSubagentNotFound
	}
	if err != nil {
		return store.SessionSubagent{}, err
	}
	return subagentFromSQL(&row)
}
func (g *SessionRepo) GetSubagent(ctx context.Context, workspaceID, id string) (store.SessionSubagent, error) {
	if err := g.checkReady(ctx, "get subagent"); err != nil {
		return store.SessionSubagent{}, err
	}
	row, err := readSubagent(ctx, g.queries, id)
	if err != nil {
		return row, err
	}
	if row.WorkspaceID != workspaceID {
		return store.SessionSubagent{}, store.ErrSubagentNotFound
	}
	return row, nil
}
func (g *SessionRepo) GetSubagentByChild(ctx context.Context, id string) (store.SessionSubagent, error) {
	if err := g.checkReady(ctx, "get subagent by child"); err != nil {
		return store.SessionSubagent{}, err
	}
	row, err := g.queries.GetSubagentByChild(ctx, sql.NullString{String: id, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionSubagent{}, store.ErrSubagentNotFound
	}
	if err != nil {
		return store.SessionSubagent{}, err
	}
	return subagentFromSQL(&row)
}

func (g *SessionRepo) LinkChild(
	ctx context.Context,
	id, childID string,
	at time.Time,
) (out store.SessionSubagent, err error) {
	err = g.withImmediateTransaction(ctx, "link subagent child", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		_, e := q.LinkSubagentChild(
			ctx,
			sqlcgen.LinkSubagentChildParams{
				ChildID: sql.NullString{String: childID, Valid: true},
				Now:     sql.NullString{String: store.FormatTimestamp(at), Valid: true},
				ID:      id,
			},
		)
		if e != nil {
			return e
		}
		out, e = readSubagent(ctx, q, id)
		return e
	})
	return out, err
}
func (g *SessionRepo) UpdateProgress(ctx context.Context, id, progress string, at time.Time) error {
	if err := g.checkReady(ctx, "update subagent progress"); err != nil {
		return err
	}
	_, err := g.queries.UpdateSubagentProgress(
		ctx,
		sqlcgen.UpdateSubagentProgressParams{Progress: progress, UpdatedAt: store.FormatTimestamp(at), ID: id},
	)
	return err
}

func (g *SessionRepo) FinalizeSubagent(
	ctx context.Context,
	in store.SubagentFinalize,
) (out store.SessionSubagent, changed bool, err error) {
	if !store.IsSubagentStatusTerminal(in.Status) {
		return out, false, fmt.Errorf("store: nonterminal finalization status %q", in.Status)
	}
	if in.SettledAt.IsZero() {
		in.SettledAt = g.now()
	}
	if in.WorkState == "" {
		in.WorkState = store.SubagentWorkStateResultAvailable
	}
	err = g.withImmediateTransaction(ctx, "finalize subagent", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		n, e := q.FinalizeSubagent(
			ctx,
			sqlcgen.FinalizeSubagentParams{
				ID:              in.ID,
				Status:          in.Status,
				WorkState:       in.WorkState,
				Result:          subagentNullString(in.Result),
				Error:           subagentNullString(in.Error),
				ResultTruncated: subagentBool(in.ResultTruncated),
				Now:             sql.NullString{String: store.FormatTimestamp(in.SettledAt), Valid: true},
			},
		)
		if e != nil {
			return e
		}
		out, e = readSubagent(ctx, q, in.ID)
		changed = n == 1
		return e
	})
	return out, changed, err
}
func (g *SessionRepo) UpgradeWakePolicy(ctx context.Context, id string) (out store.SessionSubagent, err error) {
	err = g.withImmediateTransaction(ctx, "upgrade subagent wake policy", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if e := q.UpgradeSubagentWakePolicy(
			ctx,
			sqlcgen.UpgradeSubagentWakePolicyParams{UpdatedAt: store.FormatTimestamp(g.now()), ID: id},
		); e != nil {
			return e
		}
		var e error
		out, e = readSubagent(ctx, q, id)
		return e
	})
	return out, err
}

func (g *SessionRepo) UpdateSubagentState(
	ctx context.Context, id, status, workState string, at time.Time,
) (out store.SessionSubagent, changed bool, err error) {
	err = g.withImmediateTransaction(ctx, "update subagent state", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		current, readErr := readSubagent(ctx, q, id)
		if readErr != nil {
			return readErr
		}
		out = current
		if store.IsSubagentStatusTerminal(current.Status) {
			return nil
		}
		switch status {
		case store.SubagentStatusQueued, store.SubagentStatusRunning, store.SubagentStatusWaiting:
		default:
			return fmt.Errorf("store: invalid nonterminal subagent status %q", status)
		}
		switch workState {
		case store.SubagentWorkStateWorking,
			store.SubagentWorkStateWaitingForChildren,
			store.SubagentWorkStateResultAvailable:
		default:
			return fmt.Errorf("store: invalid subagent work state %q", workState)
		}
		if at.IsZero() {
			at = g.now()
		}
		affected, updateErr := q.UpdateSubagentState(ctx, sqlcgen.UpdateSubagentStateParams{
			ID: id, Status: status, WorkState: workState, Now: store.FormatTimestamp(at),
		})
		if updateErr != nil {
			return updateErr
		}
		if affected == 0 {
			return nil
		}
		changed = true
		out, readErr = readSubagent(ctx, q, id)
		return readErr
	})
	return out, changed, err
}

func (g *SessionRepo) UpdateNativeSubagentTitle(ctx context.Context, id, title string, at time.Time) (out store.SessionSubagent, changed bool, err error) {
	err = g.withImmediateTransaction(ctx, "update native subagent title", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		n, e := q.UpdateNativeSubagentTitle(ctx, sqlcgen.UpdateNativeSubagentTitleParams{ID: id, Title: title, UpdatedAt: store.FormatTimestamp(at)})
		if e != nil {
			return e
		}
		out, e = readSubagent(ctx, q, id)
		changed = n == 1
		return e
	})
	return out, changed, err
}
