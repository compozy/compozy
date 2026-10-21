package globaldb

import (
	"context"
	"database/sql"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
)

func loopRequestWhere(workspaceID looppkg.WorkspaceID, query looppkg.RequestQuery) ([]string, []any) {
	where, args := store.BuildClauses(store.ReadScopeClause("run.profile_id", query.ReadScope), store.StringClause("request.loop_run_id", string(query.RunID)))
	return appendLoopWorkspaceFilter(where, args, "request.workspace_id", string(workspaceID), query.AllWorkspaces, query.WorkspaceIDs)
}

func (g *LoopRepo) queryRequestRows(ctx context.Context, workspaceID looppkg.WorkspaceID, query looppkg.RequestQuery, state string, cursor *requestCursor, limit int) (*sql.Rows, error) {
	where, args := loopRequestWhere(workspaceID, query)
	order := "request.resolved_at DESC, request.rowid DESC"
	if state == looppkg.RequestStatePending {
		where = append(where, "request.state = 'pending'")
		order = "request.expires_at IS NULL ASC, request.expires_at ASC, request.opened_at ASC, request.rowid ASC"
		if cursor != nil {
			where = append(where, "(request.expires_at IS NULL, COALESCE(request.expires_at, ''), request.opened_at, request.rowid) > (?, ?, ?, ?)")
			args = append(args, cursor.NullExpiry, nullableCursorTime(*cursor), cursor.Secondary, cursor.RowID)
		}
	} else {
		where = append(where, "request.state IN ('answered','expired','canceled')")
		if cursor != nil {
			where = append(where, "(request.resolved_at, request.rowid) < (?, ?)")
			args = append(args, cursor.Primary, cursor.RowID)
		}
	}
	// dynamic-sql: optional breadth, profile, run and cursor predicates share the same count/page population.
	statement := store.AppendWhere("SELECT "+requestSelectColumns+" FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id", where) + " ORDER BY " + order + " LIMIT ?"
	return g.db.QueryContext(ctx, statement, append(args, limit)...)
}
