package globaldb

import (
	"context"
	"database/sql"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func (g *LoopRepo) queryRequestRows(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	state string,
	cursor *requestCursor,
	limit int,
) (*sql.Rows, error) {
	if state == looppkg.RequestStatePending {
		return g.queryPendingRequestRows(ctx, workspaceID, runID, cursor, limit)
	}
	return g.queryResolvedRequestRows(ctx, workspaceID, runID, cursor, limit)
}

func (g *LoopRepo) queryPendingRequestRows(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	cursor *requestCursor,
	limit int,
) (*sql.Rows, error) {
	if runID != "" && cursor != nil {
		return g.db.QueryContext(ctx, `SELECT `+requestSelectColumns+`
			FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
			WHERE request.workspace_id = ? AND request.loop_run_id = ? AND request.state = 'pending'
			AND (request.expires_at IS NULL, COALESCE(request.expires_at, ''), request.opened_at, request.rowid)
				> (?, ?, ?, ?)
			ORDER BY request.expires_at IS NULL ASC, request.expires_at ASC, request.opened_at ASC,
				request.rowid ASC LIMIT ?`, workspaceID, runID, cursor.NullExpiry,
			nullableCursorTime(*cursor), cursor.Secondary, cursor.RowID, limit)
	}
	if runID != "" {
		return g.db.QueryContext(ctx, `SELECT `+requestSelectColumns+`
			FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
			WHERE request.workspace_id = ? AND request.loop_run_id = ? AND request.state = 'pending'
			ORDER BY request.expires_at IS NULL ASC, request.expires_at ASC, request.opened_at ASC,
				request.rowid ASC LIMIT ?`, workspaceID, runID, limit)
	}
	if cursor != nil {
		return g.db.QueryContext(ctx, `SELECT `+requestSelectColumns+`
			FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
			WHERE request.workspace_id = ? AND request.state = 'pending'
			AND (request.expires_at IS NULL, COALESCE(request.expires_at, ''), request.opened_at, request.rowid)
				> (?, ?, ?, ?)
			ORDER BY request.expires_at IS NULL ASC, request.expires_at ASC, request.opened_at ASC,
				request.rowid ASC LIMIT ?`, workspaceID, cursor.NullExpiry,
			nullableCursorTime(*cursor), cursor.Secondary, cursor.RowID, limit)
	}
	return g.db.QueryContext(ctx, `SELECT `+requestSelectColumns+`
		FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
		WHERE request.workspace_id = ? AND request.state = 'pending'
		ORDER BY request.expires_at IS NULL ASC, request.expires_at ASC, request.opened_at ASC,
			request.rowid ASC LIMIT ?`, workspaceID, limit)
}

func (g *LoopRepo) queryResolvedRequestRows(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	cursor *requestCursor,
	limit int,
) (*sql.Rows, error) {
	if runID != "" && cursor != nil {
		return g.db.QueryContext(ctx, `SELECT `+requestSelectColumns+`
			FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
			WHERE request.workspace_id = ? AND request.loop_run_id = ?
				AND request.state IN ('answered','expired','canceled')
				AND (request.resolved_at, request.rowid) < (?, ?)
			ORDER BY request.resolved_at DESC, request.rowid DESC LIMIT ?`,
			workspaceID, runID, cursor.Primary, cursor.RowID, limit)
	}
	if runID != "" {
		return g.db.QueryContext(ctx, `SELECT `+requestSelectColumns+`
			FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
			WHERE request.workspace_id = ? AND request.loop_run_id = ?
				AND request.state IN ('answered','expired','canceled')
			ORDER BY request.resolved_at DESC, request.rowid DESC LIMIT ?`, workspaceID, runID, limit)
	}
	if cursor != nil {
		return g.db.QueryContext(ctx, `SELECT `+requestSelectColumns+`
			FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
			WHERE request.workspace_id = ? AND request.state IN ('answered','expired','canceled')
				AND (request.resolved_at, request.rowid) < (?, ?)
			ORDER BY request.resolved_at DESC, request.rowid DESC LIMIT ?`,
			workspaceID, cursor.Primary, cursor.RowID, limit)
	}
	return g.db.QueryContext(ctx, `SELECT `+requestSelectColumns+`
		FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
		WHERE request.workspace_id = ? AND request.state IN ('answered','expired','canceled')
		ORDER BY request.resolved_at DESC, request.rowid DESC LIMIT ?`, workspaceID, limit)
}
