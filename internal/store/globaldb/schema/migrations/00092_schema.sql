-- +goose Up
-- add column "completed_at" to table: "loop_runs"
ALTER TABLE `loop_runs` ADD COLUMN `completed_at` timestamp NULL;

UPDATE loop_runs
SET completed_at = (
  SELECT event.at
  FROM loop_run_events AS event
  WHERE event.workspace_id = loop_runs.workspace_id
    AND event.loop_run_id = loop_runs.id
    AND event.kind = 'status_changed'
    AND json_extract(event.payload_json, '$.to') IN (
      'done', 'no-op', 'blocked', 'failed', 'exhausted', 'stalled', 'canceled'
    )
  ORDER BY event.seq DESC
  LIMIT 1
)
WHERE loop_runs.status IN (
  'done', 'no-op', 'blocked', 'failed', 'exhausted', 'stalled', 'canceled'
);
