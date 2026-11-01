-- name: CancelPriorSessionSteers :many
UPDATE session_input_queue
SET status = sqlc.arg(canceled_status), canceled_at = sqlc.narg(canceled_at), updated_at = sqlc.arg(updated_at)
WHERE session_id = sqlc.arg(session_id)
  AND mode = sqlc.arg(steer_mode)
  AND status = sqlc.arg(queued_status)
  AND origin_session_id = sqlc.arg(origin_session_id)
RETURNING id;

-- name: GetNextDispatchableSessionInput :one
WITH prepared_goal AS (
  SELECT goal_input_batch_max_rowid AS cutoff FROM session_input_queue
  WHERE session_id = sqlc.arg(session_id) AND owner_kind = 'goal'
    AND status = sqlc.arg(queued_status) AND dispatchable = 0
    AND terminal_at IS NULL AND fence_kind IS NULL
    AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  ORDER BY enqueued_at, id LIMIT 1
)
SELECT queued.* FROM session_input_queue AS queued
WHERE queued.session_id = sqlc.arg(session_id)
  AND queued.status = sqlc.arg(queued_status)
  AND queued.dispatchable = 1
  AND (queued.owner_kind IS NULL OR queued.owner_kind = 'synthetic')
  AND queued.session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id))
  AND ((SELECT cutoff FROM prepared_goal) IS NULL
       OR queued.rowid <= (SELECT cutoff FROM prepared_goal)
       OR (queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human'))
ORDER BY CASE
           WHEN queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human' THEN 0
           WHEN queued.owner_kind = 'goal' THEN 2
           WHEN (SELECT cutoff FROM prepared_goal) IS NULL
             OR queued.rowid <= (SELECT cutoff FROM prepared_goal) THEN 1
           ELSE 3
         END,
         CASE queued.origin_class WHEN 'human' THEN 0 WHEN 'agent' THEN 1 ELSE 2 END,
         queued.delivery DESC, queued.priority DESC, queued.enqueued_at ASC, queued.id ASC
LIMIT 1;

-- name: FreezePreparedGoalInputBatch :exec
UPDATE session_input_queue
SET goal_input_batch_max_rowid = (
  SELECT COALESCE(MAX(input.rowid), 0) FROM session_input_queue AS input
  WHERE input.session_id = sqlc.arg(session_id)
    AND input.session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
    AND input.status = sqlc.arg(queued_status) AND input.dispatchable = 1
    AND input.terminal_at IS NULL
    AND (input.owner_kind IS NULL OR input.owner_kind = 'synthetic')
)
WHERE session_input_queue.id = (
  SELECT id FROM session_input_queue
  WHERE session_id = sqlc.arg(session_id) AND owner_kind = 'goal'
    AND status = sqlc.arg(queued_status) AND dispatchable = 0
    AND terminal_at IS NULL AND fence_kind IS NULL
    AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  ORDER BY enqueued_at, id LIMIT 1
) AND session_input_queue.goal_input_batch_max_rowid IS NULL;

-- name: ClaimSessionInput :execrows
UPDATE session_input_queue
SET status = sqlc.arg(dispatching_status), dispatch_started_at = sqlc.narg(now),
    attempt_count = attempt_count + 1, updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id)
  AND status = sqlc.arg(queued_status) AND dispatchable = 1 AND (owner_kind IS NULL OR owner_kind = 'synthetic');

-- name: PeekNextSessionInput :one
WITH prepared_goal AS (
  SELECT goal_input_batch_max_rowid AS cutoff FROM session_input_queue
  WHERE session_id = sqlc.arg(session_id) AND owner_kind = 'goal'
    AND status = sqlc.arg(queued_status) AND dispatchable = 0
    AND terminal_at IS NULL AND fence_kind IS NULL
    AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  ORDER BY enqueued_at, id LIMIT 1
)
SELECT queued.* FROM session_input_queue AS queued
WHERE queued.session_id = sqlc.arg(session_id)
  AND queued.status = sqlc.arg(queued_status)
  AND queued.terminal_at IS NULL
  AND queued.session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id))
  AND (((queued.owner_kind IS NULL OR queued.owner_kind = 'synthetic') AND queued.dispatchable = 1)
       OR (owner_kind = 'goal' AND queued.dispatchable = 0 AND queued.fence_kind IS NULL))
