-- +goose Up
ALTER TABLE loop_runs DROP COLUMN consecutive_failures;

CREATE INDEX idx_task_events_wake_event
ON task_events(task_id, event_type, json_extract(payload_json, '$.wake_event_id'))
WHERE event_type IN ('task.wake.delivered', 'task.wake.suppressed');
