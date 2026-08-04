-- name: CancelPriorSessionSteers :exec
UPDATE session_input_queue
SET status = sqlc.arg(canceled_status), canceled_at = sqlc.narg(canceled_at), updated_at = sqlc.arg(updated_at)
WHERE session_id = sqlc.arg(session_id)
  AND mode = sqlc.arg(steer_mode)
  AND status = sqlc.arg(queued_status);

-- name: GetNextDispatchableSessionInput :one
SELECT * FROM session_input_queue
WHERE session_id = sqlc.arg(session_id)
  AND status = sqlc.arg(queued_status)
  AND dispatchable = 1
  AND owner_kind IS NULL
  AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
ORDER BY delivery DESC, enqueued_at ASC, id ASC
LIMIT 1;

-- name: ClaimSessionInput :execrows
UPDATE session_input_queue
SET status = sqlc.arg(dispatching_status), dispatch_started_at = sqlc.narg(now),
    attempt_count = attempt_count + 1, updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id)
  AND status = sqlc.arg(queued_status) AND dispatchable = 1 AND owner_kind IS NULL;

-- name: PeekNextSessionInput :one
SELECT * FROM session_input_queue
WHERE session_id = sqlc.arg(session_id)
  AND status = sqlc.arg(queued_status)
  AND terminal_at IS NULL
  AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  AND ((owner_kind IS NULL AND dispatchable = 1)
       OR (owner_kind = 'goal' AND dispatchable = 0 AND fence_kind IS NULL))
ORDER BY delivery DESC, enqueued_at ASC, id ASC
LIMIT 1;

-- name: ListPendingSessionInputs :many
SELECT * FROM session_input_queue
WHERE session_id = sqlc.arg(session_id)
  AND status IN (sqlc.arg(queued_status), sqlc.arg(dispatching_status))
  AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  AND owner_kind IS NULL
ORDER BY delivery DESC, enqueued_at ASC, id ASC;

-- name: GetSessionInputQueueEntry :one
SELECT * FROM session_input_queue
WHERE session_id = sqlc.arg(session_id) AND id = sqlc.arg(id);

-- name: GetSessionInputQueueEntryByID :one
SELECT * FROM session_input_queue WHERE id = sqlc.arg(id);

-- name: AdvanceSessionInputGeneration :execrows
UPDATE sessions SET input_generation = input_generation + 1, updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: GetSessionInputGeneration :one
SELECT input_generation FROM sessions WHERE id = sqlc.arg(id);

-- name: ListSessionInputQueueSummary :many
SELECT mode, status, COUNT(*) AS count
FROM session_input_queue
WHERE session_id = sqlc.arg(session_id)
  AND session_generation = sqlc.arg(session_generation)
  AND status IN (sqlc.arg(queued_status), sqlc.arg(dispatching_status))
GROUP BY mode, status;

-- name: InsertSessionInputQueueEntry :exec
INSERT INTO session_input_queue (
  id, session_id, prompt_admission_id, message_id, idempotency_key, turn_id, target_turn_id, event_id,
  status, mode, delivery, text,
  runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed,
  session_generation, task_run_id, run_generation,
  attempt_count, enqueued_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(session_id), sqlc.narg(prompt_admission_id),
  sqlc.arg(message_id), sqlc.arg(idempotency_key), sqlc.arg(turn_id), sqlc.arg(target_turn_id), sqlc.arg(event_id),
  sqlc.arg(status), sqlc.arg(mode), sqlc.arg(delivery), sqlc.arg(text),
  sqlc.arg(runtime_provider), sqlc.arg(runtime_model),
  sqlc.arg(runtime_reasoning_effort), sqlc.arg(runtime_speed),
  sqlc.arg(session_generation), sqlc.arg(task_run_id), sqlc.narg(run_generation),
  0, sqlc.arg(enqueued_at), sqlc.arg(updated_at)
);

-- name: CountPendingSessionInputs :one
SELECT COUNT(*) FROM session_input_queue
WHERE session_id = sqlc.arg(session_id)
  AND status IN (sqlc.arg(queued_status), sqlc.arg(dispatching_status));

-- name: ReleaseSessionInput :execrows
UPDATE session_input_queue
SET status = sqlc.arg(queued_status), dispatch_started_at = NULL, updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id)
  AND status = sqlc.arg(dispatching_status);

-- name: MarkSessionInputSent :execrows
UPDATE session_input_queue
SET status = sqlc.arg(sent_status), sent_at = sqlc.narg(now), failure_summary = '', updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id)
  AND status = sqlc.arg(dispatching_status);

-- name: MarkSessionInputFailed :execrows
UPDATE session_input_queue
SET status = sqlc.arg(failed_status), failed_at = sqlc.narg(now),
    failure_summary = sqlc.arg(failure_summary), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id)
  AND status = sqlc.arg(dispatching_status);

-- name: CancelSessionInput :exec
UPDATE session_input_queue
SET status = sqlc.arg(canceled_status), canceled_at = sqlc.narg(now), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id);

-- name: CancelPendingSessionInputs :execrows
UPDATE session_input_queue
SET status = sqlc.arg(canceled_status), canceled_at = sqlc.narg(now), updated_at = sqlc.arg(updated_at)
WHERE session_id = sqlc.arg(session_id)
  AND session_generation < sqlc.arg(session_generation)
  AND status IN (sqlc.arg(queued_status), sqlc.arg(dispatching_status));