ORDER BY CASE
           WHEN queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human' THEN 0
           WHEN queued.owner_kind = 'goal' THEN 2
           WHEN (SELECT cutoff FROM prepared_goal) IS NULL
             OR queued.rowid <= (SELECT cutoff FROM prepared_goal) THEN 1
           ELSE 3
         END,
         CASE queued.origin_class WHEN 'human' THEN 0 WHEN 'agent' THEN 1 ELSE 2 END,
         queued.delivery DESC, queued.priority DESC, queued.enqueued_at ASC, queued.id ASC
LIMIT 1;

-- name: ListPendingSessionInputs :many
WITH prepared_goal AS (
  SELECT goal_input_batch_max_rowid AS cutoff FROM session_input_queue
  WHERE session_id = sqlc.arg(session_id) AND owner_kind = 'goal'
    AND status = sqlc.arg(queued_status) AND dispatchable = 0
    AND terminal_at IS NULL AND fence_kind IS NULL
    AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  ORDER BY enqueued_at, id LIMIT 1
)
SELECT queued.* FROM session_input_queue AS queued
WHERE queued.session_id = sqlc.arg(session_id)
  AND (queued.owner_kind IS NULL OR queued.owner_kind != 'goal')
  AND queued.status IN (sqlc.arg(queued_status), sqlc.arg(dispatching_status))
  AND queued.session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id))
ORDER BY CASE
           WHEN queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human' THEN 0
           WHEN queued.owner_kind = 'goal' THEN 2
           WHEN (SELECT cutoff FROM prepared_goal) IS NULL
             OR queued.rowid <= (SELECT cutoff FROM prepared_goal) THEN 1
           ELSE 3
         END,
         CASE queued.origin_class WHEN 'human' THEN 0 WHEN 'agent' THEN 1 ELSE 2 END,
         queued.delivery DESC, queued.priority DESC, queued.enqueued_at ASC, queued.id ASC;

-- name: GetSessionInputQueueEntry :one
SELECT * FROM session_input_queue
WHERE session_id = sqlc.arg(session_id) AND id = sqlc.arg(id);

-- name: GetSessionInputQueueEntryByID :one
SELECT * FROM session_input_queue WHERE id = sqlc.arg(id);

-- name: ReserveSessionSteer :execrows
UPDATE session_input_queue
SET status = 'dispatching', dispatch_started_at = sqlc.arg(now),
    attempt_count = attempt_count + 1, updated_at = sqlc.arg(now)
WHERE session_input_queue.session_id = sqlc.arg(session_id) AND session_input_queue.id = sqlc.arg(id)
  AND mode = 'steer' AND status = 'queued' AND owner_kind IS NULL
  AND session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id));

-- name: ResolveSessionSteer :execrows
UPDATE session_input_queue
SET status = sqlc.arg(status), steer_delivery = sqlc.arg(steer_delivery),
    dispatchable = CASE WHEN sqlc.arg(status) = 'queued' THEN 1 ELSE 0 END,
    delivery = CASE WHEN sqlc.arg(status) = 'queued' AND origin_class <> 'human' THEN 'after_turn' ELSE delivery END,
    turn_id = CASE WHEN sqlc.arg(status) = 'sent' THEN target_turn_id ELSE turn_id END,
    sent_at = sqlc.narg(sent_at), dispatch_started_at = NULL, updated_at = sqlc.arg(now)
WHERE session_id = sqlc.arg(session_id) AND id = sqlc.arg(id)
  AND mode = 'steer' AND status = 'dispatching' AND owner_kind IS NULL;

