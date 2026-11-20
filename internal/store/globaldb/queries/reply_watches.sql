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

-- name: ListPendingReplyWatchesBySender :many
SELECT * FROM session_prompt_reply_watches
WHERE sender_session_id = ? AND state IN ('armed', 'fired')
ORDER BY created_at, id;

-- name: ListPendingReplyWatchesByTarget :many
SELECT * FROM session_prompt_reply_watches
WHERE target_session_id = ? AND state IN ('armed', 'fired')
ORDER BY created_at, id;

-- name: BindReplyWatch :exec
UPDATE session_prompt_reply_watches SET turn_id = ?, queue_entry_id = ?
WHERE id = ? AND state = 'armed';

-- name: FireReplyWatch :execrows
UPDATE session_prompt_reply_watches
SET state = 'fired', outcome = sqlc.narg(outcome), reply_text = sqlc.narg(reply_text),
 reply_truncated = sqlc.arg(reply_truncated), fired_at = sqlc.narg(fired_at),
 turn_id = COALESCE(sqlc.narg(result_turn_id), turn_id),
 follow_up_after = sqlc.arg(next_after),
 interim_wait = sqlc.narg(interim_wait)
WHERE id = sqlc.arg(id) AND state = 'armed' AND follow_up_after = sqlc.arg(expected_after);

-- name: DeliverReplyWatch :execrows
UPDATE session_prompt_reply_watches SET state = 'delivered', delivered_input_id = ?, delivered_at = ?
WHERE id = ? AND state = 'fired';

-- name: DeliverInterimReplyWatch :execrows
UPDATE session_prompt_reply_watches
SET state = 'armed', interim_wait = NULL, delivered_input_id = ?, delivered_at = ?
WHERE id = ? AND state = 'fired' AND interim_wait IS NOT NULL;

-- name: SupersedeReplyWatchFollowUps :many
UPDATE session_prompt_reply_watches
SET state = CASE WHEN state = 'armed' THEN 'abandoned' ELSE state END,
 abandon_reason = CASE WHEN state = 'armed' THEN 'superseded' ELSE NULL END,
 follow_up_after = CASE WHEN state = 'armed' THEN follow_up_after ELSE 0 END,
 interim_wait = NULL
WHERE follow_up_after > 0
 AND (state = 'armed' OR (state = 'fired' AND interim_wait IS NOT NULL))
 AND EXISTS (
  SELECT 1 FROM session_prompt_reply_watches AS newer
  JOIN session_prompt_admissions AS admission ON admission.id = newer.admission_id AND admission.state = 'completed'
  WHERE (sqlc.arg(newer_id) = '' OR newer.id = sqlc.arg(newer_id))
   AND newer.abandon_reason IS NOT 'send_failed' AND newer.rowid > session_prompt_reply_watches.rowid
   AND newer.sender_session_id = session_prompt_reply_watches.sender_session_id AND newer.target_session_id = session_prompt_reply_watches.target_session_id
   AND newer.workspace_id = session_prompt_reply_watches.workspace_id AND newer.target_workspace_id = session_prompt_reply_watches.target_workspace_id
 )
RETURNING id, state;

-- name: HasNewerReplyWatch :one
SELECT EXISTS (
 SELECT 1 FROM session_prompt_reply_watches AS older
 JOIN session_prompt_reply_watches AS newer
   ON newer.sender_session_id = older.sender_session_id AND newer.target_session_id = older.target_session_id
  AND newer.workspace_id = older.workspace_id AND newer.target_workspace_id = older.target_workspace_id
  AND newer.rowid > older.rowid
 JOIN session_prompt_admissions AS admission ON admission.id = newer.admission_id AND admission.state = 'completed'
 WHERE older.id = sqlc.arg(older_id)
  AND newer.abandon_reason IS NOT 'send_failed'
);

-- name: AbandonReplyWatch :execrows
UPDATE session_prompt_reply_watches SET state = 'abandoned', abandon_reason = sqlc.narg(abandon_reason)
WHERE id = sqlc.arg(id) AND state IN ('armed', 'fired')
 AND (sqlc.narg(abandon_reason) IS NOT 'send_failed' OR (state = 'armed' AND follow_up_after = 0));

-- name: ReplyWatchAdmission :one
SELECT * FROM session_prompt_admissions WHERE id = ?;

-- name: ReplyWatchMessageInputs :many
SELECT * FROM session_input_queue WHERE session_id = ? AND message_id = ? ORDER BY enqueued_at, id;

-- name: ReplyWatchSenderExists :one
SELECT EXISTS(SELECT 1 FROM sessions WHERE id = ? AND workspace_id = ? AND archived_at IS NULL);

-- name: RearmFailedReplyWatch :exec
UPDATE session_prompt_reply_watches SET state = 'armed', abandon_reason = NULL
WHERE admission_id = ? AND state = 'abandoned' AND abandon_reason = 'send_failed';
