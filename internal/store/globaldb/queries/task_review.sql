-- name: LookupRunReviewBySession :one
SELECT review_id, task_id, run_id, parent_review_id, policy, review_round, attempt,
       status, outcome, confidence, reason, delivery_id, missing_work_json,
       next_round_guidance, review_text, reviewer_session_id, reviewer_agent_name,
       reviewer_peer_id, reviewer_channel_id, reviewed_by_kind, reviewed_by_ref,
       requested_at, routed_at, started_at, reviewed_at, deadline_at, created_at, updated_at
FROM task_run_reviews
WHERE reviewer_session_id = sqlc.arg(reviewer_session_id)
  AND status IN (sqlc.arg(routed_status), sqlc.arg(in_review_status))
ORDER BY updated_at DESC, review_id DESC
LIMIT 1;

-- name: ListRunReviews :many
SELECT review_id, task_id, run_id, parent_review_id, policy, review_round, attempt,
       status, outcome, confidence, reason, delivery_id, missing_work_json,
       next_round_guidance, review_text, reviewer_session_id, reviewer_agent_name,
       reviewer_peer_id, reviewer_channel_id, reviewed_by_kind, reviewed_by_ref,
       requested_at, routed_at, started_at, reviewed_at, deadline_at, created_at, updated_at
FROM task_run_reviews
WHERE (CAST(sqlc.arg(task_id) AS TEXT) = '' OR task_id = CAST(sqlc.arg(task_id) AS TEXT))
  AND (CAST(sqlc.arg(run_id) AS TEXT) = '' OR run_id = CAST(sqlc.arg(run_id) AS TEXT))
  AND (CAST(sqlc.arg(status) AS TEXT) = '' OR status = CAST(sqlc.arg(status) AS TEXT))
  AND (CAST(sqlc.arg(reviewer_session_id) AS TEXT) = ''
       OR reviewer_session_id = CAST(sqlc.arg(reviewer_session_id) AS TEXT))
ORDER BY updated_at DESC, review_id DESC
LIMIT sqlc.arg(result_limit);

-- name: InsertRunReviewRequest :execrows
INSERT INTO task_run_reviews (
  review_id, task_id, run_id, parent_review_id, policy, review_round, attempt,
  status, outcome, confidence, reason, delivery_id, missing_work_json,
  next_round_guidance, review_text, reviewer_session_id, reviewer_agent_name,
  reviewer_peer_id, reviewer_channel_id, reviewed_by_kind, reviewed_by_ref,
  requested_at, routed_at, started_at, reviewed_at, deadline_at, created_at, updated_at
) VALUES (
  sqlc.arg(review_id), sqlc.arg(task_id), sqlc.arg(run_id), sqlc.narg(parent_review_id),
  sqlc.arg(policy), sqlc.arg(review_round), sqlc.arg(attempt), sqlc.arg(status),
  sqlc.narg(outcome), sqlc.narg(confidence), sqlc.arg(reason), sqlc.narg(delivery_id),
  sqlc.arg(missing_work_json), sqlc.arg(next_round_guidance), sqlc.arg(review_text),
  sqlc.narg(reviewer_session_id), sqlc.arg(reviewer_agent_name), sqlc.arg(reviewer_peer_id),
  sqlc.arg(reviewer_channel_id), sqlc.arg(reviewed_by_kind), sqlc.arg(reviewed_by_ref),
  sqlc.arg(requested_at), sqlc.narg(routed_at), sqlc.narg(started_at), sqlc.narg(reviewed_at),
  sqlc.narg(deadline_at), sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(run_id, review_round, attempt) DO NOTHING;

-- name: GetRunReviewByID :one
SELECT review_id, task_id, run_id, parent_review_id, policy, review_round, attempt,
       status, outcome, confidence, reason, delivery_id, missing_work_json,
       next_round_guidance, review_text, reviewer_session_id, reviewer_agent_name,
       reviewer_peer_id, reviewer_channel_id, reviewed_by_kind, reviewed_by_ref,
       requested_at, routed_at, started_at, reviewed_at, deadline_at, created_at, updated_at
FROM task_run_reviews
WHERE review_id = sqlc.arg(review_id);

-- name: GetRunReviewByRunRoundAttempt :one
SELECT review_id, task_id, run_id, parent_review_id, policy, review_round, attempt,
       status, outcome, confidence, reason, delivery_id, missing_work_json,
       next_round_guidance, review_text, reviewer_session_id, reviewer_agent_name,
       reviewer_peer_id, reviewer_channel_id, reviewed_by_kind, reviewed_by_ref,
       requested_at, routed_at, started_at, reviewed_at, deadline_at, created_at, updated_at
FROM task_run_reviews
WHERE run_id = sqlc.arg(run_id)
  AND review_round = sqlc.arg(review_round)
  AND attempt = sqlc.arg(attempt);

-- name: LinkTaskRunReviewRequest :execrows
UPDATE task_runs
SET review_required = 0,
    review_request_round = sqlc.arg(review_request_round),
    review_policy_snapshot = sqlc.arg(review_policy_snapshot),
    review_request_id = sqlc.narg(review_request_id)
WHERE id = sqlc.arg(id)
  AND (review_request_id IS NULL OR review_request_id = sqlc.narg(review_request_id));

-- name: UpdateRunReviewVerdict :execrows
UPDATE task_run_reviews
SET status = sqlc.arg(status), outcome = sqlc.narg(outcome), confidence = sqlc.narg(confidence),
    reason = sqlc.arg(reason), delivery_id = sqlc.narg(delivery_id),
    missing_work_json = sqlc.arg(missing_work_json),
    next_round_guidance = sqlc.arg(next_round_guidance), review_text = sqlc.arg(review_text),
    reviewed_by_kind = sqlc.arg(reviewed_by_kind), reviewed_by_ref = sqlc.arg(reviewed_by_ref),
    reviewed_at = sqlc.narg(reviewed_at), updated_at = sqlc.arg(updated_at)
WHERE review_id = sqlc.arg(review_id);

-- name: UpdateTaskReviewRollup :execrows
UPDATE tasks
SET review_round = sqlc.arg(review_round),
    last_review_id = sqlc.narg(last_review_id),
    last_review_outcome = sqlc.arg(last_review_outcome),
    review_circuit_opened_at = sqlc.narg(review_circuit_opened_at),
    review_circuit_reason = sqlc.arg(review_circuit_reason),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: GetTaskRunIDByReviewID :one
SELECT id FROM task_runs WHERE review_id = sqlc.arg(review_id);

-- name: UpdateRunReviewBinding :execrows
UPDATE task_run_reviews
SET status = sqlc.arg(status),
    reviewer_session_id = sqlc.narg(reviewer_session_id),
    reviewer_agent_name = sqlc.arg(reviewer_agent_name),
    reviewer_peer_id = sqlc.arg(reviewer_peer_id),
    reviewer_channel_id = sqlc.arg(reviewer_channel_id),
    started_at = COALESCE(started_at, sqlc.narg(started_at)),
    updated_at = sqlc.arg(updated_at)
WHERE review_id = sqlc.arg(review_id);
