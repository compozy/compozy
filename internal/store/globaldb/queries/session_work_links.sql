-- name: SaveSessionWorkLink :execrows
INSERT INTO session_work_links(session_id, workspace_id, repo_key, branch, pr_number, worktree_id, worktree_name, provenance)
VALUES (sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(repo_key), sqlc.arg(branch), sqlc.narg(pr_number), sqlc.arg(worktree_id), sqlc.arg(worktree_name), sqlc.arg(provenance))
ON CONFLICT(session_id, repo_key, branch) DO UPDATE SET
 unlinked = 0,
 pr_number = COALESCE(excluded.pr_number, session_work_links.pr_number),
 worktree_id = CASE WHEN excluded.worktree_id <> '' THEN excluded.worktree_id ELSE session_work_links.worktree_id END,
 worktree_name = CASE WHEN excluded.worktree_name <> '' THEN excluded.worktree_name ELSE session_work_links.worktree_name END;

-- name: UnlinkSessionWork :execrows
INSERT INTO session_work_links(session_id, workspace_id, repo_key, branch, pr_number, worktree_id, worktree_name, provenance, unlinked)
VALUES (sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(repo_key), sqlc.arg(branch), sqlc.narg(pr_number), sqlc.arg(worktree_id), sqlc.arg(worktree_name), sqlc.arg(provenance), 1)
ON CONFLICT(session_id, repo_key, branch) DO UPDATE SET unlinked = 1;

-- name: ReadSessionWorkLinks :many
SELECT l.*, CAST(CASE WHEN l.worktree_id <> '' AND (w.id IS NULL OR w.state = 'removed') THEN 1 ELSE 0 END AS INTEGER) AS worktree_removed
FROM session_work_links l JOIN json_each(sqlc.arg(ids)) i ON l.session_id = i.value
LEFT JOIN worktrees w ON w.id = l.worktree_id AND w.workspace_id = l.workspace_id;

-- name: ReadSessionPendingWork :many
SELECT DISTINCT session_id FROM session_input_queue JOIN json_each(sqlc.arg(ids)) i ON session_id = i.value
WHERE status = 'dispatching' OR (status = 'queued' AND dispatchable = 1);

-- name: ReadSessionWorkFamily :many
WITH RECURSIVE family(root_id, id, workspace_id, profile_id) AS (
 SELECT s.id AS root_id, s.id AS id, s.workspace_id AS workspace_id, s.profile_id AS profile_id FROM sessions s JOIN json_each(sqlc.arg(ids)) i ON i.value = s.id
 UNION
 SELECT f.root_id, s.id, s.workspace_id, s.profile_id FROM sessions s JOIN family f ON s.parent_session_id = f.id
 WHERE s.workspace_id = f.workspace_id AND s.profile_id = f.profile_id AND COALESCE(s.lineage_kind, '') NOT IN ('fork', 'continue')
)
SELECT f.root_id, s.id, s.workspace_id, s.profile_id, s.name, COALESCE(s.worktree_id, '') AS worktree_id, COALESCE(w.name, '') AS worktree_name
FROM family f JOIN sessions s ON s.id = f.id
LEFT JOIN worktrees w ON w.id = s.worktree_id AND w.workspace_id = s.workspace_id;

-- name: SaveAutomaticSessionWorkLink :execrows
INSERT INTO session_work_links(session_id, workspace_id, repo_key, branch, pr_number, worktree_id, worktree_name, provenance)
VALUES (sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(repo_key), sqlc.arg(branch), sqlc.narg(pr_number), sqlc.arg(worktree_id), sqlc.arg(worktree_name), sqlc.arg(provenance))
ON CONFLICT(session_id, repo_key, branch) DO UPDATE SET
 pr_number = COALESCE(excluded.pr_number, session_work_links.pr_number),
 worktree_id = CASE WHEN excluded.worktree_id <> '' THEN excluded.worktree_id ELSE session_work_links.worktree_id END,
 worktree_name = CASE WHEN excluded.worktree_name <> '' THEN excluded.worktree_name ELSE session_work_links.worktree_name END;
