package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
)

// AggregateSessionsByAgent returns exact durable visible-session metrics grouped by agent.
func (g *SessionRepo) AggregateSessionsByAgent(
	ctx context.Context,
	query store.SessionAgentMetricsQuery,
) (metrics []store.SessionAgentMetrics, err error) {
	if err := g.checkReady(ctx, "aggregate sessions by agent"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	now := g.now()
	if _, err := g.SweepExpiredSessionAttachLocks(ctx, now); err != nil {
		return nil, fmt.Errorf("store: sweep expired session attach locks before aggregating agents: %w", err)
	}

	where, args, err := sessionCatalogPageFilters(store.SessionCatalogPageQuery{
		WorkspaceID:         strings.TrimSpace(query.WorkspaceID),
		ExcludeIDs:          query.ExcludeIDs,
		ExcludeSessionTypes: query.ExcludeSessionTypes,
		ExcludeSpawnRoles:   query.ExcludeSpawnRoles,
	}, store.FormatTimestamp(now))
	if err != nil {
		return nil, err
	}
	where = append(where, "trim(agent_name) != ''")
	runtimeEnd := "CASE WHEN state = 'active' THEN ? ELSE updated_at END"
	runtimeSeconds := "MAX(0, CAST(strftime('%s', " + runtimeEnd + ") AS INTEGER) - " +
		"CAST(strftime('%s', created_at) AS INTEGER))"
	lastActivity := "MAX(COALESCE(last_update_at, updated_at))"
	sqlQuery := store.AppendWhere(`SELECT agent_name,
		COUNT(1),
		SUM(CASE WHEN state = 'active' THEN 1 ELSE 0 END),
		SUM(CASE WHEN state = 'stopped' AND (
			trim(COALESCE(failure_kind, '')) != '' OR
			stop_reason IN ('agent_crashed', 'error')
		) THEN 1 ELSE 0 END),
		SUM(`+runtimeSeconds+`),
		`+lastActivity+`
		FROM sessions`, where)
	sqlQuery += " GROUP BY agent_name ORDER BY lower(agent_name), agent_name"
	args = append([]any{store.FormatTimestamp(now)}, args...)

	// dynamic-sql: exclusion slices and catalog visibility filters alter the aggregate query structure.
	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query session agent metrics: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close session agent metric rows: %w", closeErr))
		}
	}()

	metrics = make([]store.SessionAgentMetrics, 0)
	for rows.Next() {
		var metric store.SessionAgentMetrics
		var lastActivityRaw string
		if scanErr := rows.Scan(
			&metric.AgentName,
			&metric.Total,
			&metric.Active,
			&metric.Failed,
			&metric.RuntimeSeconds,
			&lastActivityRaw,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan session agent metrics: %w", scanErr)
		}
		metric.AgentName = strings.TrimSpace(metric.AgentName)
		metric.LastActivityAt, err = store.ParseTimestamp(lastActivityRaw)
		if err != nil {
			return nil, fmt.Errorf("store: parse session agent last activity: %w", err)
		}
		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate session agent metrics: %w", err)
	}
	return metrics, nil
}

var _ store.SessionAgentMetricsReader = (*SessionRepo)(nil)
