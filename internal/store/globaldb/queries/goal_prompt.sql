-- name: InsertOpenGoalTurn :exec
INSERT INTO loop_goal_turns (
    loop_run_id, seq, generation, node_id, item_index, turn,
    session_id, binding_handle, binding_epoch, prompt_id, prompt_attempt,
    usage_base_tokens, actor_kind, actor_id, started_at
) VALUES (
    sqlc.arg(loop_run_id), sqlc.arg(seq), sqlc.arg(generation), sqlc.arg(node_id),
    sqlc.arg(item_index), sqlc.arg(turn), sqlc.arg(session_id), sqlc.arg(binding_handle),
    sqlc.arg(binding_epoch), sqlc.arg(prompt_id), sqlc.arg(prompt_attempt),
    sqlc.arg(usage_base_tokens), sqlc.arg(actor_kind), sqlc.arg(actor_id),
    CAST(sqlc.arg(started_at) AS TEXT)
);

-- name: StartGoalWorkCheckpoint :execrows
UPDATE loop_goal_checkpoints
SET phase = 'prompting', turns_used = sqlc.arg(turns_used), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'queued' AND task_run_id = sqlc.arg(task_run_id)
  AND queue_entry_id = sqlc.arg(queue_entry_id) AND prompt_id = sqlc.arg(prompt_id);

-- name: StartGoalCompactionCheckpoint :execrows
UPDATE loop_goal_checkpoints
SET phase = 'compacting', updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'queued' AND task_run_id = sqlc.arg(task_run_id)
  AND queue_entry_id = sqlc.arg(queue_entry_id) AND prompt_id = sqlc.arg(prompt_id);

-- name: ClaimGoalQueueRow :execrows
UPDATE session_input_queue
SET status = 'dispatching', dispatchable = 1, activated_at = CAST(sqlc.arg(activated_at) AS TEXT),
    dispatch_started_at = CAST(sqlc.arg(dispatch_started_at) AS TEXT),
    attempt_count = attempt_count + 1, operation_usage_base_tokens = sqlc.narg(operation_usage_base_tokens),
    dispatch_token_hash = sqlc.narg(dispatch_token_hash), fence_kind = NULL, fence_disposition = NULL,
    fence_reason_code = NULL, fenced_at = NULL, updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE id = sqlc.arg(id) AND status = 'queued' AND dispatchable = 0 AND terminal_at IS NULL
  AND owner_kind = sqlc.narg(owner_kind) AND owner_epoch = sqlc.narg(owner_epoch);

-- name: GetNextGoalTurnSequence :one
SELECT CAST(COALESCE(MAX(seq), 0) + 1 AS INTEGER)
FROM loop_goal_turns
WHERE loop_run_id = sqlc.arg(loop_run_id);

-- name: UpdatePreparedGoalQueueFence :execrows
UPDATE session_input_queue
SET status = sqlc.arg(status), dispatchable = 0, fence_kind = sqlc.narg(fence_kind),
    fence_disposition = sqlc.narg(fence_disposition), fence_reason_code = sqlc.narg(fence_reason_code),
    fenced_at = CAST(sqlc.arg(fenced_at) AS TEXT),
    terminal_kind = CASE WHEN sqlc.arg(grantable) = 1 THEN NULL ELSE sqlc.narg(terminal_kind) END,
    terminal_disposition = CASE WHEN sqlc.arg(grantable) = 1 THEN NULL ELSE sqlc.narg(terminal_disposition) END,
    terminal_reason_code = CASE WHEN sqlc.arg(grantable) = 1 THEN NULL ELSE sqlc.narg(terminal_reason_code) END,
    terminal_at = CASE WHEN sqlc.arg(grantable) = 1 THEN NULL ELSE CAST(sqlc.arg(terminal_at) AS TEXT) END,
    canceled_at = CASE WHEN sqlc.arg(grantable) = 1 THEN canceled_at ELSE CAST(sqlc.arg(canceled_at) AS TEXT) END,
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.narg(loop_run_id) AND prompt_id = sqlc.narg(prompt_id)
  AND status = 'queued' AND dispatchable = 0 AND terminal_at IS NULL
  AND owner_epoch = sqlc.narg(owner_epoch);

-- name: UpdatePreparedGoalCheckpointFence :execrows
UPDATE loop_goal_checkpoints
SET phase = sqlc.arg(phase), goal_status = sqlc.arg(goal_status),
    control_cause = sqlc.narg(control_cause), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'queued' AND queue_entry_id = sqlc.arg(queue_entry_id) AND prompt_id = sqlc.arg(prompt_id);

-- name: GetDeniedGoalBudgetVersion :one
SELECT budget_version FROM loop_runs WHERE id = sqlc.arg(id);

