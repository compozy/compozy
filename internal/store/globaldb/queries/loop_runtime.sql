-- name: ListLoopCatalogAggregates :many
WITH requested_names(loop_name) AS (
  SELECT DISTINCT CAST(value AS TEXT) FROM json_each(sqlc.arg(names_json))
)
SELECT lr.loop_name, COUNT(*) AS runs,
       SUM(CASE WHEN lr.status = 'done' THEN 1 ELSE 0 END) AS succeeded,
       SUM(CASE WHEN lr.status IN ('failed', 'blocked', 'exhausted', 'stalled') THEN 1 ELSE 0 END) AS failed
FROM requested_names AS requested
JOIN loop_runs AS lr INDEXED BY idx_loop_runs_catalog
  ON lr.workspace_id = sqlc.arg(workspace_id) AND lr.loop_name = requested.loop_name
WHERE lr.created_at >= sqlc.arg(aggregate_after)
GROUP BY lr.loop_name;

-- name: ListLoopCatalogLatestRuns :many
WITH requested_names(loop_name) AS (
  SELECT DISTINCT CAST(value AS TEXT) FROM json_each(sqlc.arg(names_json))
), latest_ids(id) AS (
  SELECT (
    SELECT candidate.id FROM loop_runs AS candidate INDEXED BY idx_loop_runs_catalog
    WHERE candidate.workspace_id = sqlc.arg(workspace_id)
      AND candidate.loop_name = requested.loop_name
    ORDER BY candidate.created_at DESC, candidate.id DESC LIMIT 1
  )
  FROM requested_names AS requested
)
SELECT lr.id, lr.loop_name, lr.status, lr.created_at
FROM loop_runs AS lr JOIN latest_ids ON latest_ids.id = lr.id;

-- name: OldestQueuedLoopRunReadyForPromotion :one
SELECT q.id, q.workspace_id, q.loop_name, q.generation
FROM loop_runs q
WHERE q.workspace_id = sqlc.arg(workspace_id)
  AND q.loop_name = sqlc.arg(loop_name)
  AND q.status = 'queued'
  AND NOT EXISTS (
    SELECT 1 FROM loop_runs active
    WHERE active.workspace_id = q.workspace_id AND active.loop_name = q.loop_name
      AND active.status IN ('running', 'watching', 'needs-approval', 'paused')
  )
  AND NOT EXISTS (
    SELECT 1 FROM loop_runs older
    WHERE older.workspace_id = q.workspace_id AND older.loop_name = q.loop_name
      AND older.status = 'queued'
      AND (older.created_at < q.created_at OR (older.created_at = q.created_at AND older.id < q.id))
  )
ORDER BY q.created_at ASC, q.id ASC LIMIT 1;

-- name: ListLoopRunsMissingActiveCoordinator :many
SELECT lr.id, lr.generation
FROM loop_runs lr
WHERE lr.status = 'running'
  AND NOT EXISTS (
    SELECT 1 FROM task_runs tr
    WHERE tr.loop_run_id = lr.id AND tr.run_kind = 'coordinator'
      AND tr.status IN ('queued', 'claimed', 'starting', 'running')
  )
ORDER BY lr.created_at ASC, lr.id ASC;

-- name: ListQueuedLoopRunsReadyForPromotion :many
SELECT q.id, q.workspace_id, q.loop_name, q.generation
FROM loop_runs q
WHERE q.status = 'queued'
  AND NOT EXISTS (
    SELECT 1 FROM loop_runs active
    WHERE active.workspace_id = q.workspace_id AND active.loop_name = q.loop_name
      AND active.status IN ('running', 'watching', 'needs-approval', 'paused')
  )
  AND NOT EXISTS (
    SELECT 1 FROM loop_runs older
    WHERE older.workspace_id = q.workspace_id AND older.loop_name = q.loop_name
      AND older.status = 'queued'
      AND (older.created_at < q.created_at OR (older.created_at = q.created_at AND older.id < q.id))
  )