-- name: CancelPendingSessionSteer :execrows
UPDATE session_input_queue
SET status = 'canceled', dispatchable = 0, canceled_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE session_id = sqlc.arg(session_id) AND id = sqlc.arg(id)
  AND mode = 'steer' AND status = 'sent' AND steer_delivery = 'pending_injection'
  AND owner_kind IS NULL;

-- name: SettlePendingSessionSteer :execrows
UPDATE session_input_queue
SET steer_delivery = sqlc.arg(steer_delivery), updated_at = sqlc.arg(now),
    status = CASE WHEN sqlc.arg(steer_delivery) = 'interrupt_fallback' THEN 'queued' ELSE 'sent' END,
    dispatchable = CASE WHEN sqlc.arg(steer_delivery) = 'interrupt_fallback' THEN 1 ELSE 0 END,
    delivery = CASE WHEN sqlc.arg(steer_delivery) = 'interrupt_fallback' AND origin_class <> 'human'
                    THEN 'after_turn' ELSE delivery END,
    sent_at = CASE WHEN sqlc.arg(steer_delivery) = 'interrupt_fallback' THEN NULL ELSE sent_at END,
    turn_id = CASE WHEN sqlc.arg(steer_delivery) = 'interrupt_fallback' THEN sqlc.arg(fallback_turn_id) ELSE turn_id END
WHERE session_input_queue.session_id = sqlc.arg(session_id) AND session_input_queue.id = sqlc.arg(id)
  AND mode = 'steer' AND status = 'sent' AND steer_delivery = 'pending_injection'
  AND owner_kind IS NULL
  AND session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id));

-- name: AdvanceSessionInputGeneration :execrows
UPDATE sessions SET input_generation = input_generation + 1, updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: GetSessionInputGeneration :one
SELECT input_generation FROM sessions WHERE id = sqlc.arg(id);

-- name: ListSessionInputQueueSummary :many
SELECT mode, status, COUNT(*) AS count
FROM session_input_queue
WHERE session_id = sqlc.arg(session_id)
  AND (owner_kind IS NULL OR owner_kind != 'goal')
  AND session_generation = sqlc.arg(session_generation)
  AND status IN (sqlc.arg(queued_status), sqlc.arg(dispatching_status))
GROUP BY mode, status;

-- name: InsertSessionInputQueueEntry :exec
INSERT INTO session_input_queue (
  priority, id, session_id, prompt_admission_id, message_id, idempotency_key, turn_id, target_turn_id, event_id,
  status, mode, delivery, steer_delivery, text, owner_kind, synthetic_prompt_json, skill_invocations_json, mentions_json, attachments_json, origin_json,
  runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed, runtime_acp_options_json,
  session_generation, task_run_id, run_generation,
  attempt_count, enqueued_at, updated_at, dispatchable, origin_class, origin_session_id, human_ingress
) VALUES (
  sqlc.arg(priority), sqlc.arg(id), sqlc.arg(session_id), sqlc.narg(prompt_admission_id),
  sqlc.arg(message_id), sqlc.arg(idempotency_key), sqlc.arg(turn_id), sqlc.arg(target_turn_id), sqlc.arg(event_id),
  sqlc.arg(status), sqlc.arg(mode), sqlc.arg(delivery), sqlc.narg(steer_delivery), sqlc.arg(text), sqlc.narg(owner_kind), sqlc.narg(synthetic_prompt_json), sqlc.arg(skill_invocations_json), sqlc.arg(mentions_json),
  sqlc.arg(attachments_json), sqlc.narg(origin_json),
  sqlc.arg(runtime_provider), sqlc.arg(runtime_model),
  sqlc.arg(runtime_reasoning_effort), sqlc.arg(runtime_speed), sqlc.arg(runtime_acp_options_json),
  sqlc.arg(session_generation), sqlc.arg(task_run_id), sqlc.narg(run_generation),
  0, sqlc.arg(enqueued_at), sqlc.arg(updated_at),
  CASE WHEN sqlc.arg(mode) = 'steer' AND sqlc.narg(steer_delivery) IS NULL THEN 0 ELSE 1 END,
  sqlc.arg(origin_class), sqlc.arg(origin_session_id), sqlc.arg(human_ingress)
);