-- name: InsertPreparedGoalPromptQueue :exec
INSERT INTO session_input_queue (
    id, session_id, status, mode, text, session_generation,
    task_run_id, run_generation, attempt_count, enqueued_at, updated_at,
    loop_run_id, owner_kind, owner_epoch, binding_epoch, prompt_id,
    prompt_kind, prompt_attempt, operation_usage_base_tokens, dispatchable
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), 'queued', sqlc.arg(mode), sqlc.arg(text),
    (SELECT input_generation FROM sessions WHERE id = sqlc.arg(session_id)),
    sqlc.arg(task_run_id), sqlc.arg(run_generation), 0, CAST(sqlc.arg(enqueued_at) AS TEXT),
    CAST(sqlc.arg(updated_at) AS TEXT), sqlc.narg(loop_run_id), sqlc.narg(owner_kind),
    sqlc.narg(owner_epoch), sqlc.narg(binding_epoch), sqlc.narg(prompt_id),
    sqlc.narg(prompt_kind), sqlc.arg(prompt_attempt), sqlc.narg(operation_usage_base_tokens), 0
);

-- name: CheckpointPreparedGoalPrompt :execrows
UPDATE loop_goal_checkpoints
SET phase = 'queued', task_run_id = sqlc.narg(task_run_id), queue_entry_id = sqlc.narg(queue_entry_id),
    prompt_id = sqlc.narg(prompt_id), prompt_kind = sqlc.narg(prompt_kind),
    prompt_attempt = sqlc.arg(prompt_attempt), session_id = sqlc.narg(session_id),
    binding_handle = sqlc.narg(binding_handle), binding_epoch = sqlc.narg(binding_epoch),
    compaction_baseline_used = sqlc.narg(compaction_baseline_used),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND goal_status = 'active'
  AND phase IN ('idle','preparing');

-- name: ValidateGoalPromptTaskRun :one
SELECT 1 FROM task_runs WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.narg(loop_run_id);

-- name: EnsureAmbiguousGoalQueueTerminal :execrows
UPDATE session_input_queue
SET status = 'failed', terminal_kind = 'ambiguous', terminal_reason_code = sqlc.narg(terminal_reason_code),
    terminal_tokens_reported = 0, terminal_tokens_used = NULL,
    terminal_at = CAST(sqlc.arg(terminal_at) AS TEXT), failed_at = CAST(sqlc.arg(failed_at) AS TEXT),
    failure_summary = sqlc.arg(failure_summary), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.narg(loop_run_id) AND prompt_id = sqlc.narg(prompt_id)
  AND status IN ('dispatching','sent') AND terminal_at IS NULL;