ORDER BY q.created_at ASC, q.id ASC;

-- name: GetLastCoordinatorTaskIDForLoopRun :one
SELECT task_id FROM task_runs
WHERE loop_run_id = sqlc.arg(loop_run_id) AND run_kind = 'coordinator'
ORDER BY queued_at DESC, id DESC LIMIT 1;

-- name: GetLastLoopTokenTick :one
SELECT payload_json, at FROM loop_run_events
WHERE loop_run_id = sqlc.arg(loop_run_id) AND kind = sqlc.arg(kind)
ORDER BY seq DESC LIMIT 1;

-- name: AdvanceLoopRunProgress :exec
UPDATE loop_runs
SET last_progress_at = CASE
  WHEN last_progress_at IS NULL OR last_progress_at < sqlc.arg(progress_at) THEN sqlc.arg(progress_at)
  ELSE last_progress_at
END
WHERE id = sqlc.arg(id);

-- name: ListLoopGenerationOutputs :many
SELECT generation, node_id, item_index, status, output_ref, task_run_id, child_loop_run_id
FROM loop_generation_outputs
WHERE loop_run_id = sqlc.arg(loop_run_id) AND generation = sqlc.arg(generation)
ORDER BY node_id ASC, item_index ASC;

-- name: GetLoopGenerationOutputStatus :one
SELECT status FROM loop_generation_outputs
WHERE loop_run_id = sqlc.arg(loop_run_id) AND task_run_id = sqlc.arg(task_run_id)
ORDER BY generation DESC LIMIT 1;

-- name: GetLoopOutputBlob :one
SELECT payload_json FROM loop_output_blobs WHERE output_ref = sqlc.arg(output_ref);

-- name: SweepOrphanedLoopOutputBlobs :exec
DELETE FROM loop_output_blobs
WHERE NOT EXISTS (
  SELECT 1 FROM loop_generation_outputs
  WHERE loop_generation_outputs.output_ref = loop_output_blobs.output_ref
)
AND NOT EXISTS (
  SELECT 1 FROM loop_goal_turns
  WHERE loop_goal_turns.evidence_ref = loop_output_blobs.output_ref
     OR loop_goal_turns.prompt_ref = loop_output_blobs.output_ref
)
AND NOT EXISTS (
  SELECT 1 FROM loop_goal_judge_attempts
  WHERE loop_goal_judge_attempts.evidence_ref = loop_output_blobs.output_ref
)
AND NOT EXISTS (
  SELECT 1 FROM loop_goal_checkpoints
  WHERE loop_goal_checkpoints.report_evidence_ref = loop_output_blobs.output_ref
);

-- name: ProjectLoopPauseToGoalCheckpoints :exec
UPDATE loop_goal_checkpoints
SET control_actor_kind = CASE WHEN sqlc.arg(requested) = 1 THEN sqlc.arg(actor_kind) ELSE NULL END,
    control_actor_id = CASE WHEN sqlc.arg(requested) = 1 THEN sqlc.arg(actor_id) ELSE NULL END,
    control_requested_at = CASE WHEN sqlc.arg(requested) = 1 THEN sqlc.arg(requested_at) ELSE NULL END,
    updated_at = sqlc.arg(updated_at)
WHERE loop_run_id = sqlc.arg(loop_run_id) AND phase NOT IN ('awaiting_control', 'terminal');

-- name: SetLoopRunPauseState :execrows
UPDATE loop_runs
SET pause_requested = sqlc.arg(requested),
    control_actor_kind = CASE WHEN sqlc.arg(requested) = 1 THEN sqlc.arg(actor_kind) ELSE NULL END,
    control_actor_id = CASE WHEN sqlc.arg(requested) = 1 THEN sqlc.arg(actor_id) ELSE NULL END,
    control_requested_at = CASE WHEN sqlc.arg(requested) = 1 THEN sqlc.arg(requested_at) ELSE NULL END
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id)
  AND status = 'running' AND pause_requested = sqlc.arg(expected_pause);

