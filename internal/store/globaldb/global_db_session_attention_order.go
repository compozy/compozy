package globaldb

import (
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// The numeric branches match store.SessionCatalogAttentionRank. Needs-you rows
// lead, followed by unseen finished rows and then sessions with no attention.
const sessionCatalogAttentionRankExpression = `(CASE
	WHEN pending_permission_count > 0
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
