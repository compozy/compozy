package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/store"
)

const (
	sessionCatalogSessionTypeColumn = "session_type"
	sessionCatalogSpawnRoleColumn   = "spawn_role"
	sessionCatalogSortRecent        = "recent"
	sessionCatalogSortCreated       = "created"
	sessionCatalogSortName          = "name"
	sessionCatalogSortNavigator     = "navigator"
	sessionCatalogSortAttention     = "attention"
)

const sessionCatalogVerdictOnlyExpression = `(session_type = 'system' AND EXISTS (
 SELECT 1 FROM session_creation_profiles AS policy
 WHERE policy.profile_ref = sessions.creation_profile_ref
 AND json_extract(policy.profile_json, '$.runtime_mode') = 'verdict-only'))`

// sessionCatalogHierarchyChildExpression is store.HierarchyParentSessionID in SQL: a parented
// session is a child of its parent unless it was continued or forked from it, which names the
// source as provenance and makes the session a new top-level session.
var sessionCatalogHierarchyChildExpression = fmt.Sprintf(
	"(COALESCE(parent_session_id, '') <> '' AND lineage_kind NOT IN ('%s', '%s'))",
	store.LineageKindContinue,
	store.LineageKindFork,
)

const sessionInfoSelectQuery = `SELECT effective_workspace_access, id, profile_id, name, agent_name, provider, model,
	reasoning_effort, speed,
	acp_options_json, speed_resolution_json, runtime_status, runtime_transition, runtime_failure,
	runtime_generation, runtime_recovery_json,
	selected_provider, selected_model, selected_reasoning_effort, selected_speed, selected_acp_options_json,
	runtime_selection_revision, workspace_id, worktree_id,
	session_type,
	parent_session_id, root_session_id, spawn_depth, spawn_role, ttl_expires_at,
	auto_stop_on_parent, notify_creator, spawn_budget_json, permission_policy_json,
	lineage_kind, origin_message_id, origin_agent_name,
	state, archived_at, acp_session_id, stop_reason, stop_escalated, stop_verification_failed, stop_detail,
	failure_kind, failure_summary, crash_bundle_path,
	subprocess_pid, subprocess_started_at, last_update_at, stall_state, stall_reason,
	activity_json, attached_to, attach_expires_at, transcript_epoch,
	pending_permission_count, pending_clarify_count, attention_revision,
	last_settled_revision, last_seen_revision, last_seen_at, attention_changed_at, ended_early_turn_id,
	soul_snapshot_id, soul_digest, parent_soul_digest,
	created_at, updated_at, ` + sessionCatalogVerdictOnlyExpression + `
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

	if !query.SkipTotal {
		source, sourceArgs, sourceErr := sessionCatalogPageSource(query)
		if sourceErr != nil {
			return store.SessionCatalogPage{}, sourceErr
		}
		countQuery := store.AppendWhere("SELECT COUNT(1) FROM "+source, where)
		// dynamic-sql: page filters and exclusion slices alter the count query structure.
		if err := tx.QueryRowContext(ctx, countQuery, append(sourceArgs, args...)...).Scan(&page.Total); err != nil {
			return store.SessionCatalogPage{}, fmt.Errorf("store: count session catalog page: %w", err)
		}
	}
	page.Sessions, err = querySessionCatalogRows(ctx, tx, query, where, args)
	if err != nil {
		return store.SessionCatalogPage{}, err
	}
	// Reading a page must not acquire the writer or sweep unrelated history.
	// Expired leases are absent in the read projection; attach mutations still
	// own durable lease reclamation.
	for index := range page.Sessions {
		info := &page.Sessions[index]
		if expiry := info.AttachExpiresAtValue(); expiry != nil && !expiry.After(now) {
			info.SetAttach("", nil)
		}
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
	source, sourceArgs, err := sessionCatalogPageSource(query)
	if err != nil {
		return nil, err
	}
	selectQuery := strings.TrimSuffix(sessionInfoSelectQuery, "FROM sessions") + "FROM " + source
	sqlQuery := store.AppendWhere(selectQuery, where)
	args = append(sourceArgs, args...)
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

func appendSessionCatalogVisibilityFilters(where []string, query store.SessionCatalogPageQuery) []string {
	if query.ExcludeVerdictOnly {
		where = append(where, "NOT "+sessionCatalogVerdictOnlyExpression)
	}
	return appendSubagentSessionFilter(where, query.Subagents)
}

// appendSessionWorkspaceIDsFilter narrows to the All projects Project filter; @user selects User rows.
func appendSessionWorkspaceIDsFilter(where []string, args []any, workspaceIDs []string) ([]string, []any, error) {
	if len(workspaceIDs) == 0 {
		return where, args, nil
	}
	ids := append([]string(nil), workspaceIDs...)
	for i, id := range ids {
		if id == workspaceaccess.UserSelector {
			ids[i] = ""
		}
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return nil, nil, fmt.Errorf("store: encode session workspace filter: %w", err)
	}
	where = append(where, "workspace_id IN (SELECT CAST(value AS TEXT) FROM json_each(?))")
	return where, append(args, string(encoded)), nil
}

func sessionCatalogPageFilters(
	query store.SessionCatalogPageQuery,
	now string,
) ([]string, []any, error) {
	where, args := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.SessionWorkspaceClause(query.WorkspaceID),
		store.StringClause("worktree_id", query.WorktreeID),
		store.StringClause("state", query.State),
		store.StringClause(sessionCatalogSessionTypeColumn, query.SessionType),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("parent_session_id", query.ParentSessionID),
		store.StringClause("root_session_id", query.RootSessionID),
		store.StringClause("lineage_kind", string(query.LineageKind)),
	)
	where, args, err := appendSessionWorkspaceIDsFilter(where, args, query.WorkspaceIDs)
	if err != nil {
		return nil, nil, err
	}
	where, args = appendSessionCatalogSearch(where, args, query)
	if query.AttentionOnly {
		where = append(
			where,
			sessionCatalogBadgeExpression+" IN ('needs-attention', 'waiting-for-auth', 'waiting-for-input', 'failed')",
		)
	}
	if len(query.Badges) > 0 {
		placeholders := make([]string, len(query.Badges))
		for index, badge := range query.Badges {
			placeholders[index] = "?"
			args = append(args, badge)
		}
		where = append(where, sessionCatalogBadgeExpression+" IN ("+strings.Join(placeholders, ",")+")")
	}
	if query.HideChildren {
		where = append(where, "NOT "+sessionCatalogHierarchyChildExpression)
	}
	if len(query.Columns) > 0 {
		placeholders := make([]string, len(query.Columns))
		for index, column := range query.Columns {
			placeholders[index] = "?"
			args = append(args, column)
		}
		where = append(where, sessionCatalogBoardColumnExpression+" IN ("+strings.Join(placeholders, ",")+")")
	}
	where = appendSessionCatalogVisibilityFilters(where, query)
	if query.Resumable {
		where = append(where, sessionListResumableWhere)
		args = append(args, now)
	}
	where, args = appendSessionArchiveFilter(where, args, query.Archive)
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

func appendSessionArchiveFilter(
	where []string,
	args []any,
	filter store.SessionArchiveFilter,
) ([]string, []any) {
	switch filter {
	case store.SessionArchiveExclude:
		where = append(where, "archived_at IS NULL")
	case store.SessionArchiveOnly:
		where = append(where, "archived_at IS NOT NULL")
	}
	return where, args
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
	case sessionCatalogSortNavigator:
		return sessionCatalogNavigatorOrderClause, nil
	case sessionCatalogSortName:
		return " ORDER BY " + sessionCatalogNameExpression + " ASC, created_at DESC, id DESC", nil
	case sessionCatalogSortCreated:
		return heartbeatOrderByCreatedDesc, nil
	case sessionCatalogSortRecent:
		return " ORDER BY updated_at DESC, created_at DESC, id DESC", nil
	case sessionListSortActivity:
		return " ORDER BY COALESCE(last_update_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC", nil
	case sessionCatalogSortAttention:
		return sessionCatalogAttentionOrderClause, nil
	case sessionCatalogSortManual:
		return sessionCatalogManualOrderClause, nil
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
	if strings.TrimSpace(sortKey) == sessionCatalogSortNavigator {
		return sessionCatalogNavigatorCursorClause(position, primary)
	}
	if strings.TrimSpace(sortKey) == sessionCatalogSortManual {
		return sessionCatalogManualCursorClause(position, created)
	}
	if strings.TrimSpace(sortKey) == sessionCatalogSortName {
		return `(` + sessionCatalogNameExpression + ` > ? OR (` + sessionCatalogNameExpression + ` = ? AND
			(created_at < ? OR (created_at = ? AND id < ?))))`, []any{
			*position.Name, *position.Name, created, created, strings.TrimSpace(position.ID),
		}
	}
	if strings.TrimSpace(sortKey) == sessionCatalogSortCreated {
		return `(created_at < ? OR (created_at = ? AND id < ?))`, []any{
			created,
			created,
			strings.TrimSpace(position.ID),
		}
	}
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
	if strings.TrimSpace(sortKey) == sessionCatalogSortAttention {
		return sessionCatalogAttentionCursorClause(position, primary, secondary, created)
	}
	clause := `(updated_at < ? OR (updated_at = ? AND created_at < ?) OR
		(updated_at = ? AND created_at = ? AND id < ?))`
	return clause, []any{primary, primary, created, primary, created, strings.TrimSpace(position.ID)}
}

var _ store.SessionCatalogPager = (*SessionRepo)(nil)

func appendSubagentSessionFilter(where []string, filter string) []string {
	switch filter {
	case "exclude":
		return append(where, "(spawn_role IS NULL OR spawn_role <> 'subagent')")
	case "only":
		return append(where, "spawn_role = 'subagent'")
	default:
		return where
	}
}

func appendSessionCatalogSearch(where []string, args []any, query store.SessionCatalogPageQuery) ([]string, []any) {
	if search := strings.ToLower(strings.TrimSpace(query.Search)); search != "" {
		if query.SearchFields == "title_agent" {
			search = store.LowerSessionCatalogSearchText(strings.TrimSpace(query.Search))
			where = append(
				where,
				"(instr(compozy_unicode_lower("+sessionCatalogDisplayTitleExpression+"), ?) > 0 OR "+
					"instr(compozy_unicode_lower(agent_name), ?) > 0)",
			)
			args = append(args, search, search)
		} else {
			where = append(where, `(instr(lower(id), ?) > 0 OR
			instr(lower(COALESCE(name, '')), ?) > 0 OR
			instr(lower(agent_name), ?) > 0 OR
			instr(lower(provider), ?) > 0)`)
			for range 4 {
				args = append(args, search)
			}
		}
	}
	return where, args
}
