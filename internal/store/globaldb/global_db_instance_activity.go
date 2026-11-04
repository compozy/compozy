package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

var _ agentinstance.ActivityReader = (*InstanceRepo)(nil)

func (r *InstanceRepo) ReadActivity(
	ctx context.Context,
	query agentinstance.ActivityQuery,
) (agentinstance.Activity, error) {
	results, err := r.ReadActivities(ctx, []agentinstance.ActivityQuery{query})
	if err != nil {
		return emptyInstanceActivity(), err
	}
	return results[0], nil
}

func emptyInstanceActivity() agentinstance.Activity {
	return agentinstance.Activity{
		ReportsByDay:  []agentinstance.ActivityDay{},
		CostByProject: []agentinstance.ActivityCost{},
		BusyByHour:    []agentinstance.ActivityHour{},
	}
}

func (r *InstanceRepo) ReadActivities(
	ctx context.Context,
	queries []agentinstance.ActivityQuery,
) ([]agentinstance.Activity, error) {
	if err := r.checkReady(ctx, "instance activity"); err != nil {
		return nil, err
	}
	results := make([]agentinstance.Activity, len(queries))
	for i := range results {
		results[i] = emptyInstanceActivity()
	}
	cte, args, err := instanceActivityScopes(queries)
	if err != nil {
		return nil, err
	}
	if cte == "" {
		return results, nil
	}
	for _, projection := range []struct {
		statement string
		scan      func(*sql.Rows) error
	}{
		{instanceActivityCosts, func(rows *sql.Rows) error {
			var index int
			var cost agentinstance.ActivityCost
			if err := rows.Scan(&index, &cost.WorkspaceID, &cost.USDEstimate, &cost.Tokens); err != nil {
				return err
			}
			if cost.Tokens != 0 || cost.USDEstimate != 0 {
				results[index].CostByProject = append(results[index].CostByProject, cost)
			}
			return nil
		}},
		{instanceActivityReports, func(rows *sql.Rows) error {
			var index int
			var day agentinstance.ActivityDay
			if err := rows.Scan(&index, &day.Day, &day.Told, &day.NeededYou, &day.Quiet); err != nil {
				return err
			}
			results[index].ReportsByDay = append(results[index].ReportsByDay, day)
			return nil
		}},
		{instanceActivityHours, func(rows *sql.Rows) error {
			var index int
			var hour agentinstance.ActivityHour
			if err := rows.Scan(&index, &hour.Weekday, &hour.Hour, &hour.Events); err != nil {
				return err
			}
			results[index].BusyByHour = append(results[index].BusyByHour, hour)
			return nil
		}},
	} {
		if err := r.readInstanceActivity(ctx, cte+projection.statement, args, projection.scan); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (r *InstanceRepo) readInstanceActivity(
	ctx context.Context,
	statement string,
	args []any,
	scan func(*sql.Rows) error,
) (err error) {
	rows, err := r.readDB.QueryContext(ctx, statement, args...)
	if err != nil {
		return err
	}
	defer func() { err = joinRowsCloseError(rows, err, "instance activity") }()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// dynamic-sql: this owner expands only bound scope tuples, shared by the three activity projections.
func instanceActivityScopes(queries []agentinstance.ActivityQuery) (string, []any, error) {
	var values []string
	var args []any
	for index, query := range queries {
		if query.ProfileID == "" || query.AgentName == "" || query.Since.IsZero() {
			return "", nil, errors.New("activity requires profile, agent and time window")
		}
		workspaces := query.WorkspaceIDs
		workspaces = slices.Compact(slices.Sorted(slices.Values(workspaces)))
		for _, workspace := range workspaces {
			values = append(values, "(?, ?, ?, ?, ?, ?)")
			args = append(
				args,
				index,
				query.ProfileID,
				query.AgentName,
				workspace,
				store.FormatTimestamp(query.Since),
				store.LocalDay(query.Since),
			)
		}
	}
	if len(values) == 0 {
		return "", nil, nil
	}
	if len(values) > 5000 {
		return "", nil, fmt.Errorf("too many activity scopes: %d", len(values))
	}
	return "WITH scopes(request_id, profile_id, agent_name, workspace_id, since, day) AS (VALUES " + strings.Join(
		values,
		",",
	) + ") ", args, nil
}

const instanceActivityCosts = `SELECT s.request_id, u.workspace_id,
 COALESCE(SUM(CASE WHEN u.cost_currency = 'USD' THEN u.total_cost ELSE 0 END), 0), SUM(u.total_tokens)
 FROM scopes s JOIN token_usage_daily u
 ON u.profile_id = s.profile_id AND u.agent_name = s.agent_name AND u.workspace_id = s.workspace_id AND u.day >= s.day
 GROUP BY s.request_id, u.workspace_id ORDER BY s.request_id, u.workspace_id`

const instanceActivityReports = `SELECT request_id, date(created_at, 'localtime'),
 SUM(CASE WHEN kind = 'proposal' THEN 1 ELSE 0 END),
 SUM(CASE WHEN kind NOT IN ('proposal', 'quiet') THEN 1 ELSE 0 END),
 SUM(CASE WHEN kind = 'quiet' THEN 1 ELSE 0 END)
 FROM (
 SELECT s.request_id, r.created_at, r.kind FROM scopes s
 JOIN agent_instances i ON i.profile_id = s.profile_id AND i.agent_name = s.agent_name
 AND i.workspace_id = s.workspace_id
 JOIN agent_instance_reports r ON r.instance_id = i.id AND r.created_at >= s.since
 UNION ALL
 SELECT s.request_id, e.created_at, 'quiet' FROM scopes s
 JOIN agent_instances i ON i.profile_id = s.profile_id AND i.agent_name = s.agent_name
 AND i.workspace_id = s.workspace_id
 JOIN agent_heartbeat_wake_events e ON e.instance_id = i.id AND e.created_at >= s.since AND e.result = 'quiet'
 ) GROUP BY 1, 2 ORDER BY 1, 2`

const instanceActivityHours = `SELECT s.request_id,
 CAST(strftime('%w', e.timestamp, 'localtime') AS INTEGER),
 CAST(strftime('%H', e.timestamp, 'localtime') AS INTEGER), COUNT(*)
 FROM scopes s JOIN event_summaries e
 ON e.profile_id = s.profile_id AND e.agent_name = s.agent_name AND e.workspace_id = s.workspace_id
 AND e.timestamp >= s.since
 GROUP BY 1, 2, 3 ORDER BY 1, 2, 3`
