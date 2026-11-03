-- name: DismissSessionAttention :execrows
UPDATE sessions
SET ended_early_turn_id = '', later_returned_at = NULL,
 failure_seen_at = CASE WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') AND failure_seen_at IS NULL THEN sqlc.arg(stamp) ELSE failure_seen_at END,
 last_seen_revision = CASE WHEN last_settled_revision > last_seen_revision THEN attention_revision + 1 ELSE last_seen_revision END,
 last_seen_at = CASE WHEN last_settled_revision > last_seen_revision THEN sqlc.arg(stamp) ELSE last_seen_at END,
 attention_revision = attention_revision + 1, attention_changed_at = sqlc.arg(stamp)
WHERE id = sqlc.arg(id) AND attention_revision = sqlc.arg(revision) AND ended_early_turn_id = sqlc.arg(turn_id)
 AND (CASE WHEN COALESCE(failure_kind, '') <> '' THEN json_array(failure_kind, failure_summary, crash_bundle_path, updated_at) ELSE '' END) = sqlc.arg(failure_identity)
 AND (ended_early_turn_id <> '' OR later_returned_at IS NOT NULL OR
 (state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') AND failure_seen_at IS NULL));
