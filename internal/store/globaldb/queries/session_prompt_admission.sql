-- name: GetSessionPromptAdmissionByIdempotencyKey :one
SELECT * FROM session_prompt_admissions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND session_id = sqlc.arg(session_id)
  AND idempotency_key = sqlc.arg(idempotency_key);

-- name: GetSessionPromptAdmissionByMessageID :one
SELECT * FROM session_prompt_admissions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND session_id = sqlc.arg(session_id)
  AND message_id = sqlc.arg(message_id);

-- name: InsertSessionPromptAdmission :exec
INSERT INTO session_prompt_admissions (
  id, workspace_id, session_id, message_id, idempotency_key, operation,
  fingerprint_version, request_fingerprint, state, mode, authored_text,
	 skill_invocations_json, attachments_json,
  runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed,
  turn_id, event_id, created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(session_id), sqlc.arg(message_id),
  sqlc.arg(idempotency_key), sqlc.arg(operation), sqlc.arg(fingerprint_version),
  sqlc.arg(request_fingerprint), sqlc.arg(state), sqlc.arg(mode), sqlc.arg(authored_text),
	 sqlc.arg(skill_invocations_json), sqlc.arg(attachments_json),
  sqlc.arg(runtime_provider), sqlc.arg(runtime_model), sqlc.arg(runtime_reasoning_effort),
  sqlc.arg(runtime_speed), sqlc.arg(turn_id), sqlc.arg(event_id),
  sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: CommitSessionPromptDispatch :execrows
UPDATE session_prompt_admissions
SET state = sqlc.arg(dispatch_committed_state),
    authored_text = '', runtime_provider = '', runtime_model = '',
    runtime_reasoning_effort = '', runtime_speed = '',
    dispatch_committed_at = sqlc.arg(dispatch_committed_at), updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND session_id = sqlc.arg(session_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
  AND state = sqlc.arg(reserved_state);

-- name: CompleteSessionPromptAdmission :execrows
UPDATE session_prompt_admissions
SET state = sqlc.arg(completed_state), result_json = sqlc.arg(result_json),
    authored_text = '', runtime_provider = '', runtime_model = '',
    runtime_reasoning_effort = '', runtime_speed = '',
    completed_at = sqlc.arg(completed_at), updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND session_id = sqlc.arg(session_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
  AND state = sqlc.arg(expected_state);

-- name: MarkSessionPromptAdmissionIndeterminate :execrows
UPDATE session_prompt_admissions
SET state = sqlc.arg(indeterminate_state), indeterminate_reason = sqlc.arg(indeterminate_reason),
    authored_text = '', runtime_provider = '', runtime_model = '',
    runtime_reasoning_effort = '', runtime_speed = '', updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND session_id = sqlc.arg(session_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
  AND state IN (sqlc.arg(dispatch_committed_state), sqlc.arg(completed_state));

-- name: CommitQueuedSessionPromptAdmissionDispatch :execrows
UPDATE session_prompt_admissions
SET state = sqlc.arg(dispatch_committed_state),
    dispatch_committed_at = sqlc.arg(dispatch_committed_at), completed_at = NULL,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND session_id = sqlc.arg(session_id)
  AND state = sqlc.arg(completed_state);

-- name: CompleteQueuedSessionPromptAdmissionDispatch :execrows
UPDATE session_prompt_admissions
SET state = sqlc.arg(completed_state), completed_at = sqlc.arg(completed_at),
    indeterminate_reason = '', updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND session_id = sqlc.arg(session_id)
  AND state = sqlc.arg(dispatch_committed_state);

-- name: MarkQueuedSessionPromptAdmissionIndeterminate :execrows
UPDATE session_prompt_admissions
SET state = sqlc.arg(indeterminate_state), indeterminate_reason = sqlc.arg(indeterminate_reason),
    completed_at = NULL, updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND session_id = sqlc.arg(session_id)
  AND state = sqlc.arg(dispatch_committed_state);

-- name: GetQueuedSessionPromptAdmissionState :one
SELECT state FROM session_prompt_admissions
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id);
