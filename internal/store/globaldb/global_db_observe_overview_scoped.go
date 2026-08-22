package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func (g *ObserveRepo) queryTokenUsageDays(
	ctx context.Context,
	query store.OverviewDayQuery,
) (days []store.TokenUsageDay, err error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	where = append([]string{"day >= ?"}, where...)
	args = append([]any{strings.TrimSpace(query.SinceDay)}, args...)
	rows, err := g.db.QueryContext(ctx, `
		SELECT day, CAST(SUM(input_tokens) AS INTEGER),
			CAST(SUM(output_tokens) AS INTEGER), CAST(SUM(total_tokens) AS INTEGER)
		FROM token_usage_daily WHERE `+overviewWhere(where)+` GROUP BY day ORDER BY day`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query token usage by day: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "token usage by day") }()
	for rows.Next() {
		var day store.TokenUsageDay
		if scanErr := rows.Scan(&day.Day, &day.InputTokens, &day.OutputTokens, &day.TotalTokens); scanErr != nil {
			return nil, fmt.Errorf("store: scan token usage by day: %w", scanErr)
		}
		days = append(days, day)
	}
	return days, rows.Err()
}

func (g *ObserveRepo) queryTokenUsageAgents(
	ctx context.Context,
	query store.OverviewDayQuery,
) (totals []store.TokenUsageAgentTotal, err error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	where = append([]string{"day >= ?"}, where...)
	args = append([]any{strings.TrimSpace(query.SinceDay)}, args...)
	rows, err := g.db.QueryContext(ctx, `
		SELECT agent_name, CAST(SUM(total_tokens) AS INTEGER)
		FROM token_usage_daily WHERE `+overviewWhere(where)+`
		GROUP BY agent_name ORDER BY SUM(total_tokens) DESC, agent_name`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query token usage by agent: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "token usage by agent") }()
	for rows.Next() {
		var total store.TokenUsageAgentTotal
		if scanErr := rows.Scan(&total.AgentName, &total.TotalTokens); scanErr != nil {
			return nil, fmt.Errorf("store: scan token usage by agent: %w", scanErr)
		}
		total.AgentName = strings.TrimSpace(total.AgentName)
		totals = append(totals, total)
	}
	return totals, rows.Err()
}

func (g *ObserveRepo) queryTokenUsageProfiles(
	ctx context.Context,
	query store.OverviewDayQuery,
) (totals []store.TokenUsageProfileTotal, err error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("u.profile_id", query.ReadScope),
		store.StringClause("u.workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	where = append([]string{"u.day >= ?"}, where...)
	args = append([]any{strings.TrimSpace(query.SinceDay)}, args...)
	rows, err := g.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.color, COALESCE(p.icon, ''), COALESCE(p.emoji, ''),
			p.state = 'archived', CAST(SUM(u.total_tokens) AS INTEGER)
		FROM token_usage_daily u JOIN profiles p ON p.id = u.profile_id
		WHERE `+overviewWhere(where)+`
		GROUP BY p.id, p.name, p.color, p.icon, p.emoji, p.state
		ORDER BY SUM(u.total_tokens) DESC, p.name, p.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query token usage by profile: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "token usage by profile") }()
	for rows.Next() {
		var total store.TokenUsageProfileTotal
		if scanErr := rows.Scan(
			&total.ProfileID,
			&total.ProfileName,
			&total.Color,
			&total.Icon,
			&total.Emoji,
			&total.Archived,
			&total.TotalTokens,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan token usage by profile: %w", scanErr)
		}
		totals = append(totals, total)
	}
	return totals, rows.Err()
}

func (g *ObserveRepo) queryTokenUsageCosts(
	ctx context.Context,
	query store.OverviewDayQuery,
) (groups []store.TokenUsageCostGroup, err error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	where = append([]string{"day >= ?"}, where...)
	args = append([]any{strings.TrimSpace(query.SinceDay)}, args...)
	rows, err := g.db.QueryContext(ctx, `
		SELECT cost_status, cost_source, COALESCE(cost_currency, ''),
			CAST(SUM(COALESCE(total_cost, 0)) AS REAL),
			CAST(SUM(CASE WHEN total_cost IS NULL THEN 1 ELSE 0 END) AS INTEGER), COUNT(1)
		FROM token_usage_daily WHERE `+overviewWhere(where)+`
		GROUP BY cost_status, cost_source, COALESCE(cost_currency, '')`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query token usage cost: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "token usage cost") }()
	for rows.Next() {
		var group store.TokenUsageCostGroup
		if scanErr := rows.Scan(
			&group.CostStatus, &group.CostSource, &group.CostCurrency, &group.TotalCost,
			&group.RowsWithoutCost, &group.RowsTotal,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan token usage cost: %w", scanErr)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (g *ObserveRepo) queryTaskRunOutcomeDays(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (days []store.TaskRunOutcomeDay, err error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("t.profile_id", query.ReadScope),
		store.StringClause("r.workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	rows, err := g.db.QueryContext(ctx, `
		SELECT CAST(date(r.ended_at, 'localtime') AS TEXT),
			CAST(SUM(CASE WHEN r.status = 'completed' THEN 1 ELSE 0 END) AS INTEGER),
			CAST(SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END) AS INTEGER),
			CAST(SUM(CASE WHEN r.status = 'canceled' THEN 1 ELSE 0 END) AS INTEGER)
		FROM task_runs r JOIN tasks t ON t.id = r.task_id
		WHERE r.ended_at IS NOT NULL AND r.ended_at >= ?
			AND r.status IN ('completed', 'failed', 'canceled') AND r.run_kind = 'worker'
			AND `+overviewWhere(where)+` GROUP BY date(r.ended_at, 'localtime') ORDER BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query task run outcomes by day: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "task run outcomes") }()
	for rows.Next() {
		var day store.TaskRunOutcomeDay
		if scanErr := rows.Scan(&day.Day, &day.Completed, &day.Failed, &day.Canceled); scanErr != nil {
			return nil, fmt.Errorf("store: scan task run outcomes: %w", scanErr)
		}
		days = append(days, day)
	}
	return days, rows.Err()
}

