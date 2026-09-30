package globaldb

import (
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// The numeric branches match store.SessionCatalogAttentionRank. Needs-you rows
// lead, followed by unseen finished rows and then sessions with no attention.
const sessionCatalogAttentionRankExpression = `(CASE
	WHEN (stop_verification_failed = 1 AND state <> 'stopped')
		OR pending_permission_count > 0
		OR pending_clarify_count > 0
		OR trim(COALESCE(failure_kind, '')) IN ('` + string(store.FailurePermission) + `', '` +
	string(store.FailureProviderAuth) + `')
		OR (state = 'stopped'
			AND trim(COALESCE(failure_kind, '')) <> ''
			AND trim(COALESCE(failure_kind, '')) <> '` + string(store.FailureCanceled) + `')
		THEN 0
	WHEN state = 'active'
		AND trim(COALESCE(stall_state, '')) <> '` + store.SessionStallStateDetected + `'
		AND trim(COALESCE(json_extract(
			CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END,
			'$.turn_id'
		), '')) = ''
		AND last_settled_revision > last_seen_revision
		THEN 1
	ELSE 2
END)`

const sessionCatalogDisplayTitleExpression = `(CASE WHEN trim(COALESCE(name, '')) <> ''
 AND trim(name) <> id AND trim(name) <> trim(agent_name)
 THEN trim(name) ELSE 'New session' END)`

// This expression follows session.CanonicalBadge precedence for durable snapshots.
// Health/supervision are runtime overlays and are applied by the manager before paging.
const sessionCatalogBadgeExpression = `(CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 'needs-attention'
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, ''))
 NOT IN ('', '` + string(store.FailureCanceled) + `') THEN 'failed'
 WHEN pending_permission_count > 0
 OR trim(COALESCE(failure_kind, '')) IN ('` + string(store.FailurePermission) + `', '` +
	string(store.FailureProviderAuth) + `') THEN 'waiting-for-auth'
 WHEN pending_clarify_count > 0 THEN 'waiting-for-input'
 WHEN state = 'stopped' THEN 'stopped'
 WHEN trim(COALESCE(stall_state, '')) = '` + store.SessionStallStateDetected + `' THEN 'hung'
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(
 CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 'running'
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 'done'
 WHEN state = 'active' THEN 'idle'
 ELSE 'unknown' END)`

const sessionCatalogAttentionOrderClause = " ORDER BY " + sessionCatalogAttentionRankExpression +
	" ASC, COALESCE(attention_changed_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC"

func sessionCatalogAttentionCursorClause(
	position store.SessionCatalogPosition,
	primary string,
	secondary string,
	created string,
) (string, []any) {
	clause := `(` + sessionCatalogAttentionRankExpression + ` > ? OR
		(` + sessionCatalogAttentionRankExpression + ` = ? AND
			(COALESCE(attention_changed_at, updated_at) < ? OR
			(COALESCE(attention_changed_at, updated_at) = ? AND updated_at < ?) OR
			(COALESCE(attention_changed_at, updated_at) = ? AND updated_at = ? AND created_at < ?) OR
			(COALESCE(attention_changed_at, updated_at) = ? AND updated_at = ? AND created_at = ? AND id < ?))))`
	rank := int(position.AttentionRank)
	return clause, []any{
		rank,
		rank, primary,
		primary, secondary,
		primary, secondary, created,
		primary, secondary, created, strings.TrimSpace(position.ID),
	}
}
