-- name: InsertInstanceWakeState :exec
INSERT INTO agent_instance_wake_state (instance_id) VALUES (?) ON CONFLICT DO NOTHING;

-- name: GetInstanceWakeState :one
SELECT * FROM agent_instance_wake_state WHERE instance_id = ?;

-- name: ListDueInstanceWakes :many
SELECT w.* FROM agent_instance_wake_state w JOIN agent_instances i ON i.id=w.instance_id
WHERE i.state='active' AND i.archived_at IS NULL AND i.checks_enabled=1
AND w.occurrence_id='' AND w.next_wake_at<=sqlc.arg(now)
ORDER BY w.next_wake_at,w.instance_id LIMIT sqlc.arg(page_limit);

-- name: ListInflightInstanceWakes :many
SELECT * FROM agent_instance_wake_state WHERE occurrence_id<>'' ORDER BY instance_id;

-- name: AdmitInstanceWake :execrows
UPDATE agent_instance_wake_state
SET occurrence_id=sqlc.arg(occurrence_id), occurrence_started_at=sqlc.arg(now),
triage_session_id='', occurrence_cancelled=0,
next_wake_at=CASE WHEN sqlc.arg(manual_check)=1 THEN next_wake_at ELSE sqlc.arg(next_wake_at) END
WHERE agent_instance_wake_state.instance_id=sqlc.arg(instance_id) AND occurrence_id=''
AND (SELECT count(*) FROM agent_instance_wake_state AS inflight WHERE inflight.occurrence_id<>'') < CAST(sqlc.arg(max_concurrent) AS INTEGER)
AND EXISTS (SELECT 1 FROM agent_instances i WHERE i.id=agent_instance_wake_state.instance_id AND i.state='active'
AND i.archived_at IS NULL AND i.checks_enabled=1)
AND (sqlc.arg(manual_check)=1 OR next_wake_at<=sqlc.arg(now));

-- name: SkipInstanceWake :execrows
UPDATE agent_instance_wake_state SET next_wake_at=sqlc.arg(next_wake_at)
WHERE agent_instance_wake_state.instance_id=sqlc.arg(instance_id) AND occurrence_id='' AND next_wake_at=sqlc.arg(expected_due)
AND next_wake_at<=sqlc.arg(now);

-- name: BindInstanceTriage :execrows
UPDATE agent_instance_wake_state SET triage_session_id=sqlc.arg(session_id)
WHERE instance_id=sqlc.arg(instance_id) AND occurrence_id=sqlc.arg(occurrence_id)
AND triage_session_id='' AND occurrence_cancelled=0;

-- name: CancelInstanceWake :exec
UPDATE agent_instance_wake_state SET occurrence_cancelled=1 WHERE instance_id=? AND occurrence_id<>'';

-- name: FinishInstanceWake :execrows
UPDATE agent_instance_wake_state SET occurrence_id='',triage_session_id='',occurrence_started_at='',occurrence_cancelled=0,
last_wake_at=sqlc.arg(now),
last_digest=CASE WHEN sqlc.arg(completed)=1 THEN sqlc.arg(digest) ELSE last_digest END,
last_digest_at=CASE WHEN sqlc.arg(completed)=1 THEN sqlc.arg(now) ELSE last_digest_at END,
automation_cursor=CASE WHEN sqlc.arg(completed)=1 THEN sqlc.arg(automation_cursor) ELSE automation_cursor END,
quiet_streak=sqlc.arg(quiet_streak),failure_streak=sqlc.arg(failure_streak),
next_wake_at=CASE WHEN sqlc.arg(next_wake_at)='' THEN next_wake_at ELSE sqlc.arg(next_wake_at) END
WHERE instance_id=sqlc.arg(instance_id) AND occurrence_id=sqlc.arg(occurrence_id);

-- name: InsertInstanceWakeEvent :exec
INSERT INTO agent_heartbeat_wake_events
(id,workspace_id,agent_name,source,result,reason,detail,denied_calls,occurrence_id,instance_id,created_at,expires_at)
SELECT sqlc.arg(id),workspace_id,agent_name,sqlc.arg(source),sqlc.arg(result),sqlc.arg(reason),
sqlc.arg(detail),sqlc.arg(denied_calls),sqlc.arg(occurrence_id),sqlc.arg(instance_id),sqlc.arg(created_at),sqlc.arg(expires_at)
FROM agent_instances WHERE agent_instances.id=sqlc.arg(instance_id);

-- name: SetInstanceChecks :execrows
UPDATE agent_instances SET checks_enabled=sqlc.arg(enabled), updated_at=sqlc.arg(now)
WHERE id=sqlc.arg(instance_id) AND archived_at IS NULL AND checks_enabled<>sqlc.arg(enabled);

-- name: ScheduleInstanceChecks :exec
UPDATE agent_instance_wake_state SET next_wake_at=sqlc.arg(next_wake_at)
WHERE instance_id=sqlc.arg(instance_id);

-- name: GetInstanceOccurrence :one
SELECT 'running' AS state, '' AS result, '' AS reason, '' AS report_id
FROM agent_instance_wake_state w WHERE w.instance_id=sqlc.arg(instance_id) AND w.occurrence_id=sqlc.arg(occurrence_id) AND w.occurrence_id<>''
UNION ALL
SELECT 'finished' AS state, e.result, e.reason, coalesce(r.id, '') AS report_id
FROM agent_heartbeat_wake_events e
LEFT JOIN agent_instance_reports r ON r.instance_id=e.instance_id AND r.source_key='occ:' || e.occurrence_id
WHERE e.instance_id=sqlc.arg(instance_id) AND e.occurrence_id=sqlc.arg(occurrence_id) AND e.occurrence_id<>''
LIMIT 1;

-- name: ListInstanceCheckIns :many
SELECT e.id, e.result, e.reason, e.created_at FROM agent_heartbeat_wake_events e
WHERE e.instance_id=sqlc.arg(instance_id)
AND (sqlc.arg(cursor)='' OR (e.created_at, e.id) <
 (SELECT c.created_at, c.id FROM agent_heartbeat_wake_events c WHERE c.instance_id=sqlc.arg(instance_id) AND c.id=sqlc.arg(cursor)))
ORDER BY e.created_at DESC, e.id DESC LIMIT sqlc.arg(page_limit);
