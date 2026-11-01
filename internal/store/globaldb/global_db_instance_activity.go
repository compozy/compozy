package globaldb

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

var _ agentinstance.ActivityReader = (*InstanceRepo)(nil)

func (r *InstanceRepo) ReadActivity(ctx context.Context, query agentinstance.ActivityQuery) (result agentinstance.Activity, err error) {
	result = agentinstance.Activity{ReportsByDay: []agentinstance.ActivityDay{}, CostByProject: []agentinstance.ActivityCost{}, BusyByHour: []agentinstance.ActivityHour{}}
	if err := r.checkReady(ctx, "instance activity"); err != nil {
		return result, err
	}
	if query.ProfileID == "" || query.AgentName == "" || query.Since.IsZero() {
		return result, errors.New("activity requires profile, agent and time window")
	}
	clauses := []store.Clause{store.StringClause("profile_id", query.ProfileID), store.StringClause("agent_name", query.AgentName)}
	// dynamic-sql: the shared scope clauses apply to daily usage and instance report projections.
	statement, args := instanceActivitySQL(query.WorkspaceID, `SELECT COALESCE(SUM(CASE WHEN cost_currency = 'USD' THEN total_cost ELSE 0 END), 0), COALESCE(SUM(total_tokens), 0) FROM token_usage_daily`, "", append(clauses, store.StringCompareClause("day", ">=", store.LocalDay(query.Since)))...)
	cost := agentinstance.ActivityCost{WorkspaceID: query.WorkspaceID}
	if err := r.db.QueryRowContext(ctx, statement, args...).Scan(&cost.USDEstimate, &cost.Tokens); err != nil {
		return result, err
	}
	if cost.Tokens != 0 || cost.USDEstimate != 0 {
		result.CostByProject = append(result.CostByProject, cost)
	}
	source := `(SELECT r.created_at, r.kind, i.profile_id, i.workspace_id, i.agent_name
        FROM agent_instance_reports r JOIN agent_instances i ON i.id = r.instance_id
        UNION ALL
        SELECT e.created_at, 'quiet', i.profile_id, i.workspace_id, i.agent_name
        FROM agent_heartbeat_wake_events e JOIN agent_instances i ON i.id = e.instance_id
        WHERE e.result = 'quiet')`
	statement, args = instanceActivitySQL(
		query.WorkspaceID,
		`SELECT date(created_at, 'localtime'), SUM(CASE WHEN kind = 'proposal' THEN 1 ELSE 0 END), SUM(CASE WHEN kind NOT IN ('proposal', 'quiet') THEN 1 ELSE 0 END), SUM(CASE WHEN kind = 'quiet' THEN 1 ELSE 0 END) FROM `+source,
		` GROUP BY 1 ORDER BY 1`,
		append(clauses, store.TimeClause("created_at", ">=", query.Since))...)
	rows, err := r.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return result, err
	}
	defer func() { err = joinRowsCloseError(rows, err, "instance activity reports") }()
	for rows.Next() {
		var day agentinstance.ActivityDay
		if err := rows.Scan(&day.Day, &day.Told, &day.NeededYou, &day.Quiet); err != nil {
			return result, err
		}
		result.ReportsByDay = append(result.ReportsByDay, day)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	statement, args = instanceActivitySQL(query.WorkspaceID, `SELECT CAST(strftime('%w', timestamp, 'localtime') AS INTEGER), CAST(strftime('%H', timestamp, 'localtime') AS INTEGER), COUNT(*) FROM event_summaries`, ` GROUP BY 1, 2 ORDER BY 1, 2`, append(clauses, store.TimeClause("timestamp", ">=", query.Since))...)
	hours, err := r.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return result, err
	}
	defer func() { err = joinRowsCloseError(hours, err, "instance activity hours") }()
	for hours.Next() {
		var hour agentinstance.ActivityHour
		if err := hours.Scan(&hour.Weekday, &hour.Hour, &hour.Events); err != nil {
			return result, err
		}
		result.BusyByHour = append(result.BusyByHour, hour)
	}
	return result, hours.Err()
}

func instanceActivitySQL(workspace, prefix, suffix string, clauses ...store.Clause) (string, []any) {
	statement, args := store.BuildWhereQuery(prefix, "", clauses...)
	return statement + " AND workspace_id = ?" + suffix, append(args, workspace)
}
