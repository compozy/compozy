package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
)

const (
	sessionCatalogSessionTypeColumn = "session_type"
	sessionCatalogSpawnRoleColumn   = "spawn_role"
	sessionCatalogSortRecent        = "recent"
)

const sessionInfoSelectQuery = `SELECT id, name, agent_name, provider, workspace_id,
	network_spec_json, network_mode, network_channel, network_source, session_type,
	parent_session_id, root_session_id, spawn_depth, spawn_role, ttl_expires_at,
	auto_stop_on_parent, spawn_budget_json, permission_policy_json,
	state, acp_session_id, stop_reason, stop_detail,
	failure_kind, failure_summary, crash_bundle_path,
	subprocess_pid, subprocess_started_at, last_update_at, stall_state, stall_reason,
	activity_json, attached_to, attach_expires_at, transcript_epoch,
	soul_snapshot_id, soul_digest, parent_soul_digest,
	sandbox_id, sandbox_backend, sandbox_profile, sandbox_instance_id,
	sandbox_state, sandbox_provider_state_json,
	sandbox_last_sync_at, sandbox_last_sync_error,
	created_at, updated_at
FROM sessions`

// PageSessions returns one bounded durable-catalog page and a filter-scoped total.
func (g *SessionRepo) PageSessions(
	ctx context.Context,
	query store.SessionCatalogPageQuery,
) (page store.SessionCatalogPage, err error) {
	if err := g.checkReady(ctx, "page sessions"); err != nil {
		return store.SessionCatalogPage{}, err
	}
	if err := query.Validate(); err != nil {
		return store.SessionCatalogPage{}, err
	}
	now := g.now()
	if _, err := g.SweepExpiredSessionAttachLocks(ctx, now); err != nil {
		return store.SessionCatalogPage{}, err
	}

	where, args, err := sessionCatalogPageFilters(query, store.FormatTimestamp(now))
	if err != nil {
		return store.SessionCatalogPage{}, err
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return store.SessionCatalogPage{}, fmt.Errorf("store: begin session catalog page transaction: %w", err)
	}
	defer func() {
		joinCleanupError(&err, rollbackTx(tx, "session catalog page"))
	}()

	countQuery := store.AppendWhere("SELECT COUNT(1) FROM sessions", where)
	// dynamic-sql: page filters and exclusion slices alter the count query structure.
	if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&page.Total); err != nil {
		return store.SessionCatalogPage{}, fmt.Errorf("store: count session catalog page: %w", err)
	}
	page.Sessions, err = querySessionCatalogRows(ctx, tx, query, where, args)
	if err != nil {
		return store.SessionCatalogPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return store.SessionCatalogPage{}, fmt.Errorf("store: commit session catalog page transaction: %w", err)
	}
	return page, nil
}