-- name: GetLoopRunPauseState :one
SELECT status, pause_requested, control_actor_kind, control_actor_id
FROM loop_runs WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id);

-- name: UpsertLoopDefinitionSnapshot :execrows
INSERT INTO loop_definition_snapshots (
  workspace_id, definition_digest, definition_version, definition_json, byte_size,
  created_at, last_used_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(definition_digest), sqlc.arg(definition_version),
  sqlc.arg(definition_json), sqlc.arg(byte_size), sqlc.arg(created_at), sqlc.arg(last_used_at)
)
ON CONFLICT(workspace_id, definition_digest) DO UPDATE SET
  last_used_at = excluded.last_used_at
WHERE loop_definition_snapshots.definition_version = excluded.definition_version
  AND loop_definition_snapshots.definition_json = excluded.definition_json
  AND loop_definition_snapshots.byte_size = excluded.byte_size;

-- name: GetLoopDefinitionSnapshot :one
SELECT workspace_id, definition_digest, definition_version, definition_json, byte_size,
       created_at, last_used_at
FROM loop_definition_snapshots
WHERE workspace_id = sqlc.arg(workspace_id) AND definition_digest = sqlc.arg(definition_digest);

-- name: ActivateLoopApproval :execrows
UPDATE loop_runs
SET active_gate_id = sqlc.arg(active_gate_id),
    active_human_criteria_json = sqlc.arg(active_human_criteria_json),
    budget_approval_seq = budget_approval_seq + sqlc.arg(budget_delta)
WHERE id = sqlc.arg(id);

-- name: UpsertLoopGateDecision :exec
INSERT INTO loop_gate_decisions (
  workspace_id, loop_run_id, generation, gate_id, criterion_id, decision,
  actor_kind, actor_ref, origin_kind, origin_ref, note, decided_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(loop_run_id), sqlc.arg(generation), sqlc.arg(gate_id),
  sqlc.arg(criterion_id), sqlc.arg(decision), sqlc.arg(actor_kind), sqlc.arg(actor_ref),
  sqlc.arg(origin_kind), sqlc.arg(origin_ref), sqlc.arg(note), sqlc.arg(decided_at)
)
ON CONFLICT(loop_run_id, generation, gate_id, criterion_id) DO UPDATE SET
  decision = excluded.decision, actor_kind = excluded.actor_kind, actor_ref = excluded.actor_ref,
  origin_kind = excluded.origin_kind, origin_ref = excluded.origin_ref,
  note = excluded.note, decided_at = excluded.decided_at;

-- name: ListLoopGateDecisions :many
SELECT criterion_id, decision, actor_kind, actor_ref, origin_kind, origin_ref, note
FROM loop_gate_decisions
WHERE workspace_id = sqlc.arg(workspace_id) AND loop_run_id = sqlc.arg(loop_run_id)
  AND generation = sqlc.arg(generation) AND gate_id = sqlc.arg(gate_id)
ORDER BY criterion_id ASC;

-- name: CompareAndSwapLoopRunStatus :execrows
UPDATE loop_runs
SET status = sqlc.arg(status),
    pause_requested = CASE WHEN sqlc.arg(clear_control_state) THEN 0 ELSE pause_requested END,
    control_actor_kind = CASE WHEN sqlc.arg(clear_control_state) THEN NULL ELSE control_actor_kind END,
    control_actor_id = CASE WHEN sqlc.arg(clear_control_state) THEN NULL ELSE control_actor_id END,
    control_requested_at = CASE WHEN sqlc.arg(clear_control_state) THEN NULL ELSE control_requested_at END,
    active_gate_id = CASE WHEN sqlc.arg(status) != 'needs-approval' THEN '' ELSE active_gate_id END,
    active_human_criteria_json = CASE WHEN sqlc.arg(status) != 'needs-approval' THEN '[]' ELSE active_human_criteria_json END
WHERE id = sqlc.arg(id) AND status = sqlc.arg(expected_status);
