package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

var _ store.SessionStatusRowsReader = (*SessionRepo)(nil)
var _ store.SessionFailureSummaryReader = (*SessionRepo)(nil)

// SessionStatusRows omits histories, runtime options and authority projections.
func (g *SessionRepo) SessionStatusRows(
	ctx context.Context, scope store.ReadScope,
) (result []store.SessionInfo, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("store: read session status rows: %w", err)
		}
	}()
	if err := g.checkReady(ctx, "session status rows"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	where, args := store.BuildClauses(store.ReadScopeClause("profile_id", scope))
	// dynamic-sql: the owner/attention snapshot follows the caller's profile selection.
	rows, err := g.readDB.QueryContext(ctx, store.AppendWhere(`SELECT id, profile_id, workspace_id,
 pending_permission_count, pending_clarify_count, last_settled_revision, last_seen_revision,
 ended_early_turn_id, failure_seen_at FROM sessions`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("store: query status rows: %w", err)
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var info store.SessionInfo
		var attention store.SessionAttention
		var failureSeen sql.NullString
		if err := rows.Scan(&info.ID, &info.ProfileID, &info.WorkspaceID,
			&attention.PendingPermissionCount, &attention.PendingClarifyCount,
			&attention.LastSettledRevision, &attention.LastSeenRevision,
			&attention.EndedEarlyTurnID, &failureSeen); err != nil {
			return nil, err
		}
		attention.FailureSeenAt, err = parseNullableAttentionTime(failureSeen, "failure seen")
		if err != nil {
			return nil, err
		}
		info.Attention = &attention
		result = append(result, info)
	}
	return result, rows.Err()
}

func (g *SessionRepo) SessionFailureSummary(
	ctx context.Context,
	scope store.ReadScope,
) (result store.SessionFailureSummary, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("store: read session failure summary: %w", err)
		}
	}()
	if err := g.checkReady(ctx, "session failure summary"); err != nil {
		return result, err
	}
	if err := scope.Validate(); err != nil {
		return result, err
	}
	where, args := store.BuildClauses(store.ReadScopeClause("profile_id", scope))
	where = append(where, "trim(COALESCE(failure_kind, '')) <> ''")
	tx, err := g.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, fmt.Errorf("store: begin failure summary: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()
	// dynamic-sql: failure counts and their bounded detail share the caller's profile selection.
	rows, err := tx.QueryContext(
		ctx,
		store.AppendWhere("SELECT trim(failure_kind), COUNT(1) FROM sessions", where)+" GROUP BY 1",
		args...)
	if err != nil {
		return result, fmt.Errorf("store: query failure counts: %w", err)
	}
	result, err = scanSessionFailureCounts(rows)
	if err != nil {
		return result, err
	}
	// dynamic-sql: select only the ten failure details within the same scoped snapshot.
	rows, err = tx.QueryContext(
		ctx,
		store.AppendWhere(
			`SELECT id, agent_name, provider, workspace_id, state, failure_kind,
            failure_summary, crash_bundle_path, updated_at FROM sessions`,
			where,
		)+" ORDER BY updated_at DESC, created_at DESC, id DESC LIMIT 10",
		args...)
	if err != nil {
		return result, fmt.Errorf("store: query recent failures: %w", err)
	}
	for rows.Next() {
		var info store.SessionInfo
		var failure store.SessionFailure
		var updated string
		if scanErr := rows.Scan(
			&info.ID,
			&info.AgentName,
			&info.Provider,
			&info.WorkspaceID,
			&info.State,
			&failure.Kind,
			&failure.Summary,
			&failure.CrashBundlePath,
			&updated,
		); scanErr != nil {
			return result, errors.Join(scanErr, rows.Close())
		}
		info.UpdatedAt, err = store.ParseTimestamp(updated)
		if err != nil {
			return result, errors.Join(err, rows.Close())
		}
		info.Failure = new(failure.Normalize())
		result.Recent = append(result.Recent, info)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func scanSessionFailureCounts(rows *sql.Rows) (result store.SessionFailureSummary, err error) {
	defer func() { err = errors.Join(err, rows.Close()) }()
	result.ByKind = make(map[store.FailureKind]int)
	for rows.Next() {
		var kind store.FailureKind
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return result, err
		}
		result.Total += count
		result.ByKind[kind] = count
	}
	return result, rows.Err()
}
