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
