-- name: GetGoalDatabaseNow :one
SELECT CAST(strftime('%Y-%m-%dT%H:%M:%fZ', 'now') AS TEXT);

-- name: FlushGoalTaskUsage :execrows
UPDATE task_runs
SET tokens_used = CASE
    WHEN sqlc.arg(tokens_reported) = 1 AND sqlc.arg(live_tokens_used) > tokens_used
      THEN sqlc.arg(live_tokens_used)
    ELSE tokens_used
END
WHERE task_runs.id = sqlc.arg(id) AND task_runs.loop_run_id = sqlc.arg(task_loop_run_id)
  AND EXISTS (
      SELECT 1 FROM loop_goal_checkpoints AS checkpoint
      WHERE checkpoint.loop_run_id = sqlc.arg(checkpoint_loop_run_id)
        AND checkpoint.generation = sqlc.arg(generation)
        AND checkpoint.node_id = sqlc.arg(node_id)
        AND checkpoint.item_index = sqlc.arg(item_index)
        AND checkpoint.control_epoch = sqlc.arg(control_epoch)
        AND COALESCE(checkpoint.binding_epoch, 0) = sqlc.arg(binding_epoch)
        AND checkpoint.phase = sqlc.arg(phase)
        AND COALESCE(checkpoint.task_run_id, '') = sqlc.arg(task_run_id)
        AND COALESCE(checkpoint.queue_entry_id, '') = sqlc.arg(queue_entry_id)
        AND COALESCE(checkpoint.prompt_id, '') = sqlc.arg(prompt_id)
  );

-- name: AdvanceGoalBudgetVersion :one
UPDATE loop_runs
SET budget_version = budget_version + 1
WHERE id = sqlc.arg(id)
RETURNING budget_tokens, budget_wall_sec, budget_on_exceeded, started_at, budget_version;

-- name: ValidateGoalBudgetDecision :one
SELECT CASE
    WHEN budget_version = sqlc.arg(budget_version)
     AND julianday('now') <= julianday(CAST(sqlc.arg(valid_until) AS TEXT)) THEN 1
    ELSE 0
END
FROM loop_runs
WHERE id = sqlc.arg(id);

-- name: InsertGoalCheckpoint :execrows
INSERT INTO loop_goal_checkpoints (
    loop_run_id, generation, node_id, item_index, control_epoch,
    control_actor_kind, control_actor_id, control_requested_at, phase, goal_status, control_cause,
    turns_used, turn_limit, broken_streak, recovery_streak, task_run_id,
    queue_entry_id, prompt_id, prompt_kind, prompt_attempt, context_state,
    usage_sequence, usage_pending_after_sequence, compaction_baseline_used,
    compaction_recovery_required, session_id, binding_handle, binding_epoch,
    context_nudge_ratio, control_grant_id, control_grant_kind, control_grant_cause,
    control_grant_turn, control_grant_scope, control_grant_consumed, judge_attempt_id,
    compaction_cancel_prompt_id, compaction_cancel_cause, compaction_cancel_requested_at,
    report_prompt_id, report_status, report_evidence_ref, report_binding_epoch,
    report_actor_kind, report_actor_id, report_recorded_at, updated_at
) VALUES (
    sqlc.arg(loop_run_id), sqlc.arg(generation), sqlc.arg(node_id), sqlc.arg(item_index), sqlc.arg(control_epoch),
    sqlc.narg(control_actor_kind), sqlc.narg(control_actor_id), CAST(sqlc.narg(control_requested_at) AS TEXT),
    sqlc.arg(phase), sqlc.arg(goal_status), sqlc.narg(control_cause),
    sqlc.arg(turns_used), sqlc.arg(turn_limit), sqlc.arg(broken_streak), sqlc.arg(recovery_streak),
    sqlc.narg(task_run_id), sqlc.narg(queue_entry_id), sqlc.narg(prompt_id), sqlc.narg(prompt_kind),
    sqlc.arg(prompt_attempt), sqlc.arg(context_state), sqlc.narg(usage_sequence),
    sqlc.narg(usage_pending_after_sequence), sqlc.narg(compaction_baseline_used),
    sqlc.arg(compaction_recovery_required), sqlc.narg(session_id), sqlc.narg(binding_handle),
    sqlc.narg(binding_epoch), sqlc.arg(context_nudge_ratio), sqlc.arg(control_grant_id),
    sqlc.narg(control_grant_kind), sqlc.narg(control_grant_cause), sqlc.narg(control_grant_turn),
    sqlc.narg(control_grant_scope), sqlc.arg(control_grant_consumed), sqlc.narg(judge_attempt_id),
    sqlc.narg(compaction_cancel_prompt_id), sqlc.narg(compaction_cancel_cause),
    CAST(sqlc.narg(compaction_cancel_requested_at) AS TEXT), sqlc.narg(report_prompt_id),
    sqlc.narg(report_status), sqlc.narg(report_evidence_ref), sqlc.narg(report_binding_epoch),
    sqlc.narg(report_actor_kind), sqlc.narg(report_actor_id), CAST(sqlc.narg(report_recorded_at) AS TEXT),
    CAST(sqlc.arg(updated_at) AS TEXT)
)
ON CONFLICT(loop_run_id, generation, node_id, item_index) DO NOTHING;

