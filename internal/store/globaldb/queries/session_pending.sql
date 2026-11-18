-- name: HasPermissionOccurrence :one
SELECT EXISTS (
 SELECT 1 FROM session_pending_interactions active
 WHERE active.session_id = sqlc.arg(session_id) AND active.kind = 'permission'
 AND active.provider_request_id = sqlc.arg(request_id) AND active.turn_id = sqlc.arg(turn_id)
 AND active.status IN ('pending', 'orphaned')
 UNION ALL
 SELECT 1 FROM session_pending_interactions settled
 WHERE settled.session_id = sqlc.arg(session_id) AND settled.kind = 'permission'
 AND settled.provider_request_id = sqlc.arg(request_id) AND settled.turn_id = sqlc.arg(turn_id)
 AND settled.status IN ('resolved', 'timed_out', 'canceled') AND settled.created_at = sqlc.arg(created_at)
);
