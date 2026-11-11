-- name: GetProposalPick :one
SELECT p.*, COALESCE(q.status,'failed') AS queue_status FROM agent_instance_proposal_picks p
LEFT JOIN session_input_queue q ON q.id=p.queue_entry_id WHERE p.report_id=? AND p.proposal_index=?;

-- name: ListProposalPicks :many
SELECT p.*, COALESCE(q.status,'failed') AS queue_status FROM agent_instance_proposal_picks p
LEFT JOIN session_input_queue q ON q.id=p.queue_entry_id WHERE p.report_id=? ORDER BY p.proposal_index;

-- name: InsertProposalPick :exec
INSERT INTO agent_instance_proposal_picks(report_id,proposal_index,instance_id,action_id,actor,target_session_id,queue_entry_id,picked_at)
VALUES(?,?,?,?,?,?,?,?);

-- name: CountAutoPicks :one
SELECT COUNT(*) FROM agent_instance_proposal_picks WHERE instance_id=? AND actor='auto_accept' AND picked_at>?;

-- name: CancelInstanceAutoPicks :exec
UPDATE session_input_queue SET status='canceled', canceled_at=?, updated_at=?
WHERE status='queued' AND owner_kind='synthetic' AND message_id LIKE 'pick:%'
AND id IN (SELECT queue_entry_id FROM agent_instance_proposal_picks WHERE instance_id=? AND actor='auto_accept');

-- name: GetInstanceReportForPick :one
SELECT * FROM agent_instance_reports WHERE id=? AND instance_id=?;
