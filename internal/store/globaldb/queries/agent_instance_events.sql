-- name: ListPendingAgentInstanceEvents :many
SELECT e.*, CAST(coalesce(json_extract(r.metadata_json, '$.instance_event_instruction'), t.prompt, '') AS TEXT) AS instruction,
CAST(coalesce(json_extract(r.metadata_json, '$.instance_event_trigger_name'), t.name, '') AS TEXT) AS trigger_name, CAST(coalesce(json_extract(r.metadata_json, '$.instance_event_event'), t.event, '') AS TEXT) AS event
FROM agent_instance_events e LEFT JOIN automation_triggers t ON t.id = e.trigger_id
LEFT JOIN automation_runs r ON r.id = e.run_id
WHERE e.instance_id = ? AND e.consumed_at IS NULL
ORDER BY e.received_at, e.id LIMIT 20;

-- name: RecordInstanceEventBatch :exec
UPDATE agent_instance_events SET occurrence_id = sqlc.arg(occurrence_id)
WHERE instance_id = sqlc.arg(instance_id) AND consumed_at IS NULL
AND id IN (SELECT value FROM json_each(sqlc.arg(event_ids)));

-- name: ConsumeInstanceEventBatch :exec
UPDATE agent_instance_events SET consumed_at = sqlc.arg(now)
WHERE instance_id = sqlc.arg(instance_id) AND occurrence_id = sqlc.arg(occurrence_id) AND consumed_at IS NULL;

-- name: InsertAgentInstanceEvent :exec
INSERT INTO agent_instance_events (id, instance_id, trigger_id, run_id, summary, received_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetInstanceEventReceipt :one
SELECT coalesce(CAST(json_extract(metadata_json, '$.instance_event_id') AS TEXT), '') AS event_id,
coalesce(CAST(json_extract(metadata_json, '$.instance_event_instance_id') AS TEXT), '') AS instance_id
FROM automation_runs WHERE id = ? AND profile_id = ? AND trigger_id = ?;

-- name: RecordInstanceEventReceipt :exec
UPDATE automation_runs SET metadata_json = json_set(metadata_json,
'$.instance_event_id', sqlc.arg(event_id),
'$.instance_event_trigger_name', sqlc.arg(trigger_name), '$.instance_event_event', sqlc.arg(event),
'$.instance_event_instruction', sqlc.arg(instruction), '$.instance_event_instance_id', sqlc.arg(instance_id))
WHERE id = sqlc.arg(run_id);

-- name: CountOmittedInstanceEvents :one
SELECT count(*) FROM agent_instance_events
WHERE agent_instance_events.instance_id = sqlc.arg(instance_id) AND consumed_at IS NULL
AND id NOT IN (SELECT retained.id FROM agent_instance_events AS retained WHERE retained.instance_id = sqlc.arg(instance_id) ORDER BY retained.received_at DESC, retained.id DESC LIMIT 200);

-- name: TrimInstanceEvents :exec
DELETE FROM agent_instance_events WHERE agent_instance_events.instance_id = sqlc.arg(instance_id)
AND id NOT IN (SELECT retained.id FROM agent_instance_events AS retained WHERE retained.instance_id = sqlc.arg(instance_id) ORDER BY retained.received_at DESC, retained.id DESC LIMIT 200);

-- name: AddOmittedInstanceEvents :exec
UPDATE agent_instance_wake_state SET events_omitted = events_omitted + sqlc.arg(omitted)
WHERE instance_id = sqlc.arg(instance_id);

-- name: PullForwardInstanceEventWake :exec
UPDATE agent_instance_wake_state SET next_wake_at = sqlc.arg(next_wake_at)
WHERE instance_id = sqlc.arg(instance_id) AND (next_wake_at = '' OR next_wake_at > sqlc.arg(next_wake_at));

-- name: LatestInstanceEventCheckReason :one
SELECT reason FROM agent_heartbeat_wake_events WHERE instance_id = ? AND result <> 'skipped'
ORDER BY created_at DESC, id DESC LIMIT 1;