-- name: GetGoalCheckpoint :one
SELECT *
FROM loop_goal_checkpoints
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index);

-- name: GrantGoalCheckpointControl :execrows
UPDATE loop_goal_checkpoints
SET control_epoch = control_epoch + 1, phase = 'idle', goal_status = 'active', control_cause = NULL,
    turn_limit = sqlc.arg(turn_limit), control_actor_kind = NULL, control_actor_id = NULL,
    control_requested_at = NULL, control_grant_id = sqlc.arg(control_grant_id),
    control_grant_kind = sqlc.narg(control_grant_kind), control_grant_cause = sqlc.narg(control_grant_cause),
    control_grant_turn = sqlc.narg(control_grant_turn), control_grant_scope = sqlc.narg(control_grant_scope),
    control_grant_consumed = sqlc.arg(control_grant_consumed), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND phase = 'awaiting_control';

-- name: ProjectGoalGrantOutput :execrows
UPDATE loop_generation_outputs
SET goal_status = 'active', goal_turns_used = sqlc.arg(goal_turns_used),
    goal_turn_limit = sqlc.arg(goal_turn_limit)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index);

-- name: UpdateGoalCheckpointControl :execrows
UPDATE loop_goal_checkpoints
SET phase = sqlc.arg(phase), goal_status = sqlc.arg(goal_status), control_cause = sqlc.narg(control_cause),
    control_actor_kind = sqlc.narg(control_actor_kind), control_actor_id = sqlc.narg(control_actor_id),
    control_requested_at = CAST(sqlc.arg(control_requested_at) AS TEXT),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch)
  AND COALESCE(binding_epoch, 0) = sqlc.arg(binding_epoch)
  AND phase = sqlc.arg(expected_phase)
  AND COALESCE(task_run_id, '') = sqlc.arg(task_run_id)
  AND COALESCE(queue_entry_id, '') = sqlc.arg(queue_entry_id)
  AND COALESCE(prompt_id, '') = sqlc.arg(prompt_id);

-- name: IncrementGoalBudgetVersion :execrows
UPDATE loop_runs
SET budget_version = budget_version + 1
WHERE id = sqlc.arg(id);

-- name: RecordGoalReportIntent :execrows
UPDATE loop_goal_checkpoints
SET report_prompt_id = sqlc.narg(report_prompt_id), report_status = sqlc.narg(report_status),
    report_evidence_ref = sqlc.narg(report_evidence_ref), report_binding_epoch = sqlc.narg(report_binding_epoch),
    report_actor_kind = sqlc.narg(report_actor_kind), report_actor_id = sqlc.narg(report_actor_id),
    report_recorded_at = CAST(sqlc.arg(report_recorded_at) AS TEXT),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'prompting' AND goal_status = 'active' AND prompt_id = sqlc.arg(prompt_id)
  AND report_status IS NULL;

