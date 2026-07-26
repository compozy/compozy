-- name: InsertGoalJudgeAttempt :exec
INSERT INTO loop_goal_judge_attempts (
    attempt_id, loop_run_id, generation, node_id, item_index, turn,
    judge_digest, status, blocking_json, usage_base_tokens, started_at
) VALUES (
    sqlc.arg(attempt_id), sqlc.arg(loop_run_id), sqlc.arg(generation), sqlc.arg(node_id),
    sqlc.arg(item_index), sqlc.arg(turn), sqlc.arg(judge_digest), 'running', '[]',
    sqlc.arg(usage_base_tokens), CAST(sqlc.arg(started_at) AS TEXT)
);

-- name: AttachGoalJudgeAttempt :execrows
UPDATE loop_goal_checkpoints
SET judge_attempt_id = sqlc.narg(judge_attempt_id), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'judging' AND task_run_id = sqlc.arg(task_run_id) AND prompt_id = sqlc.arg(prompt_id)
  AND (judge_attempt_id IS NULL OR judge_attempt_id = sqlc.narg(judge_attempt_id));

-- name: CompleteGoalJudgeAttempt :execrows
UPDATE loop_goal_judge_attempts
SET status = 'completed', outcome = sqlc.narg(outcome), blocking_json = sqlc.arg(blocking_json),
    evidence_ref = sqlc.narg(evidence_ref), tokens_used = sqlc.narg(tokens_used),
    completed_at = CAST(sqlc.arg(completed_at) AS TEXT)
WHERE attempt_id = sqlc.arg(attempt_id) AND loop_run_id = sqlc.arg(loop_run_id) AND status = 'running';

-- name: GetGoalJudgeAttempt :one
SELECT * FROM loop_goal_judge_attempts
WHERE attempt_id = sqlc.arg(attempt_id) AND loop_run_id = sqlc.arg(loop_run_id)
  AND generation = sqlc.arg(generation) AND node_id = sqlc.arg(node_id)
  AND item_index = sqlc.arg(item_index);

-- name: GetGoalJudgeAttemptDigest :one
SELECT judge_digest FROM loop_goal_judge_attempts
WHERE attempt_id = sqlc.arg(attempt_id) AND loop_run_id = sqlc.arg(loop_run_id)
  AND generation = sqlc.arg(generation) AND node_id = sqlc.arg(node_id)
  AND item_index = sqlc.arg(item_index);

-- name: MarkGoalJudgeAttemptAmbiguous :execrows
UPDATE loop_goal_judge_attempts
SET status = 'ambiguous', completed_at = CAST(sqlc.arg(completed_at) AS TEXT)
WHERE attempt_id = sqlc.arg(attempt_id) AND loop_run_id = sqlc.arg(loop_run_id) AND status = 'running';

