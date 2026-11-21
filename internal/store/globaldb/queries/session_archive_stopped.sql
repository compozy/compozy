-- name: ListArchiveStoppedMembers :many
-- Archive stopped threads in one profile scope: every unarchived thread root of one workspace
-- (workspace_id set) or of every unarchived workspace (workspace_id empty), optionally one root
-- (root_id set), with each unarchived member and the facts that block archiving it. Members follow
-- ListSessionTreeDescendantIDs; roots are sessions no member walk reaches. Root first, shallowest first.
WITH RECURSIVE thread(root_id,id,in_side,depth) AS (
 SELECT root.id,root.id,root.lineage_kind = 'side',0 FROM sessions root
 WHERE root.profile_id = sqlc.arg(profile_id) AND root.archived_at IS NULL
  AND ((sqlc.arg(workspace_id) <> '' AND root.workspace_id = sqlc.arg(workspace_id))
   OR (sqlc.arg(workspace_id) = '' AND root.workspace_id IN (SELECT w.id FROM workspaces w WHERE w.archived_at IS NULL)))
  AND (sqlc.arg(root_id) = '' OR root.id = sqlc.arg(root_id))
  AND LOWER(TRIM(COALESCE(root.spawn_role,''))) <> 'auto-title'
  AND NOT EXISTS (
   SELECT 1 FROM sessions parent WHERE parent.id = root.parent_session_id
    AND parent.workspace_id = root.workspace_id AND parent.profile_id = root.profile_id
    AND root.lineage_kind NOT IN ('continue','fork'))
 UNION
 SELECT thread.root_id,child.id,thread.in_side OR child.lineage_kind = 'side',thread.depth + 1
 FROM sessions child JOIN thread ON child.parent_session_id = thread.id
 JOIN sessions parent ON parent.id = thread.id
 WHERE child.workspace_id = parent.workspace_id AND child.profile_id = parent.profile_id
  AND child.lineage_kind NOT IN ('continue','fork')
  AND (thread.in_side OR LOWER(TRIM(COALESCE(child.spawn_role,''))) <> 'auto-title')
)
SELECT CAST(thread.root_id AS TEXT) AS root_id,member.id,member.workspace_id,
 CAST(COALESCE(member.name,'') AS TEXT) AS name,member.state,
 CAST(EXISTS(SELECT 1 FROM agent_instances a WHERE a.main_session_id = member.id AND a.archived_at IS NULL) AS BOOLEAN) AS instance_main,
 CAST(member.pending_permission_count > 0 OR member.pending_clarify_count > 0 OR EXISTS(
  SELECT 1 FROM session_pending_interactions i WHERE i.session_id = member.id AND i.status IN ('pending','orphaned')) AS BOOLEAN) AS waiting_on_you,
 CAST(EXISTS(SELECT 1 FROM session_input_queue q WHERE q.session_id = member.id AND q.status IN ('queued','dispatching')) AS BOOLEAN) AS queued_input,
 CAST(member.pinned_at IS NOT NULL AS BOOLEAN) AS pinned,
 CAST(member.parked_at IS NOT NULL AS BOOLEAN) AS later
FROM thread JOIN sessions member ON member.id = thread.id
WHERE member.archived_at IS NULL
ORDER BY thread.root_id,thread.depth,member.id;