-- name: BeginGoalCompactionCancel :execrows
UPDATE loop_goal_checkpoints
SET compaction_cancel_prompt_id = sqlc.narg(compaction_cancel_prompt_id),
    compaction_cancel_cause = sqlc.narg(compaction_cancel_cause),
    compaction_cancel_requested_at = CAST(sqlc.arg(compaction_cancel_requested_at) AS TEXT),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'compacting' AND task_run_id = sqlc.arg(task_run_id)
  AND queue_entry_id = sqlc.arg(queue_entry_id) AND prompt_id = sqlc.arg(prompt_id)
  AND compaction_cancel_cause IS NULL;

-- name: ListStoppableGoalCheckpointKeys :many
SELECT generation, node_id, item_index
FROM loop_goal_checkpoints
WHERE loop_run_id = sqlc.arg(loop_run_id) AND phase != 'terminal'
ORDER BY generation ASC, node_id ASC, item_index ASC;

-- name: TerminalizeStoppedGoalCheckpoint :execrows
UPDATE loop_goal_checkpoints
SET control_epoch = control_epoch + 1, phase = 'terminal', goal_status = 'paused',
    control_cause = sqlc.narg(control_cause), control_actor_kind = sqlc.narg(control_actor_kind),
    control_actor_id = sqlc.narg(control_actor_id),
    control_requested_at = CAST(sqlc.arg(control_requested_at) AS TEXT),
    queue_entry_id = NULL, prompt_id = NULL, prompt_kind = NULL, prompt_attempt = 0,
    judge_attempt_id = NULL, report_prompt_id = NULL, report_status = NULL,
    report_evidence_ref = NULL, report_binding_epoch = NULL, report_actor_kind = NULL,
    report_actor_id = NULL, report_recorded_at = NULL, compaction_cancel_prompt_id = NULL,
    compaction_cancel_cause = NULL, compaction_cancel_requested_at = NULL,
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND phase = sqlc.arg(expected_phase)
  AND phase != 'terminal';

-- name: GetAwaitingGoalControl :one
SELECT c.generation, c.node_id, c.item_index, c.control_epoch, c.goal_status,
       COALESCE(c.control_cause, '') AS control_cause,
       c.turns_used, c.turn_limit, COALESCE(c.task_run_id, '') AS task_run_id,
       COALESCE(c.queue_entry_id, '') AS queue_entry_id,
       COALESCE(c.prompt_id, '') AS prompt_id, COALESCE(c.prompt_kind, '') AS prompt_kind,
       EXISTS (
           SELECT 1 FROM loop_goal_turns t
           WHERE t.loop_run_id = c.loop_run_id AND t.generation = c.generation
             AND t.node_id = c.node_id AND t.item_index = c.item_index
             AND t.result_status IS NULL
       ) AS open_turn,
       l.status AS run_status, l.active_gate_id, o.status AS output_status,
       COALESCE(o.task_run_id, '') AS output_task_run,
       c.control_grant_id, c.control_grant_kind, c.control_grant_cause,
       c.control_grant_turn, c.control_grant_scope, c.control_grant_consumed
FROM loop_goal_checkpoints c
JOIN loop_runs l ON l.id = c.loop_run_id
JOIN loop_generation_outputs o
  ON o.loop_run_id = c.loop_run_id AND o.generation = c.generation
 AND o.node_id = c.node_id AND o.item_index = c.item_index
WHERE c.loop_run_id = sqlc.arg(loop_run_id) AND l.workspace_id = sqlc.arg(workspace_id)
  AND c.phase = 'awaiting_control' AND o.status = sqlc.arg(output_status)
ORDER BY c.generation ASC, c.node_id ASC, c.item_index ASC
LIMIT 1;

-- name: GetGoalReentryState :one
SELECT c.generation, c.node_id, c.item_index, c.control_epoch, c.goal_status,
       COALESCE(c.control_cause, '') AS control_cause,
       c.turns_used, c.turn_limit, COALESCE(c.task_run_id, '') AS task_run_id,
       COALESCE(c.queue_entry_id, '') AS queue_entry_id,
       COALESCE(c.prompt_id, '') AS prompt_id, COALESCE(c.prompt_kind, '') AS prompt_kind,
       EXISTS (
           SELECT 1 FROM loop_goal_turns t
           WHERE t.loop_run_id = c.loop_run_id AND t.generation = c.generation
             AND t.node_id = c.node_id AND t.item_index = c.item_index
             AND t.result_status IS NULL
       ) AS open_turn,
       l.status AS run_status, l.active_gate_id, o.status AS output_status,
       COALESCE(o.task_run_id, '') AS output_task_run,
       c.control_grant_id, c.control_grant_kind, c.control_grant_cause,
       c.control_grant_turn, c.control_grant_scope, c.control_grant_consumed