-- name: FinalizeAmbiguousGoalTurn :execrows
UPDATE loop_goal_turns
SET result_status = 'ambiguous', reason_code = sqlc.narg(reason_code),
    ended_at = CAST(sqlc.arg(ended_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND prompt_id = sqlc.arg(prompt_id) AND result_status IS NULL;

-- name: CheckpointAmbiguousGoalPrompt :execrows
UPDATE loop_goal_checkpoints
SET phase = 'awaiting_control', goal_status = 'paused', control_cause = sqlc.narg(control_cause),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND task_run_id = sqlc.arg(task_run_id) AND queue_entry_id = sqlc.arg(queue_entry_id)
  AND prompt_id = sqlc.arg(prompt_id) AND phase IN ('prompting','compacting','persisting');

-- name: RejectPreparedGoalQueueRow :execrows
UPDATE session_input_queue
SET status = 'failed', dispatchable = 0, terminal_kind = 'rejected-before-submit',
    terminal_reason_code = sqlc.narg(terminal_reason_code), terminal_tokens_reported = 0,
    terminal_tokens_used = NULL, terminal_at = CAST(sqlc.arg(terminal_at) AS TEXT),
    failed_at = CAST(sqlc.arg(failed_at) AS TEXT), failure_summary = sqlc.arg(failure_summary),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.narg(loop_run_id) AND prompt_id = sqlc.narg(prompt_id)
  AND task_run_id = sqlc.arg(task_run_id) AND owner_kind = 'goal'
  AND owner_epoch = sqlc.narg(owner_epoch) AND binding_epoch = sqlc.narg(binding_epoch)
  AND status = 'queued' AND terminal_at IS NULL AND dispatch_token_hash IS NULL;

-- name: AdvanceRejectedGoalCheckpoint :execrows
UPDATE loop_goal_checkpoints
SET phase = 'preparing', prompt_attempt = sqlc.arg(prompt_attempt), queue_entry_id = NULL,
    prompt_id = NULL, prompt_kind = NULL, updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND phase = 'queued' AND task_run_id = sqlc.arg(task_run_id)
  AND queue_entry_id = sqlc.arg(queue_entry_id) AND prompt_id = sqlc.arg(prompt_id);

-- name: TerminalizePreparedGoalRevocation :execrows
UPDATE session_input_queue
SET status = 'canceled', terminal_kind = 'control-fenced',
    terminal_disposition = sqlc.narg(terminal_disposition), terminal_reason_code = sqlc.narg(terminal_reason_code),
    terminal_tokens_reported = 0, terminal_tokens_used = NULL,
    terminal_at = CAST(sqlc.arg(terminal_at) AS TEXT), canceled_at = CAST(sqlc.arg(canceled_at) AS TEXT),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.narg(loop_run_id) AND task_run_id = sqlc.arg(task_run_id)
  AND prompt_id = sqlc.narg(prompt_id) AND owner_kind = sqlc.narg(owner_kind)
  AND owner_epoch = sqlc.narg(owner_epoch) AND binding_epoch = sqlc.narg(binding_epoch)
  AND status = 'queued' AND dispatchable = 0 AND terminal_at IS NULL;

-- name: TerminalizeClaimedGoalRevocation :execrows
UPDATE session_input_queue
SET status = 'failed', terminal_kind = 'ambiguous', terminal_reason_code = sqlc.narg(terminal_reason_code),
    terminal_tokens_reported = 0, terminal_tokens_used = NULL,
    terminal_at = CAST(sqlc.arg(terminal_at) AS TEXT), failed_at = CAST(sqlc.arg(failed_at) AS TEXT),
    failure_summary = sqlc.arg(failure_summary), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.narg(loop_run_id) AND task_run_id = sqlc.arg(task_run_id)
  AND prompt_id = sqlc.narg(prompt_id) AND owner_kind = sqlc.narg(owner_kind)
  AND owner_epoch = sqlc.narg(owner_epoch) AND binding_epoch = sqlc.narg(binding_epoch)
  AND status IN ('dispatching','sent') AND terminal_at IS NULL;

-- name: TerminalizeRevokedGoalTurn :execrows
UPDATE loop_goal_turns
SET result_status = sqlc.narg(result_status), stop_reason = sqlc.narg(stop_reason),
    reason_code = sqlc.narg(reason_code), verdict_outcome = NULL, blocking_json = '[]',
    evidence_ref = NULL, tokens_used = sqlc.narg(tokens_used), ended_at = CAST(sqlc.arg(ended_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND prompt_id = sqlc.arg(prompt_id) AND binding_epoch = sqlc.arg(binding_epoch)
  AND result_status IS NULL;

-- name: AdvanceRevokedGoalCheckpoint :execrows
UPDATE loop_goal_checkpoints
SET control_epoch = control_epoch + 1, phase = 'terminal', goal_status = sqlc.arg(goal_status),
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
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND task_run_id = sqlc.arg(task_run_id) AND queue_entry_id = sqlc.arg(queue_entry_id)
  AND prompt_id = sqlc.arg(prompt_id) AND phase IN ('queued','prompting','compacting','judging','persisting');

-- name: MarkGoalRunCleared :execrows
UPDATE loop_runs SET goal_cleared_at = CAST(sqlc.arg(goal_cleared_at) AS TEXT)
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id);

-- name: RecordGoalDriverAttached :execrows
UPDATE session_input_queue
SET status = 'sent', sent_at = CAST(sqlc.arg(sent_at) AS TEXT), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.narg(loop_run_id) AND task_run_id = sqlc.arg(task_run_id)
  AND prompt_id = sqlc.narg(prompt_id) AND owner_kind = sqlc.narg(owner_kind)
  AND owner_epoch = sqlc.narg(owner_epoch) AND binding_epoch = sqlc.narg(binding_epoch)
  AND session_id = sqlc.arg(session_id) AND status = 'dispatching'
  AND dispatch_token_hash = sqlc.narg(dispatch_token_hash) AND terminal_at IS NULL;

-- name: PersistGoalQueueTerminal :execrows
UPDATE session_input_queue
SET status = sqlc.arg(status), terminal_event_start_seq = sqlc.narg(terminal_event_start_seq),
    terminal_event_end_seq = sqlc.narg(terminal_event_end_seq), terminal_kind = sqlc.narg(terminal_kind),
    terminal_stop_reason = sqlc.narg(terminal_stop_reason), terminal_disposition = sqlc.narg(terminal_disposition),
    terminal_reason_code = sqlc.narg(terminal_reason_code), terminal_tokens_reported = sqlc.arg(terminal_tokens_reported),
    terminal_tokens_used = sqlc.narg(terminal_tokens_used), terminal_at = CAST(sqlc.arg(terminal_at) AS TEXT),
    failed_at = CASE WHEN sqlc.arg(status) = 'failed' THEN CAST(sqlc.arg(terminal_at) AS TEXT) ELSE failed_at END,
    updated_at = CAST(sqlc.arg(terminal_at) AS TEXT)
WHERE id = sqlc.arg(id) AND loop_run_id = sqlc.narg(loop_run_id) AND task_run_id = sqlc.arg(task_run_id)
  AND prompt_id = sqlc.narg(prompt_id) AND owner_kind = sqlc.narg(owner_kind)
  AND owner_epoch = sqlc.narg(owner_epoch) AND binding_epoch = sqlc.narg(binding_epoch)
  AND session_id = sqlc.arg(session_id) AND status IN ('dispatching','sent') AND terminal_at IS NULL
  AND dispatch_token_hash = sqlc.narg(dispatch_token_hash);

-- name: AdvanceGoalTerminalCheckpoint :execrows
UPDATE loop_goal_checkpoints
SET phase = sqlc.arg(phase), updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND binding_epoch = sqlc.arg(binding_epoch)
  AND queue_entry_id = sqlc.arg(queue_entry_id) AND prompt_id = sqlc.arg(prompt_id)
  AND phase IN ('prompting','compacting');
