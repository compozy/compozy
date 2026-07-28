-- name: InsertNetworkDirectRoom :execrows
INSERT OR IGNORE INTO network_direct_rooms (
  workspace_id, channel, direct_id, session_a, session_b, opened_at, last_activity_at,
  message_count, open_work_count, last_message_preview
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(direct_id), sqlc.arg(session_a), sqlc.arg(session_b),
  sqlc.arg(opened_at), sqlc.arg(last_activity_at), 0, 0, ''
);

-- name: GetNetworkDirectRoomBySessionPair :one
SELECT workspace_id, channel, direct_id, session_a, session_b,
       opened_at, opened_sequence, last_activity_at, last_activity_sequence,
       message_count, open_work_count, last_message_preview
FROM network_direct_rooms
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND session_a = sqlc.arg(session_a) AND session_b = sqlc.arg(session_b);

-- name: InsertNetworkTimelineMessage :execresult
INSERT INTO network_timeline_log (
  message_id, session_id, workspace_id, channel, surface, thread_id, direct_id, direction,
  peer_from, peer_to, kind, work_id, reply_to, trace_id, causation_id, intent, text, preview_text,
  mentions_json, ext_json, body_json, timestamp
) VALUES (
  sqlc.arg(message_id), sqlc.narg(session_id), sqlc.arg(workspace_id), sqlc.arg(channel),
  sqlc.narg(surface), sqlc.narg(thread_id), sqlc.narg(direct_id), sqlc.arg(direction),
  sqlc.arg(peer_from), sqlc.narg(peer_to), sqlc.arg(kind), sqlc.narg(work_id), sqlc.narg(reply_to),
  sqlc.narg(trace_id), sqlc.narg(causation_id), sqlc.narg(intent), sqlc.narg(text),
  sqlc.arg(preview_text), sqlc.arg(mentions_json), sqlc.arg(ext_json), sqlc.arg(body_json),
  sqlc.arg(timestamp)
)
ON CONFLICT(workspace_id, message_id) DO NOTHING;

-- name: GetNetworkMessageTimestamp :one
SELECT timestamp FROM network_timeline_log
WHERE workspace_id = sqlc.arg(workspace_id) AND message_id = sqlc.arg(message_id);

-- name: InsertNetworkThread :execrows
INSERT OR IGNORE INTO network_threads (
  workspace_id, channel, thread_id, root_message_id, title, opened_by_peer_id, opened_session_id,
  opened_at, opened_sequence, last_activity_at, last_activity_sequence,
  message_count, participant_count, open_work_count, last_message_preview
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(thread_id), sqlc.arg(root_message_id),
  sqlc.arg(title), sqlc.arg(opened_by_peer_id), sqlc.arg(opened_session_id), sqlc.arg(opened_at),
  sqlc.arg(opened_sequence), sqlc.arg(last_activity_at), sqlc.arg(last_activity_sequence), 0, 0, 0, ''
);

-- name: InitializeNetworkDirectRoomSequence :execrows
UPDATE network_direct_rooms
SET opened_sequence = sqlc.arg(opened_sequence), last_activity_sequence = sqlc.arg(last_activity_sequence)
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND direct_id = sqlc.arg(direct_id) AND opened_sequence = 0;

-- name: UpsertNetworkThreadParticipant :exec
INSERT INTO network_thread_participants (
  workspace_id, channel, thread_id, session_id, first_message_id, first_seen_at, last_seen_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(thread_id), sqlc.arg(session_id),
  sqlc.arg(first_message_id), sqlc.arg(first_seen_at), sqlc.arg(last_seen_at)
)
ON CONFLICT(workspace_id, channel, thread_id, session_id) DO UPDATE SET
  last_seen_at = excluded.last_seen_at;

-- name: CountNetworkThreadMessages :one
SELECT COUNT(*) FROM network_timeline_log
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND surface = 'thread' AND thread_id = sqlc.arg(thread_id);

-- name: CountNetworkThreadParticipants :one
SELECT COUNT(*) FROM network_thread_participants
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND thread_id = sqlc.arg(thread_id);

-- name: UpdateNetworkThreadSummary :exec
UPDATE network_threads
SET last_activity_at = sqlc.arg(last_activity_at),
    last_activity_sequence = sqlc.arg(last_activity_sequence),
    message_count = sqlc.arg(message_count),
    participant_count = sqlc.arg(participant_count),
    open_work_count = sqlc.arg(open_work_count),
    last_message_preview = sqlc.arg(last_message_preview)
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND thread_id = sqlc.arg(thread_id);