FROM loop_goal_checkpoints c
JOIN loop_runs l ON l.id = c.loop_run_id
JOIN loop_generation_outputs o
  ON o.loop_run_id = c.loop_run_id AND o.generation = c.generation
 AND o.node_id = c.node_id AND o.item_index = c.item_index
WHERE c.loop_run_id = sqlc.arg(loop_run_id) AND l.workspace_id = sqlc.arg(workspace_id)
  AND c.generation = sqlc.arg(generation) AND c.node_id = sqlc.arg(node_id)
  AND c.item_index = sqlc.arg(item_index);

-- name: UpdateGoalCheckpointForReentry :execrows
UPDATE loop_goal_checkpoints
SET control_epoch = sqlc.arg(next_control_epoch), phase = sqlc.arg(phase),
    goal_status = 'active', control_cause = NULL, turn_limit = sqlc.arg(turn_limit),
    task_run_id = sqlc.narg(successor_task_run_id),
    queue_entry_id = CASE WHEN sqlc.arg(clear_operation) = 1 THEN NULL ELSE queue_entry_id END,
    prompt_id = CASE WHEN sqlc.arg(clear_operation) = 1 THEN NULL ELSE prompt_id END,
    prompt_kind = CASE WHEN sqlc.arg(clear_operation) = 1 THEN NULL ELSE prompt_kind END,
    judge_attempt_id = CASE WHEN sqlc.arg(clear_operation) = 1 THEN NULL ELSE judge_attempt_id END,
    control_actor_kind = NULL, control_actor_id = NULL, control_requested_at = NULL,
    control_grant_id = sqlc.arg(control_grant_id), control_grant_kind = sqlc.narg(control_grant_kind),
    control_grant_cause = sqlc.narg(control_grant_cause), control_grant_turn = sqlc.narg(control_grant_turn),
    control_grant_scope = sqlc.narg(control_grant_scope),
    control_grant_consumed = sqlc.arg(control_grant_consumed),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(expected_control_epoch) AND phase = 'awaiting_control'
  AND task_run_id = sqlc.arg(expected_task_run_id);

-- name: UpdateGoalPromptOwnerForReentry :execrows
UPDATE session_input_queue
SET task_run_id = sqlc.arg(successor_task_run_id), owner_epoch = sqlc.narg(next_owner_epoch),
    fence_kind = CASE
      WHEN sqlc.arg(clear_control_fence) = 1 AND fence_kind = 'control-fenced' THEN NULL ELSE fence_kind END,
    fence_disposition = CASE
      WHEN sqlc.arg(clear_control_fence) = 1 AND fence_kind = 'control-fenced' THEN NULL ELSE fence_disposition END,
    fence_reason_code = CASE
      WHEN sqlc.arg(clear_control_fence) = 1 AND fence_kind = 'control-fenced' THEN NULL ELSE fence_reason_code END,
    fenced_at = CASE
      WHEN sqlc.arg(clear_control_fence) = 1 AND fence_kind = 'control-fenced' THEN NULL ELSE fenced_at END,
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.arg(loop_run_id)
  AND prompt_id = sqlc.arg(prompt_id) AND owner_kind = 'goal'
  AND owner_epoch = sqlc.arg(expected_owner_epoch) AND task_run_id = sqlc.arg(expected_task_run_id);

-- name: UpdateGoalOutputForReentry :execrows
UPDATE loop_generation_outputs
SET status = 'enqueued', task_run_id = sqlc.narg(successor_task_run_id), goal_status = 'active',
    goal_turns_used = sqlc.narg(goal_turns_used), goal_turn_limit = sqlc.narg(goal_turn_limit)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND status = sqlc.arg(expected_status) AND task_run_id = sqlc.arg(expected_task_run_id);

-- name: ClearGoalRunControlActor :exec
UPDATE loop_runs
SET control_actor_kind = NULL, control_actor_id = NULL, control_requested_at = NULL
WHERE id = sqlc.arg(id);
