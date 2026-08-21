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
	statement, args := store.BuildWhereQuery(
		`SELECT day, CAST(SUM(input_tokens) AS INTEGER),
			CAST(SUM(output_tokens) AS INTEGER), CAST(SUM(total_tokens) AS INTEGER)
		FROM token_usage_daily`,
		` GROUP BY day ORDER BY day`,
		store.StringCompareClause("day", ">=", query.SinceDay),
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	rows, err := g.db.QueryContext(ctx, statement, args...)
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
	statement, args := store.BuildWhereQuery(
		`SELECT agent_name, CAST(SUM(total_tokens) AS INTEGER)
		FROM token_usage_daily`,
		` GROUP BY agent_name ORDER BY SUM(total_tokens) DESC, agent_name`,
		store.StringCompareClause("day", ">=", query.SinceDay),
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	rows, err := g.db.QueryContext(ctx, statement, args...)
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
	statement, args := store.BuildWhereQuery(
		`SELECT p.id, p.name, p.color, COALESCE(p.icon, ''), COALESCE(p.emoji, ''),
			p.state = 'archived', CAST(SUM(u.total_tokens) AS INTEGER)
		FROM token_usage_daily u JOIN profiles p ON p.id = u.profile_id`,
		` GROUP BY p.id, p.name, p.color, p.icon, p.emoji, p.state
		ORDER BY SUM(u.total_tokens) DESC, p.name, p.id`,
		store.StringCompareClause("u.day", ">=", query.SinceDay),
		store.ReadScopeClause("u.profile_id", query.ReadScope),
		store.StringClause("u.workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	rows, err := g.db.QueryContext(ctx, statement, args...)
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
	statement, args := store.BuildWhereQuery(
		`SELECT cost_status, cost_source, COALESCE(cost_currency, ''),
			CAST(SUM(COALESCE(total_cost, 0)) AS REAL),
			CAST(SUM(CASE WHEN total_cost IS NULL THEN 1 ELSE 0 END) AS INTEGER), COUNT(1)
		FROM token_usage_daily`,
		` GROUP BY cost_status, cost_source, COALESCE(cost_currency, '')`,
		store.StringCompareClause("day", ">=", query.SinceDay),
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	rows, err := g.db.QueryContext(ctx, statement, args...)
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
	statement, args := store.BuildAndQuery(
		`SELECT CAST(date(r.ended_at, 'localtime') AS TEXT),
			CAST(SUM(CASE WHEN r.status = 'completed' THEN 1 ELSE 0 END) AS INTEGER),
			CAST(SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END) AS INTEGER),
			CAST(SUM(CASE WHEN r.status = 'canceled' THEN 1 ELSE 0 END) AS INTEGER)
		FROM task_runs r JOIN tasks t ON t.id = r.task_id
		WHERE r.ended_at IS NOT NULL AND r.ended_at >= ?
			AND r.status IN ('completed', 'failed', 'canceled') AND r.run_kind = 'worker'`,
		` GROUP BY date(r.ended_at, 'localtime') ORDER BY 1`,
		store.ReadScopeClause("t.profile_id", query.ReadScope),
		store.StringClause("r.workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	rows, err := g.db.QueryContext(ctx, statement, args...)
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
	statement, args := store.BuildAndQuery(
		`SELECT CAST(date(closed_at, 'localtime') AS TEXT), COUNT(1)
		FROM tasks WHERE closed_at IS NOT NULL AND closed_at >= ? AND status = 'completed'`,
		` GROUP BY date(closed_at, 'localtime') ORDER BY 1`,
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	rows, err := g.db.QueryContext(ctx, statement, args...)
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
	statement, args := store.BuildAndQuery(
		`SELECT CAST(strftime('%w', timestamp, 'localtime') AS INTEGER),
			CAST(strftime('%H', timestamp, 'localtime') AS INTEGER), COUNT(1)
		FROM event_summaries WHERE timestamp >= ?`,
		` GROUP BY 1, 2 ORDER BY 1, 2`,
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	rows, err := g.db.QueryContext(ctx, statement, args...)
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
	statement, args := store.BuildWhereQuery(
		"SELECT CAST(COALESCE(MAX(timestamp), '') AS TEXT) FROM event_summaries",
		"",
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	var latest string
	if err := g.db.QueryRowContext(ctx, statement, args...).Scan(&latest); err != nil {
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
	statement, args := store.BuildAndQuery(
		`SELECT COUNT(1)
		 FROM network_audit_log AS audit
		 LEFT JOIN network_threads AS owner_thread
			ON owner_thread.workspace_id = audit.workspace_id
			AND owner_thread.channel = audit.channel
			AND owner_thread.thread_id = audit.thread_id
		 LEFT JOIN network_direct_rooms AS owner_direct
			ON owner_direct.workspace_id = audit.workspace_id
			AND owner_direct.channel = audit.channel
			AND owner_direct.direct_id = audit.direct_id
		 LEFT JOIN network_channels AS owner_channel
			ON owner_channel.workspace_id = audit.workspace_id
			AND owner_channel.channel = audit.channel
		 LEFT JOIN sessions AS owner_session
			ON owner_session.workspace_id = audit.workspace_id
			AND owner_session.id = audit.session_id
		 WHERE audit.timestamp >= ?`,
		"",
		store.ReadScopeCoalescedClause([]string{
			"owner_thread.profile_id",
			"owner_direct.profile_id",
			"owner_channel.profile_id",
			"owner_session.profile_id",
		}, query.ReadScope),
		store.StringClause("audit.workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	var count int
	if err := g.db.QueryRowContext(ctx, statement, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: count network audit messages: %w", err)
	}
	return count, nil
}

func (g *ObserveRepo) queryHookDispatchCounts(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (store.HookDispatchCounts, error) {
	statement, args := store.BuildAndQuery(
		`SELECT COUNT(1), CAST(COALESCE(SUM(CASE WHEN outcome = 'failure' THEN 1 ELSE 0 END), 0) AS INTEGER)
		FROM event_summaries WHERE type = 'hook.dispatch.complete' AND timestamp >= ?`,
		"",
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(query.Since)}, args...)
	var counts store.HookDispatchCounts
	if err := g.db.QueryRowContext(ctx, statement, args...).Scan(&counts.Runs, &counts.Failures); err != nil {
		return store.HookDispatchCounts{}, fmt.Errorf("store: count hook dispatches: %w", err)
	}
	return counts, nil
}

func (g *ObserveRepo) queryLongestUserSession(
	ctx context.Context,
	query store.OverviewSinceQuery,
) (*store.LongestSessionSample, error) {
	statement, args := store.BuildAndQuery(
		`SELECT id, agent_name, created_at,
			CAST(MAX(0, CAST(strftime('%s', CASE WHEN state = 'active' THEN ? ELSE updated_at END) AS INTEGER) -
			CAST(strftime('%s', created_at) AS INTEGER)) AS INTEGER)
		FROM sessions WHERE session_type = 'user' AND created_at >= ?`,
		` ORDER BY 4 DESC, created_at DESC LIMIT 1`,
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.StringClause("workspace_id", strings.TrimSpace(query.WorkspaceID)),
	)
	args = append([]any{store.FormatTimestamp(g.now()), store.FormatTimestamp(query.Since)}, args...)
	row := g.db.QueryRowContext(ctx, statement, args...)
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
