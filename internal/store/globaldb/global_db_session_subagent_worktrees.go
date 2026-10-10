package globaldb

import (
	"context"
	"database/sql"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) AssociateSubagentWorktree(ctx context.Context, row store.SessionSubagent) error {
	return g.withImmediateTransaction(ctx, "associate subagent worktree", func(exec globalSQLExecutor) error {
		n, err := sqlcgen.New(exec).AssociateSubagentWorktree(ctx, sqlcgen.AssociateSubagentWorktreeParams{
			ID:              row.ID,
			WorktreeID:      subagentOptionalString(row.WorktreeState().ID),
			WorktreeName:    subagentOptionalString(row.WorktreeState().Name),
			WorktreeBranch:  subagentOptionalString(row.WorktreeState().Branch),
			WorktreeBaseRef: subagentOptionalString(row.WorktreeState().BaseRef),
			WorktreeBaseSha: subagentOptionalString(row.WorktreeState().BaseSHA),
			WorktreePath:    subagentOptionalString(row.WorktreeState().Path),
		})
		if err == nil && n != 1 {
			return store.ErrSubagentNotFound
		}
		return err
	})
}
func (g *SessionRepo) SetSubagentWorktreeCleanup(ctx context.Context, id, state string) error {
	return g.withImmediateTransaction(ctx, "subagent worktree cleanup", func(exec globalSQLExecutor) error {
		n, err := sqlcgen.New(exec).
			SetSubagentWorktreeCleanup(ctx, sqlcgen.SetSubagentWorktreeCleanupParams{
				ID: id, WorktreeCleanup: subagentOptionalString(state),
			})
		if err == nil && n != 1 {
			return store.ErrSubagentNotFound
		}
		return err
	})
}
func (g *SessionRepo) ListSubagentWorktreeCleanupPending(ctx context.Context) ([]store.SessionSubagent, error) {
	if err := g.checkReady(ctx, "list subagent cleanup"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListSubagentWorktreeCleanupPending(ctx)
	if err != nil {
		return nil, err
	}
	return subagentsFromSQL(rows)
}

func persistSubagentWorktreeFacts(
	ctx context.Context,
	q *sqlcgen.Queries,
	id string,
	facts store.SubagentWorktreeFacts,
) error {
	var at sql.NullString
	if !facts.ObservedAt.IsZero() {
		at = sql.NullString{String: store.FormatTimestamp(facts.ObservedAt), Valid: true}
	}
	return q.SetSubagentWorktreeFacts(
		ctx,
		sqlcgen.SetSubagentWorktreeFactsParams{
			ID:              id,
			GitHeadSha:      subagentOptionalString(facts.HeadSHA),
			GitCommitsAhead: subagentNullInt(facts.CommitsAhead),
			GitDirtyFiles:   subagentNullInt(facts.DirtyFiles),
			GitObservedAt:   at,
			PrStatus:        subagentOptionalString(facts.PRStatus),
			PrUrl:           subagentOptionalString(facts.PRURL),
			PrNumber:        subagentNullInt(facts.PRNumber),
		},
	)
}
func subagentOptionalString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
func subagentNullInt(n *int) sql.NullInt64 {
	if n == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*n), Valid: true}
}
func subagentIntPointer(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	return new(int(n.Int64))
}

func (g *SessionRepo) HasSubagentCommittedAdmission(ctx context.Context, ws, child, id string) (bool, error) {
	if err := g.checkReady(ctx, "subagent admission evidence"); err != nil {
		return false, err
	}
	n, err := g.queries.HasSubagentCommittedAdmission(
		ctx,
		sqlcgen.HasSubagentCommittedAdmissionParams{WorkspaceID: ws, SessionID: child, SubagentID: id},
	)
	return n, err
}