-- name: CountNetworkDirectMessages :one
SELECT COUNT(*) FROM network_timeline_log
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND surface = 'direct' AND direct_id = sqlc.arg(direct_id);

-- name: UpdateNetworkDirectRoomSummary :exec
UPDATE network_direct_rooms
SET last_activity_at = sqlc.arg(last_activity_at),
    last_activity_sequence = sqlc.arg(last_activity_sequence),
    message_count = sqlc.arg(message_count),
    open_work_count = sqlc.arg(open_work_count),
    last_message_preview = sqlc.arg(last_message_preview)
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND direct_id = sqlc.arg(direct_id);

-- name: GetLatestNetworkThreadMessage :one
SELECT sequence, timestamp, preview_text
FROM network_timeline_log
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND surface = 'thread' AND thread_id = sqlc.arg(thread_id)
ORDER BY sequence DESC LIMIT 1;

-- name: GetLatestNetworkDirectMessage :one
SELECT sequence, timestamp, preview_text
FROM network_timeline_log
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND surface = 'direct' AND direct_id = sqlc.arg(direct_id)
ORDER BY sequence DESC LIMIT 1;

-- name: CountOpenNetworkThreadWork :one
SELECT COUNT(*) FROM network_work
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND surface = 'thread' AND thread_id = sqlc.arg(thread_id)
  AND state NOT IN (sqlc.arg(completed), sqlc.arg(failed), sqlc.arg(canceled));

-- name: CountOpenNetworkDirectWork :one
SELECT COUNT(*) FROM network_work
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND surface = 'direct' AND direct_id = sqlc.arg(direct_id)
  AND state NOT IN (sqlc.arg(completed), sqlc.arg(failed), sqlc.arg(canceled));

-- name: UpdateNetworkWork :exec
UPDATE network_work
SET state = sqlc.arg(state), last_activity_at = sqlc.arg(last_activity_at), terminal_at = sqlc.narg(terminal_at)
WHERE workspace_id = sqlc.arg(workspace_id) AND work_id = sqlc.arg(work_id);

-- name: InsertNetworkWork :exec
INSERT INTO network_work (
  work_id, workspace_id, channel, surface, thread_id, direct_id, opened_by_session_id,
  target_session_id, state, opened_at, last_activity_at, terminal_at
) VALUES (
  sqlc.arg(work_id), sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(surface),
  sqlc.narg(thread_id), sqlc.narg(direct_id), sqlc.arg(opened_by_session_id),
  sqlc.narg(target_session_id), sqlc.arg(state),
  sqlc.arg(opened_at), sqlc.arg(last_activity_at), NULL
);

-- name: GetNetworkWork :one
SELECT work_id, workspace_id, channel, surface, thread_id, direct_id, opened_by_session_id,
       target_session_id, state, opened_at, last_activity_at, terminal_at
FROM network_work
WHERE workspace_id = sqlc.arg(workspace_id) AND work_id = sqlc.arg(work_id);

-- name: GetPriorNetworkTimelineWorkState :one
SELECT COALESCE((
  SELECT prior.work_state
  FROM network_timeline_log prior
  WHERE prior.workspace_id = current.workspace_id
    AND prior.work_id = current.work_id
    AND prior.sequence < current.sequence
    AND prior.work_state <> ''
  ORDER BY prior.sequence DESC LIMIT 1
), '')
FROM network_timeline_log current
WHERE current.workspace_id = sqlc.arg(workspace_id) AND current.message_id = sqlc.arg(message_id);

-- name: UpdateNetworkTimelineWorkProjection :exec
UPDATE network_timeline_log
SET work_opened = sqlc.arg(work_opened), work_transitioned = sqlc.arg(work_transitioned),
    work_state = sqlc.arg(work_state)
WHERE workspace_id = sqlc.arg(workspace_id) AND message_id = sqlc.arg(message_id);

-- name: GetNetworkCursorMessageID :one
SELECT message_id FROM network_timeline_log
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND surface = sqlc.arg(surface) AND sequence = sqlc.arg(sequence)
  AND ((sqlc.arg(surface) = 'thread' AND thread_id = sqlc.arg(container_id))
    OR (sqlc.arg(surface) = 'direct' AND direct_id = sqlc.arg(container_id)));

-- name: GetNetworkCursorSequence :one
SELECT sequence FROM network_timeline_log
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
  AND surface = sqlc.arg(surface) AND message_id = sqlc.arg(message_id)
  AND ((sqlc.arg(surface) = 'thread' AND thread_id = sqlc.arg(container_id))
    OR (sqlc.arg(surface) = 'direct' AND direct_id = sqlc.arg(container_id)));
