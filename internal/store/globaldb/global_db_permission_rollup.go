package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func (g *PermissionRepo) PermissionDailyRollups(
	ctx context.Context,
	query store.PermissionRollupQuery,
) (result []store.PermissionDailyRollup, err error) {
	if err := g.checkReady(ctx, "permission daily rollups"); err != nil {
		return nil, err
	}
	if err := query.ReadScope.Validate(); err != nil {
		return nil, err
	}
	if query.Since.IsZero() || query.Until.IsZero() || query.Since.After(query.Until) {
		return nil, fmt.Errorf("store: permission rollup requires an ordered bounded time interval")
	}
	where, args := store.BuildClauses(
		store.ReadScopeClause("s.profile_id", query.ReadScope),
		store.StringClause("s.workspace_id", query.WorkspaceID),
		store.StringClause("p.agent_name", query.AgentName),
		store.TimeClause("p.timestamp", ">=", query.Since),
		store.TimeClause("p.timestamp", "<", query.Until),
	)
	if query.Unattended {
		where = append(where, "p.initiator_kind IN ('automation', 'daemon', 'extension')")
	}
	// dynamic-sql: profile, workspace, agent and initiator filters select the aggregate population.
	sqlQuery := store.AppendWhere(
		`SELECT date(p.timestamp), p.agent_name, p.decision, count(*) FROM permission_log p JOIN sessions s ON s.id=p.session_id`,
		where,
	)
	sqlQuery += " GROUP BY date(p.timestamp), p.agent_name, p.decision ORDER BY date(p.timestamp), p.agent_name, p.decision"
	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query permission rollups: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "permission rollups") }()
	result = make([]store.PermissionDailyRollup, 0)
	for rows.Next() {
		var row store.PermissionDailyRollup
		if err := rows.Scan(&row.Day, &row.AgentName, &row.Decision, &row.Count); err != nil {
			return nil, fmt.Errorf("store: scan permission rollup: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate permission rollups: %w", err)
	}
	return result, nil
}

// PermissionEntries returns the newest audit rows in the query's profile reach.
func (g *PermissionRepo) PermissionEntries(
	ctx context.Context,
	query store.PermissionEntriesQuery,
) (result []store.PermissionLogEntry, err error) {
	if err := g.checkReady(ctx, "permission entries"); err != nil {
		return nil, err
	}
	if err := query.ReadScope.Validate(); err != nil {
		return nil, err
	}
	if query.Since.IsZero() || query.Until.IsZero() || query.Since.After(query.Until) {
		return nil, fmt.Errorf("store: permission entries require an ordered bounded time interval")
	}
	where, args := store.BuildClauses(
		store.ReadScopeClause("s.profile_id", query.ReadScope),
		store.StringClause("s.workspace_id", query.WorkspaceID),
		store.StringClause("p.agent_name", query.AgentName),
		store.TimeClause("p.timestamp", ">=", query.Since),
		store.TimeClause("p.timestamp", "<", query.Until),
	)
	if query.Unattended {
		where = append(where, "p.initiator_kind IN ('automation', 'daemon', 'extension')")
	}
	// dynamic-sql: profile, workspace, agent and initiator filters select the listed population.
	sqlQuery := store.AppendWhere(
		`SELECT p.id, p.session_id, p.agent_name, p.action, p.resource, p.decision, p.policy_used, p.timestamp, p.initiator_kind, p.initiator_id FROM permission_log p JOIN sessions s ON s.id=p.session_id`,
		where,
	)
	sqlQuery += " ORDER BY p.timestamp DESC, p.id DESC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)
	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query permission entries: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "permission entries") }()
	result = make([]store.PermissionLogEntry, 0)
	for rows.Next() {
		entry, scanErr := scanPermissionLog(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate permission entries: %w", err)
	}
	return result, nil
}
