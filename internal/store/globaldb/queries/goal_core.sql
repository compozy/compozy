-- name: GoalRunWorkspaceExists :one
SELECT 1 FROM loop_runs WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id);

-- name: ActiveGoalPromptBindingExists :one
SELECT 1 FROM loop_session_bindings
WHERE loop_run_id = sqlc.arg(loop_run_id) AND workspace_id = sqlc.arg(workspace_id)
  AND handle = sqlc.arg(handle) AND binding_epoch = sqlc.arg(binding_epoch)
  AND session_id = sqlc.arg(session_id) AND state = 'active';

-- name: UpdateGoalContextUsage :execrows
UPDATE loop_goal_checkpoints
SET context_state = sqlc.arg(context_state), usage_sequence = sqlc.narg(usage_sequence),
    usage_pending_after_sequence = sqlc.narg(usage_pending_after_sequence),
    compaction_baseline_used = sqlc.narg(compaction_baseline_used),
    compaction_recovery_required = sqlc.arg(compaction_recovery_required),
    recovery_streak = sqlc.arg(recovery_streak), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = sqlc.arg(phase) AND goal_status = 'active'
  AND session_id = sqlc.arg(session_id) AND binding_handle = sqlc.arg(binding_handle);

-- name: SettleGoalCompactionCheckpoint :execrows
UPDATE loop_goal_checkpoints
SET phase = 'idle', goal_status = 'active', control_cause = NULL,
    queue_entry_id = NULL, prompt_id = NULL, prompt_kind = NULL, prompt_attempt = 0,
    context_state = sqlc.arg(context_state), usage_sequence = sqlc.narg(usage_sequence),
    usage_pending_after_sequence = sqlc.narg(usage_pending_after_sequence),
    compaction_baseline_used = sqlc.narg(compaction_baseline_used),
    compaction_recovery_required = sqlc.arg(compaction_recovery_required),
    recovery_streak = sqlc.arg(recovery_streak),
    compaction_cancel_prompt_id = NULL, compaction_cancel_cause = NULL,
    compaction_cancel_requested_at = NULL,
    control_grant_consumed = CASE WHEN sqlc.arg(consume_grant) = 1 THEN 1 ELSE control_grant_consumed END,
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'compacting' AND task_run_id = sqlc.arg(task_run_id)
  AND queue_entry_id = sqlc.arg(queue_entry_id) AND prompt_id = sqlc.arg(prompt_id);

-- name: GetGoalPromptRow :one
SELECT id, session_id, status, text, task_run_id, run_generation,
       loop_run_id, owner_kind, owner_epoch, binding_epoch, prompt_id, prompt_kind,
       prompt_attempt, dispatchable, operation_usage_base_tokens, dispatch_token_hash,
       fence_kind, fence_disposition, fence_reason_code,
       terminal_event_start_seq, terminal_event_end_seq, terminal_kind,
       terminal_stop_reason, terminal_disposition, terminal_reason_code,
       terminal_tokens_reported, terminal_tokens_used, terminal_at
FROM session_input_queue
WHERE loop_run_id = sqlc.arg(loop_run_id) AND prompt_id = sqlc.arg(prompt_id);

-- name: ProjectGoalCheckpointCounts :exec
UPDATE loop_generation_outputs
SET goal_status = sqlc.arg(goal_status), goal_turns_used = sqlc.arg(goal_turns_used),
    goal_turn_limit = sqlc.arg(goal_turn_limit)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index);

-- name: RecoverGoalQueueTerminal :execrows
UPDATE session_input_queue
SET status = sqlc.arg(status), terminal_event_start_seq = sqlc.narg(terminal_event_start_seq),
    terminal_event_end_seq = sqlc.narg(terminal_event_end_seq), terminal_kind = sqlc.arg(terminal_kind),
    terminal_stop_reason = sqlc.narg(terminal_stop_reason), terminal_disposition = sqlc.narg(terminal_disposition),
    terminal_reason_code = sqlc.narg(terminal_reason_code), terminal_tokens_reported = sqlc.arg(terminal_tokens_reported),
    terminal_tokens_used = sqlc.narg(terminal_tokens_used), terminal_at = CAST(sqlc.arg(terminal_at) AS TEXT),
    failed_at = CASE WHEN sqlc.arg(status) = 'failed' THEN CAST(sqlc.arg(terminal_at) AS TEXT) ELSE failed_at END,
    updated_at = CAST(sqlc.arg(terminal_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.arg(loop_run_id)
  AND task_run_id = sqlc.arg(task_run_id) AND prompt_id = sqlc.arg(prompt_id)
  AND owner_kind = sqlc.arg(owner_kind) AND owner_epoch = sqlc.arg(owner_epoch)
  AND binding_epoch = sqlc.arg(binding_epoch) AND session_id = sqlc.arg(session_id)
  AND status IN ('dispatching', 'sent') AND terminal_at IS NULL
  AND dispatch_token_hash = sqlc.arg(dispatch_token_hash);
