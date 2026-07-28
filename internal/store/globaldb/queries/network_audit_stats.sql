-- name: InsertNetworkAudit :exec
INSERT INTO network_audit_log (
  id, session_id, workspace_id, direction, kind, channel, surface, thread_id, direct_id, work_id,
  peer_from, peer_to, message_id, reason, size, timestamp
) VALUES (
  sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(direction), sqlc.arg(kind),
  sqlc.arg(channel), sqlc.narg(surface), sqlc.narg(thread_id), sqlc.narg(direct_id), sqlc.narg(work_id),
  sqlc.arg(peer_from), sqlc.narg(peer_to), sqlc.arg(message_id), sqlc.narg(reason), sqlc.arg(size),
  sqlc.arg(timestamp)
);

-- name: ListNetworkThreadParticipants :many
SELECT workspace_id, channel, thread_id, session_id, first_message_id, first_seen_at, last_seen_at
FROM network_thread_participants
WHERE workspace_id = sqlc.arg(workspace_id)
  AND channel = sqlc.arg(channel)
  AND thread_id = sqlc.arg(thread_id)
ORDER BY last_seen_at DESC, session_id ASC;

-- name: UpsertNetworkThreadSessionTokenStats :exec
INSERT INTO network_thread_session_token_stats (
  workspace_id, channel, thread_id, session_id, delivered_count, prompt_size_bytes,
  estimated_prompt_tokens, first_delivered_at, last_delivered_at, updated_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(thread_id), sqlc.arg(session_id),
  sqlc.arg(delivered_count), sqlc.arg(prompt_size_bytes), sqlc.arg(estimated_prompt_tokens),
  sqlc.arg(first_delivered_at), sqlc.arg(last_delivered_at), sqlc.arg(updated_at)
)
ON CONFLICT(workspace_id, channel, thread_id, session_id) DO UPDATE SET
  delivered_count = network_thread_session_token_stats.delivered_count + excluded.delivered_count,
  prompt_size_bytes = network_thread_session_token_stats.prompt_size_bytes + excluded.prompt_size_bytes,
  estimated_prompt_tokens =
    network_thread_session_token_stats.estimated_prompt_tokens + excluded.estimated_prompt_tokens,
  first_delivered_at = CASE
    WHEN network_thread_session_token_stats.first_delivered_at <= excluded.first_delivered_at
      THEN network_thread_session_token_stats.first_delivered_at
    ELSE excluded.first_delivered_at
  END,
  last_delivered_at = CASE
    WHEN network_thread_session_token_stats.last_delivered_at >= excluded.last_delivered_at
      THEN network_thread_session_token_stats.last_delivered_at
    ELSE excluded.last_delivered_at
  END,
  updated_at = excluded.updated_at;