func querySessionCatalogRows(
	ctx context.Context,
	tx *sql.Tx,
	query store.SessionCatalogPageQuery,
	baseWhere []string,
	baseArgs []any,
) (sessions []store.SessionInfo, err error) {
	where := append([]string(nil), baseWhere...)
	args := append([]any(nil), baseArgs...)
	if query.After != nil {
		cursorClause, cursorArgs := sessionCatalogCursorClause(query.Sort, *query.After)
		where = append(where, cursorClause)
		args = append(args, cursorArgs...)
	}
	sqlQuery := store.AppendWhere(sessionInfoSelectQuery, where)
	order, err := sessionCatalogOrderClause(query.Sort)
	if err != nil {
		return nil, err
	}
	sqlQuery += order
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	// dynamic-sql: keyset direction, cursor predicates, filters, and limit alter the page query structure.
	rows, err := tx.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query session catalog page: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close session catalog page rows: %w", closeErr))
		}
	}()

	sessions = make([]store.SessionInfo, 0, query.Limit)
	for rows.Next() {
		sessionInfo, scanErr := scanSessionInfo(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		sessions = append(sessions, sessionInfo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate session catalog page: %w", err)
	}
	return sessions, nil
}

func sessionCatalogPageFilters(
	query store.SessionCatalogPageQuery,
	now string,
) ([]string, []any, error) {
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("state", query.State),
		store.StringClause(sessionCatalogSessionTypeColumn, query.SessionType),
		store.StringClause("agent_name", query.AgentName),
	)
	if search := strings.ToLower(strings.TrimSpace(query.Search)); search != "" {
		where = append(where, `(instr(lower(id), ?) > 0 OR
			instr(lower(COALESCE(name, '')), ?) > 0 OR
			instr(lower(agent_name), ?) > 0 OR
			instr(lower(provider), ?) > 0 OR
			instr(lower(COALESCE(network_channel, '')), ?) > 0)`)
		for range 5 {
			args = append(args, search)
		}
	}
	if query.Resumable {
		where = append(where, sessionListResumableWhere)
		args = append(args, now)
	}

	var err error
	where, args, err = appendSessionCatalogExclusion(where, args, "id", query.ExcludeIDs)
	if err != nil {
		return nil, nil, err
	}
	where, args, err = appendSessionCatalogExclusion(
		where,
		args,
		sessionCatalogSessionTypeColumn,
		query.ExcludeSessionTypes,
	)
	if err != nil {
		return nil, nil, err
	}
	where, args, err = appendSessionCatalogExclusion(
		where,
		args,
		sessionCatalogSpawnRoleColumn,
		query.ExcludeSpawnRoles,
	)
	if err != nil {
		return nil, nil, err
	}
	return where, args, nil
}

func appendSessionCatalogExclusion(
	where []string,
	args []any,
	column string,
	values []string,
) ([]string, []any, error) {
	expression, caseFold, err := sessionCatalogExclusionExpression(column)
	if err != nil {
		return nil, nil, err
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if caseFold {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return where, args, nil
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, fmt.Errorf("store: encode session catalog %s exclusions: %w", column, err)
	}
	where = append(where, expression+" NOT IN (SELECT CAST(value AS TEXT) FROM json_each(?))")
	args = append(args, string(encoded))
	return where, args, nil
}

func sessionCatalogExclusionExpression(column string) (string, bool, error) {
	switch column {
	case "id":
		return "id", false, nil
	case sessionCatalogSessionTypeColumn:
		return "lower(COALESCE(session_type, ''))", true, nil
	case sessionCatalogSpawnRoleColumn:
		return "lower(COALESCE(spawn_role, ''))", true, nil
	default:
		return "", false, fmt.Errorf("store: unsupported session catalog exclusion column %q", column)
	}
}

func sessionCatalogOrderClause(sortKey string) (string, error) {
	switch strings.TrimSpace(sortKey) {
	case sessionCatalogSortRecent:
		return " ORDER BY updated_at DESC, created_at DESC, id DESC", nil
	case sessionListSortActivity:
		return " ORDER BY COALESCE(last_update_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC", nil
	default:
		return "", fmt.Errorf("store: unsupported session catalog sort %q", sortKey)
	}
}

func sessionCatalogCursorClause(
	sortKey string,
	position store.SessionCatalogPosition,
) (string, []any) {
	primary := store.FormatTimestamp(position.PrimaryAt)
	secondary := store.FormatTimestamp(position.SecondaryAt)
	created := store.FormatTimestamp(position.CreatedAt)
	if strings.TrimSpace(sortKey) == sessionListSortActivity {
		clause := `(COALESCE(last_update_at, updated_at) < ? OR
			(COALESCE(last_update_at, updated_at) = ? AND updated_at < ?) OR
			(COALESCE(last_update_at, updated_at) = ? AND updated_at = ? AND created_at < ?) OR
			(COALESCE(last_update_at, updated_at) = ? AND updated_at = ? AND created_at = ? AND id < ?))`
		return clause, []any{
			primary,
			primary, secondary,
			primary, secondary, created,
			primary, secondary, created, strings.TrimSpace(position.ID),
		}
	}
	clause := `(updated_at < ? OR (updated_at = ? AND created_at < ?) OR
		(updated_at = ? AND created_at = ? AND id < ?))`
	return clause, []any{primary, primary, created, primary, created, strings.TrimSpace(position.ID)}
}

var _ store.SessionCatalogPager = (*SessionRepo)(nil)