-- name: FinalizeAmbiguousGoalJudgeTurn :execrows
UPDATE loop_goal_turns
SET result_status = 'ambiguous', reason_code = sqlc.narg(reason_code),
    ended_at = CAST(sqlc.arg(ended_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND turn = sqlc.arg(turn) AND result_status IS NULL;

-- name: CheckpointAmbiguousGoalJudge :execrows
UPDATE loop_goal_checkpoints
SET phase = 'awaiting_control', goal_status = 'paused', control_cause = sqlc.narg(control_cause),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'judging' AND judge_attempt_id = sqlc.narg(judge_attempt_id);

-- name: SettleGoalTurn :execrows
UPDATE loop_goal_turns
SET result_status = sqlc.narg(result_status), stop_reason = sqlc.narg(stop_reason),
    reason_code = sqlc.narg(reason_code), verdict_outcome = sqlc.narg(verdict_outcome),
    blocking_json = sqlc.arg(blocking_json), evidence_ref = sqlc.narg(evidence_ref),
    tokens_used = sqlc.narg(tokens_used), ended_at = CAST(sqlc.arg(ended_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND prompt_id = sqlc.arg(prompt_id) AND result_status IS NULL;

-- name: ClearSettledGoalPause :exec
UPDATE loop_runs
SET pause_requested = 0, control_actor_kind = NULL, control_actor_id = NULL, control_requested_at = NULL
WHERE id = sqlc.arg(id) AND pause_requested = 1;

-- name: GetCompletedGoalTurn :one
SELECT result_status, stop_reason, reason_code, verdict_outcome, blocking_json,
       tokens_used, actor_kind, actor_id, binding_epoch
FROM loop_goal_turns
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND prompt_id = sqlc.arg(prompt_id);

-- name: GetGoalSettlementEvidenceRef :one
SELECT evidence_ref FROM loop_goal_judge_attempts
WHERE attempt_id = sqlc.arg(attempt_id) AND loop_run_id = sqlc.arg(loop_run_id)
  AND status = 'completed';

-- name: GetPendingGoalPauseActor :one
SELECT pause_requested, control_actor_kind, control_actor_id, control_requested_at
FROM loop_runs WHERE id = sqlc.arg(id);

-- name: UpdateGoalCheckpointAfterTurn :execrows
UPDATE loop_goal_checkpoints
SET phase = sqlc.arg(phase), goal_status = sqlc.arg(goal_status), control_cause = sqlc.narg(control_cause),
    broken_streak = sqlc.arg(broken_streak), recovery_streak = sqlc.arg(recovery_streak),
    queue_entry_id = NULL, prompt_id = NULL, prompt_kind = NULL, prompt_attempt = 0,
    judge_attempt_id = NULL, report_prompt_id = NULL, report_status = NULL,
    report_evidence_ref = NULL, report_binding_epoch = NULL, report_actor_kind = NULL,
    report_actor_id = NULL, report_recorded_at = NULL,
    control_actor_kind = sqlc.narg(control_actor_kind), control_actor_id = sqlc.narg(control_actor_id),
    control_requested_at = CAST(sqlc.narg(control_requested_at) AS TEXT),
    control_grant_consumed = CASE WHEN sqlc.arg(consume_grant) = 1 THEN 1 ELSE control_grant_consumed END,
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND task_run_id = sqlc.arg(task_run_id) AND queue_entry_id = sqlc.arg(queue_entry_id)
  AND prompt_id = sqlc.arg(prompt_id) AND phase IN ('prompting','judging','persisting');

-- name: GetGoalTurnByPromptID :one
SELECT * FROM loop_goal_turns
WHERE loop_goal_turns.loop_run_id = sqlc.arg(loop_run_id)
  AND loop_goal_turns.generation = sqlc.arg(generation)
  AND loop_goal_turns.node_id = sqlc.arg(node_id)
  AND loop_goal_turns.item_index = sqlc.arg(item_index)
  AND loop_goal_turns.prompt_id = sqlc.arg(prompt_id)
  AND EXISTS (
      SELECT 1 FROM loop_runs
      WHERE loop_runs.id = loop_goal_turns.loop_run_id
        AND loop_runs.workspace_id = sqlc.arg(workspace_id)
  );

-- name: FindGoalReportTargets :many
SELECT checkpoints.loop_run_id, checkpoints.generation, checkpoints.node_id,
       checkpoints.item_index, checkpoints.control_epoch, checkpoints.binding_epoch,
       checkpoints.prompt_id, runs.origin_session_id, checkpoints.session_id
FROM loop_goal_checkpoints AS checkpoints
JOIN loop_runs AS runs ON runs.id = checkpoints.loop_run_id
JOIN loop_session_bindings AS bindings
  ON bindings.loop_run_id = checkpoints.loop_run_id
 AND bindings.handle = checkpoints.binding_handle
 AND bindings.binding_epoch = checkpoints.binding_epoch
 AND bindings.session_id = checkpoints.session_id
WHERE runs.workspace_id = sqlc.arg(workspace_id) AND runs.origin_kind = 'session'
  AND runs.goal_cleared_at IS NULL
  AND runs.status IN ('queued','running','watching','needs-approval','paused')
  AND checkpoints.phase = 'prompting' AND checkpoints.goal_status = 'active'
  AND checkpoints.session_id = sqlc.narg(session_id) AND checkpoints.prompt_id IS NOT NULL
  AND bindings.workspace_id = sqlc.arg(workspace_id) AND bindings.state = 'active'
ORDER BY checkpoints.updated_at DESC, runs.created_at DESC, runs.rowid DESC
LIMIT 2;

-- name: ResolveActiveGoalOriginAliases :many
SELECT runs.origin_session_id
FROM loop_session_bindings AS bindings
JOIN loop_runs AS runs ON runs.id = bindings.loop_run_id
WHERE bindings.workspace_id = sqlc.arg(workspace_id) AND bindings.session_id = sqlc.arg(session_id)
  AND bindings.state = 'active' AND runs.workspace_id = sqlc.arg(workspace_id)
  AND runs.origin_kind = 'session' AND runs.origin_session_id <> bindings.session_id
  AND runs.goal_cleared_at IS NULL
  AND runs.status IN ('queued','running','watching','needs-approval','paused')
ORDER BY bindings.activated_at DESC, runs.created_at DESC, runs.rowid DESC
LIMIT 2;
