-- name: InsertAgentInstanceReport :exec
INSERT INTO agent_instance_reports (id, instance_id, source_key, kind, payload, fingerprint, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(source_key) DO NOTHING;

-- name: GetAgentInstanceReportBySource :one
SELECT * FROM agent_instance_reports WHERE source_key = ? AND instance_id = ?;

-- name: ListAgentInstanceReports :many
SELECT * FROM agent_instance_reports
WHERE instance_id = sqlc.arg(instance_id)
AND (sqlc.arg(report_state) = '' OR state = sqlc.arg(report_state))
AND (created_at > sqlc.arg(after_created_at) OR (created_at = sqlc.arg(after_created_at) AND id > sqlc.arg(after_id)))
ORDER BY created_at, id LIMIT sqlc.arg(page_limit);

-- name: ListRecentAgentInstanceReports :many
SELECT * FROM agent_instance_reports
WHERE instance_id = sqlc.arg(instance_id)
ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(page_limit);

-- name: ListUnpublishedAgentInstanceReports :many
SELECT r.*, i.main_session_id FROM agent_instance_reports r
JOIN agent_instances i ON i.id = r.instance_id
WHERE r.id > sqlc.arg(after_id) AND i.main_session_id IS NOT NULL
AND (r.event_session_id = '' OR (r.state = 'pending' AND r.event_session_id <> i.main_session_id))
ORDER BY r.id LIMIT sqlc.arg(page_limit);

-- name: MarkAgentInstanceReportPublished :exec
UPDATE agent_instance_reports SET event_session_id = sqlc.arg(session_id), event_seq = sqlc.arg(event_seq)
WHERE agent_instance_reports.id = sqlc.arg(id)
AND EXISTS (SELECT 1 FROM agent_instances i WHERE i.id = agent_instance_reports.instance_id AND i.main_session_id = sqlc.arg(session_id));

-- name: ReservePromptReports :exec
UPDATE agent_instance_reports SET state = 'reserved', prompt_admission_id = sqlc.arg(admission_id)
WHERE id IN (
 SELECT r.id FROM agent_instance_reports r
 JOIN agent_instances i ON i.id = r.instance_id
 WHERE i.main_session_id = sqlc.arg(session_id)
 AND r.event_session_id = sqlc.arg(session_id) AND r.event_seq > 0 AND r.state = 'pending'
 ORDER BY r.created_at, r.id LIMIT 20
);

-- name: ReleasePromptReports :exec
UPDATE agent_instance_reports SET state = 'pending', prompt_admission_id = ''
WHERE prompt_admission_id = sqlc.arg(admission_id) AND state = 'reserved';

-- name: ListPromptReports :many
SELECT * FROM agent_instance_reports WHERE prompt_admission_id = sqlc.arg(admission_id)
ORDER BY created_at, id;

-- name: ReleaseCanceledSessionReports :exec
UPDATE agent_instance_reports SET state = 'pending', prompt_admission_id = ''
WHERE state = 'reserved' AND prompt_admission_id IN (
 SELECT prompt_admission_id FROM session_input_queue
 WHERE session_id = sqlc.arg(session_id) AND status = 'canceled'
);

-- name: DeliverPromptReport :execrows
UPDATE agent_instance_reports SET state = 'delivered', delivered_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'reserved';

-- name: FailPromptReports :exec
UPDATE agent_instance_reports SET
 state = CASE WHEN state = 'reserved' THEN 'pending' ELSE 'indeterminate' END,
 prompt_admission_id = CASE WHEN state = 'reserved' THEN '' ELSE prompt_admission_id END
WHERE prompt_admission_id = sqlc.arg(admission_id) AND state IN ('reserved', 'delivered');

-- name: RecoverReservedPromptReports :exec
UPDATE agent_instance_reports SET state = 'pending', prompt_admission_id = '' WHERE state = 'reserved';

-- name: RecoverUncertainPromptReports :exec
UPDATE agent_instance_reports SET state = 'indeterminate'
WHERE state = 'delivered' AND prompt_admission_id IN (
 SELECT id FROM session_prompt_admissions WHERE state = 'dispatch_committed'
);

-- name: ListIndeterminatePromptReports :many
SELECT r.id, r.created_at, a.session_id, a.turn_id FROM agent_instance_reports r
JOIN session_prompt_admissions a ON a.id = r.prompt_admission_id
WHERE r.state = 'indeterminate' AND r.recovery_noted = 0 AND r.id > sqlc.arg(after_id)
ORDER BY r.id LIMIT 100;

-- name: SweepTerminalInstanceReports :execrows
DELETE FROM agent_instance_reports WHERE id IN (
 SELECT r.id FROM agent_instance_reports r
 WHERE r.state IN ('delivered', 'indeterminate')
 AND COALESCE(r.delivered_at, r.created_at) < sqlc.arg(cutoff)
 ORDER BY COALESCE(r.delivered_at, r.created_at), r.id LIMIT sqlc.arg(batch_limit)
);

-- name: MarkIndeterminateReportNoted :exec
UPDATE agent_instance_reports SET recovery_noted = 1 WHERE id = ? AND state = 'indeterminate';
