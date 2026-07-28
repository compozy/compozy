-- name: GetNetworkThread :one
SELECT
  workspace_id, channel, thread_id, root_message_id, title, opened_by_peer_id, opened_session_id,
  opened_at, opened_sequence, last_activity_at, last_activity_sequence,
  message_count, participant_count, open_work_count,
  CAST(COALESCE((
    SELECT SUM(stats.delivered_count)
    FROM network_thread_session_token_stats AS stats
    WHERE stats.workspace_id = network_threads.workspace_id
      AND stats.channel = network_threads.channel
      AND stats.thread_id = network_threads.thread_id
  ), 0) AS INTEGER) AS delivered_count,
  CAST(COALESCE((
    SELECT SUM(stats.prompt_size_bytes)
    FROM network_thread_session_token_stats AS stats
    WHERE stats.workspace_id = network_threads.workspace_id
      AND stats.channel = network_threads.channel
      AND stats.thread_id = network_threads.thread_id
  ), 0) AS INTEGER) AS prompt_size_bytes,
  CAST(COALESCE((
    SELECT SUM(stats.estimated_prompt_tokens)
    FROM network_thread_session_token_stats AS stats
    WHERE stats.workspace_id = network_threads.workspace_id
      AND stats.channel = network_threads.channel
      AND stats.thread_id = network_threads.thread_id
  ), 0) AS INTEGER) AS estimated_prompt_tokens,
  last_message_preview
FROM network_threads
WHERE network_threads.workspace_id = sqlc.arg(workspace_id)
  AND network_threads.channel = sqlc.arg(channel)
  AND network_threads.thread_id = sqlc.arg(thread_id);

-- name: GetNetworkDirectRoom :one
SELECT workspace_id, channel, direct_id, session_a, session_b,
       opened_at, opened_sequence, last_activity_at, last_activity_sequence,
       message_count, open_work_count, last_message_preview
FROM network_direct_rooms
WHERE network_direct_rooms.workspace_id = sqlc.arg(workspace_id)
  AND network_direct_rooms.channel = sqlc.arg(channel)
  AND network_direct_rooms.direct_id = sqlc.arg(direct_id);

-- name: ListNetworkRecents :many
SELECT
	recents.workspace_id, recents.channel, recents.surface, recents.container_id,
	recents.last_activity_at, recents.last_activity_sequence, recents.last_message_preview,
	recents.title, recents.participant_count, recents.session_a, recents.session_b
FROM (
	SELECT
		network_threads.workspace_id, network_threads.channel, 'thread' AS surface,
		network_threads.thread_id AS container_id, network_threads.last_activity_at,
		network_threads.last_activity_sequence, network_threads.last_message_preview,
		network_threads.title, network_threads.participant_count,
		'' AS session_a, '' AS session_b
	FROM network_threads
	WHERE network_threads.workspace_id = sqlc.arg(workspace_id)
	UNION ALL
	SELECT
		network_direct_rooms.workspace_id, network_direct_rooms.channel, 'direct' AS surface,
		network_direct_rooms.direct_id AS container_id, network_direct_rooms.last_activity_at,
		network_direct_rooms.last_activity_sequence, network_direct_rooms.last_message_preview,
		'' AS title, 0 AS participant_count, network_direct_rooms.session_a, network_direct_rooms.session_b
	FROM network_direct_rooms
	WHERE network_direct_rooms.workspace_id = sqlc.arg(workspace_id)
) AS recents
ORDER BY recents.last_activity_sequence DESC, recents.surface ASC, recents.container_id ASC
LIMIT sqlc.arg(row_limit);
