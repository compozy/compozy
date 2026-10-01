package globaldb

import (
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// Navigator preserves the desktop's needs-you/finished/working/rest order.
const sessionCatalogNavigatorBandExpression = `(CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0
 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(
 CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END)`

const sessionCatalogNavigatorOrderClause = " ORDER BY " + sessionCatalogNavigatorBandExpression +
	" ASC, COALESCE(attention_changed_at, updated_at) DESC, id ASC"

func sessionCatalogNavigatorCursorClause(position store.SessionCatalogPosition, primary string) (string, []any) {
	band := *position.NavigatorBand
	clause := `(` + sessionCatalogNavigatorBandExpression + ` > ? OR (` + sessionCatalogNavigatorBandExpression + ` = ? AND
 (COALESCE(attention_changed_at, updated_at) < ? OR
 (COALESCE(attention_changed_at, updated_at) = ? AND id > ?))))`
	return clause, []any{band, band, primary, primary, strings.TrimSpace(position.ID)}
}