func (g *ObserveRepo) queryTaskClosedDays(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (days []store.TaskClosedDay, err error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	rows, err := g.db.QueryContext(ctx, `
		SELECT CAST(date(closed_at, 'localtime') AS TEXT), COUNT(1)
		FROM tasks WHERE closed_at IS NOT NULL AND closed_at >= ? AND status = 'completed'
			AND `+overviewWhere(where)+` GROUP BY date(closed_at, 'localtime') ORDER BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query tasks closed by day: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "tasks closed by day") }()
	for rows.Next() {
		var day store.TaskClosedDay
		if scanErr := rows.Scan(&day.Day, &day.Closed); scanErr != nil {
			return nil, fmt.Errorf("store: scan tasks closed by day: %w", scanErr)
		}
		days = append(days, day)
	}
	return days, rows.Err()
}

func (g *ObserveRepo) queryEventHourWeekdays(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (buckets []store.EventHourWeekdayBucket, err error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	rows, err := g.db.QueryContext(ctx, `
		SELECT CAST(strftime('%w', timestamp, 'localtime') AS INTEGER),
			CAST(strftime('%H', timestamp, 'localtime') AS INTEGER), COUNT(1)
		FROM event_summaries WHERE timestamp >= ? AND `+overviewWhere(where)+`
		GROUP BY 1, 2 ORDER BY 1, 2`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query events by hour and weekday: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "event pulse") }()
	for rows.Next() {
		var bucket store.EventHourWeekdayBucket
		if scanErr := rows.Scan(&bucket.Weekday, &bucket.Hour, &bucket.Events); scanErr != nil {
			return nil, fmt.Errorf("store: scan event pulse: %w", scanErr)
		}
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func (g *ObserveRepo) queryLatestEventSummaryAt(
	ctx context.Context,
	query store.OverviewWorkspaceQuery,
) (time.Time, error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	var latest string
	if err := g.db.QueryRowContext(
		ctx,
		"SELECT CAST(COALESCE(MAX(timestamp), '') AS TEXT) FROM event_summaries WHERE "+overviewWhere(where),
		args...,
	).Scan(&latest); err != nil {
		return time.Time{}, fmt.Errorf("store: query latest event summary timestamp: %w", err)
	}
	if strings.TrimSpace(latest) == "" {
		return time.Time{}, nil
	}
	parsed, err := store.ParseTimestamp(latest)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse latest event summary timestamp: %w", err)
	}
	return parsed, nil
}

func (g *ObserveRepo) queryNetworkMessageCount(ctx context.Context, query store.OverviewSinceQuery) (int, error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	var count int
	if err := g.db.QueryRowContext(
		ctx,
		"SELECT COUNT(1) FROM network_audit_log WHERE timestamp >= ? AND "+overviewWhere(where),
		args...,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: count network audit messages: %w", err)
	}
	return count, nil
}

func (g *ObserveRepo) queryHookDispatchCounts(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (store.HookDispatchCounts, error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	var counts store.HookDispatchCounts
	if err := g.db.QueryRowContext(ctx, `
		SELECT COUNT(1), CAST(COALESCE(SUM(CASE WHEN outcome = 'failure' THEN 1 ELSE 0 END), 0) AS INTEGER)
		FROM event_summaries WHERE type = 'hook.dispatch.complete' AND timestamp >= ? AND `+overviewWhere(where),
		args...,
	).Scan(&counts.Runs, &counts.Failures); err != nil {
		return store.HookDispatchCounts{}, fmt.Errorf("store: count hook dispatches: %w", err)
	}
	return counts, nil
}

func (g *ObserveRepo) queryLongestUserSession(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (*store.LongestSessionSample, error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(g.now()), store.FormatTimestamp(query.Since)}, args...)
	row := g.db.QueryRowContext(ctx, `
		SELECT id, agent_name, created_at,
			CAST(MAX(0, CAST(strftime('%s', CASE WHEN state = 'active' THEN ? ELSE updated_at END) AS INTEGER) -
			CAST(strftime('%s', created_at) AS INTEGER)) AS INTEGER)
		FROM sessions WHERE session_type = 'user' AND created_at >= ? AND `+overviewWhere(where)+`
		ORDER BY 4 DESC, created_at DESC LIMIT 1`, args...)
	var sample store.LongestSessionSample
	var startedAt string
	if err := row.Scan(&sample.SessionID, &sample.AgentName, &startedAt, &sample.RuntimeSeconds); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: query longest user session: %w", err)
	}
	parsed, err := store.ParseTimestamp(startedAt)
	if err != nil {
		return nil, fmt.Errorf("store: parse longest session start: %w", err)
	}
	sample.AgentName = strings.TrimSpace(sample.AgentName)
	sample.StartedAt = parsed
	return &sample, nil
}

func overviewWhere(clauses []string) string {
	if len(clauses) == 0 {
		return "1 = 1"
	}
	return strings.Join(clauses, " AND ")
}
