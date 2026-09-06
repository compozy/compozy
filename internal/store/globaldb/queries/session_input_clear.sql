-- name: ClearQueuedSessionInput :execrows
UPDATE session_input_queue
SET status = 'canceled', dispatchable = 0, canceled_at = sqlc.arg(now), updated_at = sqlc.arg(now),
    terminal_at = CASE WHEN owner_kind = 'goal' THEN sqlc.arg(now) ELSE terminal_at END,
    terminal_kind = CASE WHEN owner_kind = 'goal' THEN 'control-fenced' ELSE terminal_kind END,
    terminal_disposition = CASE WHEN owner_kind = 'goal' THEN 'paused' ELSE terminal_disposition END,
    terminal_reason_code = CASE WHEN owner_kind = 'goal' THEN 'goal_control_revoked_in_flight' ELSE terminal_reason_code END
WHERE session_id = sqlc.arg(session_id) AND id = sqlc.arg(entry_id) AND status = 'queued';

-- name: RebaseDispatchingSessionInputs :exec
UPDATE session_input_queue SET session_generation = sqlc.arg(generation)
WHERE session_id = sqlc.arg(session_id) AND status = 'dispatching';

-- name: InsertSessionInputClearTrace :exec
INSERT INTO session_input_clear_traces
    (entry_id, session_id, turn_id, actor_kind, actor_id, queue_generation, created_at)
VALUES (sqlc.arg(entry_id), sqlc.arg(session_id), sqlc.arg(turn_id), sqlc.arg(actor_kind),
        sqlc.arg(actor_id), sqlc.arg(queue_generation), sqlc.arg(created_at));

-- name: ListPendingSessionInputClearTraces :many
SELECT * FROM session_input_clear_traces
WHERE session_id = sqlc.arg(session_id) AND projected_at IS NULL
ORDER BY created_at, entry_id;

-- name: MarkSessionInputClearTraceProjected :exec
UPDATE session_input_clear_traces SET projected_at = sqlc.arg(now)
WHERE session_id = sqlc.arg(session_id) AND entry_id = sqlc.arg(entry_id) AND projected_at IS NULL;