-- name: CountPendingSessionInputs :one
-- Human input and agent/synthetic input hold separate capacity pools.
SELECT COUNT(*) FROM session_input_queue
WHERE session_id = sqlc.arg(session_id)
  AND (owner_kind IS NULL OR owner_kind != 'goal')
  AND status IN (sqlc.arg(queued_status), sqlc.arg(dispatching_status))
  AND (origin_class = 'human') = (CAST(sqlc.arg(pool_class) AS TEXT) = 'human');

-- name: ReleaseSessionInput :execrows
UPDATE session_input_queue
SET status = sqlc.arg(queued_status), dispatch_started_at = NULL, updated_at = sqlc.arg(updated_at)
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

-- name: GetSyntheticSessionInputByMessage :one
SELECT * FROM session_input_queue WHERE session_id = ? AND message_id = ?
 AND owner_kind = 'synthetic' AND message_id <> '' AND status IN ('queued','dispatching');

-- name: SyntheticSessionInputAdmitted :one
-- A synthetic input counts as admitted once it was queued, is dispatching, or was sent; a failed or
-- canceled one was never delivered and may be admitted again.
SELECT EXISTS(
  SELECT 1 FROM session_input_queue WHERE session_id = ? AND message_id = ?
   AND owner_kind = 'synthetic' AND message_id <> '' AND status IN ('queued','dispatching','sent')
);

-- name: ListStaleDispatchingSessionInputs :many
-- Boot recovery: no live turn can own a non-Goal lease left by a previous daemon process.
SELECT session_id, id FROM session_input_queue
WHERE status = 'dispatching' AND (owner_kind IS NULL OR owner_kind = 'synthetic')
ORDER BY session_id, enqueued_at, id;

-- name: RecoverStagedSessionSteers :execrows
-- Boot recovery: a steer staged by a previous daemon process never reached its turn. Steers from other
-- sessions (including long-turn promotions) return to after-turn delivery; operator steers become
-- dispatchable again so the ordinary claim delivers them.
UPDATE session_input_queue
SET mode = CASE WHEN origin_class = 'human' THEN mode ELSE 'queue' END,
    delivery = CASE WHEN origin_class = 'human' THEN delivery ELSE 'after_turn' END,
    target_turn_id = CASE WHEN origin_class = 'human' THEN target_turn_id ELSE '' END,
    dispatchable = 1, updated_at = sqlc.arg(now)
WHERE status = 'queued' AND mode = 'steer' AND steer_delivery IS NULL AND dispatchable = 0
  AND owner_kind IS NULL
  AND session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = session_input_queue.session_id);

-- name: CancelSupersededAgentSessionInputs :many
-- Supersede: one sender replaces its own older undispatched messages to this target.
UPDATE session_input_queue
SET status = 'canceled', dispatchable = 0, canceled_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE session_id = sqlc.arg(session_id)
  AND origin_class = 'agent'
  AND origin_session_id = sqlc.arg(origin_session_id)
  AND owner_kind IS NULL
  AND status = 'queued'
  AND dispatchable = 1
  AND delivery = 'after_turn'
RETURNING id;

