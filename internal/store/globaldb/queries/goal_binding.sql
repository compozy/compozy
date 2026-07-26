-- name: GetActiveGoalSessionBinding :one
SELECT loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
       creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
       failure_code, created_at, activated_at, failed_at, closed_at,
       adopted_generation, adoption_attempt_id
FROM loop_session_bindings
WHERE loop_run_id = sqlc.arg(loop_run_id) AND workspace_id = sqlc.arg(workspace_id)
  AND handle = sqlc.arg(handle) AND state = 'active';

-- name: FindGoalSessionBindingAttempt :one
SELECT loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
       creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
       failure_code, created_at, activated_at, failed_at, closed_at,
       adopted_generation, adoption_attempt_id
FROM loop_session_bindings
WHERE loop_run_id = sqlc.arg(loop_run_id) AND workspace_id = sqlc.arg(workspace_id)
  AND handle = sqlc.arg(handle) AND binding_epoch = sqlc.arg(binding_epoch);

-- name: CloseGoalBindingWithCleanup :execrows
UPDATE loop_session_bindings
SET state = 'closed', closed_at = CAST(sqlc.arg(closed_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND binding_epoch = sqlc.arg(binding_epoch) AND session_id = sqlc.arg(session_id)
  AND state = 'active';

-- name: SettleStoppedGoalBindingCreationForSession :execrows
UPDATE loop_session_bindings
SET failure_code = sqlc.arg(settled_failure_code)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND binding_epoch = sqlc.arg(binding_epoch) AND session_id = sqlc.arg(session_id)
  AND state = 'failed' AND failure_code = sqlc.arg(unsettled_failure_code);

-- name: InsertGoalBindingRetryWitness :exec
INSERT INTO loop_goal_binding_retry_witnesses (
    loop_run_id, handle, failed_binding_epoch, request_digest, created_at
) VALUES (
    sqlc.arg(loop_run_id), sqlc.arg(handle), sqlc.arg(failed_binding_epoch),
    sqlc.arg(request_digest), CAST(sqlc.arg(created_at) AS TEXT)
);

-- name: GetGoalBindingRetryWitnessDigest :one
SELECT request_digest
FROM loop_goal_binding_retry_witnesses
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND failed_binding_epoch = sqlc.arg(failed_binding_epoch);

-- name: FailGoalBindingCreationAttempt :execrows
UPDATE loop_session_bindings
SET state = 'failed', failure_code = sqlc.arg(failure_code), failed_at = CAST(sqlc.arg(failed_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND binding_epoch = sqlc.arg(binding_epoch) AND state = 'creating';

-- name: InsertGoalSessionBindingAttempt :exec
INSERT INTO loop_session_bindings (
    loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
    creation_profile_ref, policy_spec_digest, creation_digest, ownership, state, created_at
) VALUES (
    sqlc.arg(loop_run_id), sqlc.arg(handle), sqlc.arg(binding_epoch), sqlc.arg(binding_attempt_id),
    sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(creation_profile_ref),
    sqlc.arg(policy_spec_digest), sqlc.arg(creation_digest), 'run-owned', 'creating', CAST(sqlc.arg(created_at) AS TEXT)
);

-- name: AdvanceGoalCheckpointBindingAttempt :execrows
UPDATE loop_goal_checkpoints
SET prompt_attempt = prompt_attempt + 1, binding_handle = sqlc.arg(binding_handle),
    binding_epoch = sqlc.arg(binding_epoch), session_id = sqlc.arg(session_id),
    updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch) AND phase = sqlc.arg(phase)
  AND prompt_attempt = sqlc.arg(prompt_attempt)
  AND COALESCE(task_run_id, '') = sqlc.arg(task_run_id)
  AND COALESCE(queue_entry_id, '') = sqlc.arg(queue_entry_id)
  AND COALESCE(prompt_id, '') = sqlc.arg(prompt_id)
  AND COALESCE(binding_epoch, 0) = sqlc.arg(expected_binding_epoch)
  AND COALESCE(session_id, '') = sqlc.arg(expected_session_id)
  AND COALESCE(binding_handle, '') = sqlc.arg(expected_binding_handle);

-- name: SettleStoppedGoalBindingBeforeActivation :execrows
UPDATE loop_session_bindings
SET failure_code = sqlc.arg(settled_failure_code)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND binding_epoch = sqlc.arg(binding_epoch) AND state = 'failed'
  AND failure_code = sqlc.arg(unsettled_failure_code);

-- name: ResetGoalRecoveryAfterBindingActivation :execrows
UPDATE loop_goal_checkpoints
SET recovery_streak = 0, compaction_baseline_used = NULL,
    compaction_recovery_required = 0, updated_at = CAST(sqlc.arg(updated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
  AND node_id = sqlc.arg(node_id) AND item_index = sqlc.arg(item_index)
  AND control_epoch = sqlc.arg(control_epoch);

-- name: RetireGoalBinding :execrows
UPDATE loop_session_bindings
SET state = sqlc.arg(state), closed_at = CAST(sqlc.arg(closed_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND binding_epoch = sqlc.arg(binding_epoch) AND state = 'active';

-- name: ActivateGoalBinding :execrows
UPDATE loop_session_bindings
SET state = 'active', activated_at = CAST(sqlc.arg(activated_at) AS TEXT)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND binding_epoch = sqlc.arg(binding_epoch) AND state = 'creating';

-- name: InsertOriginGoalSessionBinding :exec
INSERT INTO loop_session_bindings (
    loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
    creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
    created_at, activated_at, adopted_generation, adoption_attempt_id
) VALUES (
    sqlc.arg(loop_run_id), sqlc.arg(handle), 1, sqlc.arg(binding_attempt_id), sqlc.arg(session_id),
    sqlc.arg(workspace_id), sqlc.arg(creation_profile_ref), sqlc.arg(policy_spec_digest),
    sqlc.arg(creation_digest), 'origin-borrowed', 'active', CAST(sqlc.arg(created_at) AS TEXT),
    CAST(sqlc.arg(activated_at) AS TEXT), sqlc.arg(adopted_generation), sqlc.arg(adoption_attempt_id)
);

-- name: GetGoalOriginBindingAdoptionOwner :one
SELECT status, generation
FROM loop_runs
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id);

-- name: AdvanceGoalOriginBindingAdoption :execrows
UPDATE loop_session_bindings
SET adopted_generation = sqlc.arg(adopted_generation), adoption_attempt_id = sqlc.arg(adoption_attempt_id)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND binding_epoch = 1 AND ownership = 'origin-borrowed' AND state = 'active'
  AND adopted_generation = sqlc.arg(expected_adopted_generation);

-- name: ReactivateGoalOriginBinding :execrows
UPDATE loop_session_bindings
SET state = 'active', closed_at = NULL, adopted_generation = sqlc.arg(adopted_generation),
    adoption_attempt_id = sqlc.arg(adoption_attempt_id)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle)
  AND binding_epoch = 1 AND ownership = 'origin-borrowed' AND state = 'closed'
  AND adopted_generation = sqlc.arg(expected_adopted_generation);

-- name: GetMaxGoalBindingEpoch :one
SELECT CAST(COALESCE(MAX(binding_epoch), 0) AS INTEGER)
FROM loop_session_bindings
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle);

-- name: CountCreatingGoalBindings :one
SELECT COUNT(*)
FROM loop_session_bindings
WHERE loop_run_id = sqlc.arg(loop_run_id) AND handle = sqlc.arg(handle) AND state = 'creating';

-- name: ListActiveGoalBindingsForStop :many
SELECT handle, binding_epoch, session_id
FROM loop_session_bindings
WHERE loop_run_id = sqlc.arg(loop_run_id) AND workspace_id = sqlc.arg(workspace_id)
  AND state = 'active'
ORDER BY handle ASC, binding_epoch ASC;

-- name: ListCreatingGoalBindingsForStop :many
SELECT handle, binding_epoch
FROM loop_session_bindings
WHERE loop_run_id = sqlc.arg(loop_run_id) AND workspace_id = sqlc.arg(workspace_id)
  AND ownership = 'run-owned' AND state = 'creating'
ORDER BY handle ASC, binding_epoch ASC;
