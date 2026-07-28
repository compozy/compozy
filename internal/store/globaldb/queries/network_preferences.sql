-- name: UpsertNetworkSubscription :exec
INSERT INTO network_subscriptions (
  workspace_id, channel, thread_id, session_id, mode, created_at, updated_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(thread_id), sqlc.arg(session_id),
  sqlc.arg(mode), sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(workspace_id, channel, thread_id, session_id) DO UPDATE SET
  mode = excluded.mode,
  updated_at = excluded.updated_at;

-- name: DeleteNetworkSubscription :exec
DELETE FROM network_subscriptions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND channel = sqlc.arg(channel)
  AND thread_id = sqlc.arg(thread_id)
  AND session_id = sqlc.arg(session_id);

-- name: UpsertNetworkTaskThreadOrigin :exec
INSERT INTO network_task_thread_origins (
  task_id, workspace_id, channel, thread_id, origin_message_id, digest,
  source_message_ids_json, created_at, updated_at
) VALUES (
  sqlc.arg(task_id), sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(thread_id),
  sqlc.arg(origin_message_id), sqlc.arg(digest), sqlc.arg(source_message_ids_json),
  sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(task_id) DO UPDATE SET
  workspace_id = excluded.workspace_id,
  channel = excluded.channel,
  thread_id = excluded.thread_id,
  origin_message_id = excluded.origin_message_id,
  digest = excluded.digest,
  source_message_ids_json = excluded.source_message_ids_json,
  updated_at = excluded.updated_at;

-- name: UpsertTaskDesignationRollup :exec
INSERT INTO task_designation_rollups (
  designation_group_id, task_id, summary_json, created_at
) VALUES (
  sqlc.arg(designation_group_id), sqlc.arg(task_id), sqlc.arg(summary_json), sqlc.arg(created_at)
)
ON CONFLICT(designation_group_id) DO UPDATE SET
  task_id = excluded.task_id,
  summary_json = excluded.summary_json,
  created_at = excluded.created_at;
