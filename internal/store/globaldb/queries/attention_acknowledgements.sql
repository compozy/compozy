-- name: ListAcknowledgedAttention :many
SELECT occurrence_id FROM attention_acknowledgements
WHERE profile_id = sqlc.arg(profile_id)
  AND actor_kind = sqlc.arg(actor_kind)
  AND actor_id = sqlc.arg(actor_id)
  AND occurrence_id IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(occurrence_ids)));

-- name: ListUnreadAutomationFailures :many
SELECT CAST('automation-failure:' || run.id AS TEXT) AS occurrence_id,
       run.id AS run_id, run.job_id, run.trigger_id, run.workspace_id,
       CAST(COALESCE(run.ended_at, run.started_at) AS TEXT) AS occurred_at
FROM automation_runs AS run
WHERE run.status = 'failed'
  AND run.id IS NOT NULL
  AND (run.job_id IS NOT NULL OR run.trigger_id IS NOT NULL)
  AND (run.ended_at IS NOT NULL OR run.started_at IS NOT NULL)
  AND (CAST(sqlc.arg(all_profiles) AS BOOLEAN) OR run.profile_id = sqlc.arg(source_profile_id))
  AND (
    run.workspace_id = ''
    OR run.workspace_id IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(workspace_ids)))
    OR (run.workspace_id IS NULL AND (
      EXISTS (SELECT 1 FROM automation_jobs AS job WHERE job.id = run.job_id AND job.scope = 'global')
      OR EXISTS (SELECT 1 FROM automation_triggers AS trigger
                 WHERE trigger.id = run.trigger_id AND trigger.scope = 'global')
    ))
  )
  AND NOT EXISTS (
    SELECT 1 FROM attention_acknowledgements AS receipt
    WHERE receipt.profile_id = sqlc.arg(receipt_profile_id)
      AND receipt.actor_kind = sqlc.arg(actor_kind)
      AND receipt.actor_id = sqlc.arg(actor_id)
      AND receipt.occurrence_id = 'automation-failure:' || run.id
  );

-- name: SaveAttentionSnapshot :exec
INSERT INTO attention_snapshots (id, profile_id, actor_kind, actor_id, population, occurrence_ids, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET expires_at = excluded.expires_at;

-- name: DeleteExpiredAttentionSnapshots :exec
DELETE FROM attention_snapshots WHERE id IN (
    SELECT expired.id FROM attention_snapshots AS expired WHERE expired.expires_at < sqlc.arg(now)
    ORDER BY expired.expires_at LIMIT 256
);

-- name: GetAttentionSnapshot :one
SELECT occurrence_ids FROM attention_snapshots
WHERE id = sqlc.arg(id) AND profile_id = sqlc.arg(profile_id)
  AND actor_kind = sqlc.arg(actor_kind) AND actor_id = sqlc.arg(actor_id)
  AND population = sqlc.arg(population) AND expires_at >= sqlc.arg(now);

-- name: AcknowledgeAttentionOccurrences :exec
INSERT INTO attention_acknowledgements (profile_id, actor_kind, actor_id, occurrence_id, acknowledged_at)
SELECT sqlc.arg(profile_id), sqlc.arg(actor_kind), sqlc.arg(actor_id), CAST(value AS TEXT), sqlc.arg(now)
FROM json_each(sqlc.arg(occurrence_ids)) WHERE true
ON CONFLICT(profile_id, actor_kind, actor_id, occurrence_id) DO NOTHING;