-- name: ListDispatchableSessionInputs :many
-- Dispatch-ordered candidates for batching behind an already claimed head; same filter as
-- GetNextDispatchableSessionInput, so the Goal cutoff still bounds the batch.
WITH prepared_goal AS (
  SELECT goal_input_batch_max_rowid AS cutoff FROM session_input_queue
  WHERE session_id = sqlc.arg(session_id) AND owner_kind = 'goal'
    AND status = 'queued' AND dispatchable = 0
    AND terminal_at IS NULL AND fence_kind IS NULL
    AND session_generation = (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id))
  ORDER BY enqueued_at, id LIMIT 1
)
SELECT queued.* FROM session_input_queue AS queued
WHERE queued.session_id = sqlc.arg(session_id)
  AND queued.status = 'queued'
  AND queued.dispatchable = 1
  AND (queued.owner_kind IS NULL OR queued.owner_kind = 'synthetic')
  AND queued.session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id))
  AND ((SELECT cutoff FROM prepared_goal) IS NULL
       OR queued.rowid <= (SELECT cutoff FROM prepared_goal)
       OR (queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human'))
ORDER BY CASE
           WHEN queued.delivery = 'interrupt_then_prompt' AND queued.origin_class = 'human' THEN 0
           WHEN (SELECT cutoff FROM prepared_goal) IS NULL
             OR queued.rowid <= (SELECT cutoff FROM prepared_goal) THEN 1
           ELSE 3
         END,
         CASE queued.origin_class WHEN 'human' THEN 0 WHEN 'agent' THEN 1 ELSE 2 END,
         queued.delivery DESC, queued.priority DESC, queued.enqueued_at ASC, queued.id ASC
LIMIT sqlc.arg(row_limit);

-- name: MarkSessionInputSentInTurn :execrows
UPDATE session_input_queue
SET status = 'sent', sent_at = sqlc.arg(now), failure_summary = '', updated_at = sqlc.arg(now),
    turn_id = COALESCE(NULLIF(CAST(sqlc.arg(turn_id) AS TEXT), ''), turn_id)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id) AND status = 'dispatching';

-- name: PromoteQueuedAgentSessionInputToSteer :execrows
-- Long-turn promotion keeps the row identity (message, event, admission, origin) so reply watches
-- follow it. It never runs while operator input waits, and it respects the per-sender steer slot.
UPDATE session_input_queue
SET mode = 'steer', delivery = 'interrupt_then_prompt', steer_delivery = NULL, dispatchable = 0,
    target_turn_id = sqlc.arg(target_turn_id), updated_at = sqlc.arg(now)
WHERE session_input_queue.session_id = sqlc.arg(session_id) AND session_input_queue.id = sqlc.arg(id)
  AND session_input_queue.status = 'queued' AND session_input_queue.mode = 'queue'
  AND session_input_queue.origin_class = 'agent' AND session_input_queue.owner_kind IS NULL
  AND session_input_queue.dispatchable = 1
  AND session_input_queue.session_generation = (SELECT input_generation FROM sessions WHERE sessions.id = sqlc.arg(session_id))
  AND NOT EXISTS (
    SELECT 1 FROM session_input_queue AS steer
    WHERE steer.session_id = sqlc.arg(session_id) AND steer.mode = 'steer' AND steer.status = 'queued'
      AND steer.origin_session_id = session_input_queue.origin_session_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM session_input_queue AS human
    WHERE human.session_id = sqlc.arg(session_id) AND human.origin_class = 'human'
      AND human.owner_kind IS NULL AND human.status IN ('queued', 'dispatching')
  );

-- name: RevertPromotedAgentSessionInput :execrows
-- A promotion whose live delivery could not start, or whose injection fell back, returns the message to
-- plain after-turn delivery instead of canceling it, so it never holds the sender's queued-steer slot.
UPDATE session_input_queue
SET mode = 'queue', delivery = 'after_turn', steer_delivery = NULL, dispatchable = 1, target_turn_id = '',
    status = 'queued', dispatch_started_at = NULL, updated_at = sqlc.arg(now)
WHERE session_id = sqlc.arg(session_id) AND id = sqlc.arg(id)
  AND origin_class = 'agent' AND owner_kind IS NULL AND mode = 'steer'
  AND (
    (status IN ('queued', 'dispatching') AND steer_delivery IS NULL)
    OR (status = 'queued' AND steer_delivery = 'interrupt_fallback')
  );
