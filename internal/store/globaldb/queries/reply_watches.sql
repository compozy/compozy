-- name: InsertReplyWatch :exec
INSERT INTO session_prompt_reply_watches (
 id, workspace_id, sender_session_id, target_workspace_id, target_session_id,
 message_id, admission_id, turn_id, queue_entry_id, hop, state, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'armed', ?)
ON CONFLICT(target_session_id, message_id) DO NOTHING;

-- name: GetReplyWatch :one
SELECT * FROM session_prompt_reply_watches WHERE id = ?;

-- name: GetReplyWatchByMessage :one
SELECT * FROM session_prompt_reply_watches WHERE target_session_id = ? AND message_id = ?;

-- name: ListReplyWatches :many
SELECT * FROM session_prompt_reply_watches
WHERE state IN ('armed', 'fired')
 AND (sqlc.arg(state_filter) = '' OR state = sqlc.arg(state_filter))
 AND (sqlc.arg(workspace_filter) = '' OR workspace_id = sqlc.arg(workspace_filter))
 AND (sqlc.arg(sender_filter) = '' OR sender_session_id = sqlc.arg(sender_filter))
 AND (sqlc.arg(target_filter) = '' OR target_session_id = sqlc.arg(target_filter))
ORDER BY created_at, id;

-- name: BindReplyWatch :exec
UPDATE session_prompt_reply_watches SET turn_id = ?, queue_entry_id = ?
WHERE id = ? AND state = 'armed';

-- name: FireReplyWatch :execrows
UPDATE session_prompt_reply_watches
SET state = 'fired', outcome = ?, reply_text = ?, reply_truncated = ?, fired_at = ?
WHERE id = ? AND state = 'armed';

-- name: DeliverReplyWatch :execrows
UPDATE session_prompt_reply_watches SET state = 'delivered', delivered_input_id = ?, delivered_at = ?
WHERE id = ? AND state = 'fired';

-- name: AbandonReplyWatch :exec
UPDATE session_prompt_reply_watches SET state = 'abandoned', abandon_reason = ?
WHERE id = ? AND state IN ('armed', 'fired');

-- name: ReplyWatchAdmission :one
SELECT * FROM session_prompt_admissions WHERE id = ?;

-- name: ReplyWatchMessageInputs :many
SELECT * FROM session_input_queue WHERE session_id = ? AND message_id = ? ORDER BY enqueued_at, id;

-- name: ReplyWatchSenderExists :one
SELECT EXISTS(SELECT 1 FROM sessions WHERE id = ? AND workspace_id = ? AND archived_at IS NULL);

-- name: RearmFailedReplyWatch :exec
UPDATE session_prompt_reply_watches SET state = 'armed', abandon_reason = NULL
WHERE admission_id = ? AND state = 'abandoned' AND abandon_reason = 'send_failed';
