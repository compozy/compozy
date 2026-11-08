-- name: GetHeartbeatAuthorityApproval :one
SELECT sha256 FROM heartbeat_authority_approvals WHERE profile_id = ? AND definition_dir = ?;

-- name: ApproveHeartbeatAuthority :exec
INSERT INTO heartbeat_authority_approvals(profile_id,definition_dir,sha256,approved_at,approved_via) VALUES (?,?,?,?,?)
ON CONFLICT(profile_id,definition_dir) DO UPDATE SET sha256=excluded.sha256,approved_at=excluded.approved_at,approved_via=excluded.approved_via;

-- name: RevokeHeartbeatAuthority :exec
DELETE FROM heartbeat_authority_approvals WHERE profile_id = ? AND definition_dir = ?;

-- name: CountCheckInToolCalls :one
SELECT count(*) AS total, CAST(coalesce(sum(CASE WHEN access='write' AND decision='allowed' THEN 1 ELSE 0 END),0) AS INTEGER) AS writes FROM agent_instance_check_tool_calls WHERE occurrence_id = ?;

-- name: InsertCheckInToolCall :exec
INSERT INTO agent_instance_check_tool_calls(instance_id,occurrence_id,seq,tool_id,backend,mcp_server,access,decision,deny_reason,outcome,effect,args_preview,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?);

-- name: FinishCheckInToolCall :exec
UPDATE agent_instance_check_tool_calls SET outcome=?,effect=?,error_code=?,duration_ms=?,updated_at=? WHERE occurrence_id=? AND seq=? AND outcome='running';

-- name: OrphanCheckInToolCalls :exec
UPDATE agent_instance_check_tool_calls SET outcome='unknown',effect='maybe',updated_at=? WHERE occurrence_id=? AND outcome='running';

-- name: ListCheckInToolCalls :many
SELECT * FROM agent_instance_check_tool_calls WHERE instance_id=? AND occurrence_id=? ORDER BY seq;

-- name: CheckInToolCallSummary :one
SELECT count(*) AS total,CAST(coalesce(sum(CASE WHEN access='write' AND decision='allowed' AND effect='done' THEN 1 ELSE 0 END),0) AS INTEGER) AS writes_done,CAST(coalesce(sum(CASE WHEN access='write' AND decision='allowed' AND effect='maybe' THEN 1 ELSE 0 END),0) AS INTEGER) AS writes_maybe,CAST(coalesce(sum(CASE WHEN decision='denied' THEN 1 ELSE 0 END),0) AS INTEGER) AS denied FROM agent_instance_check_tool_calls WHERE occurrence_id=?;

-- name: SweepCheckInToolCalls :exec
DELETE FROM agent_instance_check_tool_calls WHERE created_at < ?;

-- name: GetInstanceCheckInOccurrence :one
SELECT id,result,reason,detail,created_at,occurrence_id,tools_status FROM agent_heartbeat_wake_events WHERE instance_id=? AND occurrence_id=? ORDER BY created_at DESC,id DESC LIMIT 1;

-- name: MarkHeartbeatAuthorityNotice :execrows
INSERT INTO heartbeat_authority_approvals(profile_id,definition_dir,sha256,approved_at,approved_via,notified_sha256) VALUES (sqlc.arg(profile_id),sqlc.arg(definition_dir),'','','http',sqlc.arg(notified_sha256))
ON CONFLICT(profile_id,definition_dir) DO UPDATE SET notified_sha256=excluded.notified_sha256 WHERE heartbeat_authority_approvals.notified_sha256<>excluded.notified_sha256;

-- name: GetCheckInSessionBinding :one
SELECT instance_id,occurrence_id FROM agent_instance_wake_state WHERE triage_session_id=? AND occurrence_id<>'';
