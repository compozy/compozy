-- name: InsertWorktree :exec
INSERT INTO worktrees (
  profile_id,
  id, workspace_id, name, branch, path, git_dir, state, pending_phase, origin,
  setup_state, setup_error, base_ref, created_branch, run_namespace, created_head,
  run_id, created_at, updated_at
) VALUES (
  sqlc.arg(profile_id),
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(name), sqlc.arg(branch), sqlc.arg(path),
  sqlc.arg(git_dir), sqlc.arg(state), sqlc.arg(pending_phase), sqlc.arg(origin),
  sqlc.arg(setup_state), sqlc.arg(setup_error), sqlc.arg(base_ref), sqlc.arg(created_branch),
  sqlc.arg(run_namespace), sqlc.arg(created_head), sqlc.arg(run_id), sqlc.arg(created_at),
  sqlc.arg(updated_at)
);

-- name: GetWorktreeByName :one
SELECT * FROM worktrees
WHERE workspace_id = sqlc.arg(workspace_id)
  AND name = sqlc.arg(name)
  AND state <> 'dismissed'
LIMIT 1;

-- name: GetWorktreeByID :one
SELECT * FROM worktrees
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(worktree_id)
LIMIT 1;

-- name: GetLiveWorktreeByPath :one
SELECT * FROM worktrees
WHERE workspace_id = sqlc.arg(workspace_id)
  AND path = sqlc.arg(path)
  AND state NOT IN ('removed', 'dismissed')
LIMIT 1;

-- name: ListWorktrees :many
SELECT * FROM worktrees
WHERE workspace_id = sqlc.arg(workspace_id)
  AND state <> 'dismissed'
ORDER BY created_at, id;

-- name: ListRecoverableWorktrees :many
SELECT * FROM worktrees
WHERE state IN ('pending', 'removing', 'missing')
ORDER BY workspace_id, created_at, id;

-- name: DeletePendingWorktree :execrows
DELETE FROM worktrees
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(worktree_id)
  AND state = 'pending';

-- name: DeleteRunMaterialization :execrows
DELETE FROM worktrees
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(worktree_id)
  AND run_id = sqlc.arg(run_id)
  AND origin = 'per_run'
  AND state IN ('pending', 'ready');

-- name: UpdatePendingWorktree :execrows
UPDATE worktrees SET
  pending_phase = sqlc.arg(pending_phase),
  branch = sqlc.arg(branch),
  git_dir = sqlc.arg(git_dir),
  base_ref = sqlc.arg(base_ref),
  created_branch = sqlc.arg(created_branch),
  run_namespace = sqlc.arg(run_namespace),
  created_head = sqlc.arg(created_head),
  updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(worktree_id)
  AND state = 'pending';

-- name: CompleteWorktreeCreate :execrows
UPDATE worktrees SET
  state = 'ready',
  pending_phase = '',
  setup_state = sqlc.arg(setup_state),
  setup_error = sqlc.arg(setup_error),
  updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(worktree_id)
  AND state = 'pending';

-- name: CompareAndSwapWorktreeState :execrows
UPDATE worktrees SET state = sqlc.arg(next_state), updated_at = sqlc.arg(updated_at)
WHERE worktrees.workspace_id = sqlc.arg(workspace_id)
  AND worktrees.id = sqlc.arg(worktree_id)
  AND worktrees.state = sqlc.arg(expected_state)
  AND (
    sqlc.arg(next_state) <> 'removing'
    OR NOT EXISTS (
      SELECT 1 FROM worktree_exit_ops
	  WHERE worktree_exit_ops.workspace_id = sqlc.arg(workspace_id)
	    AND worktree_exit_ops.worktree_id = sqlc.arg(worktree_id)
		AND worktree_exit_ops.state = 'running'
    )
  );

-- name: SetWorktreeState :execrows
UPDATE worktrees SET state = sqlc.arg(state), updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(worktree_id);

-- name: UpsertWorktreeStatus :execrows
INSERT INTO worktree_status (
  worktree_id, branch, detached, head_sha, dirty_files, insertions, deletions,
  has_upstream, ahead, behind, read_error, refreshed_at
)
SELECT id, sqlc.narg(branch), sqlc.narg(detached), sqlc.narg(head_sha),
  sqlc.narg(dirty_files), sqlc.narg(insertions), sqlc.narg(deletions),
  sqlc.narg(has_upstream), sqlc.narg(ahead), sqlc.narg(behind),
  sqlc.arg(read_error), sqlc.narg(refreshed_at)
FROM worktrees
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(worktree_id)
ON CONFLICT(worktree_id) DO UPDATE SET
  branch = excluded.branch,
  detached = excluded.detached,
  head_sha = excluded.head_sha,
  dirty_files = excluded.dirty_files,
  insertions = excluded.insertions,
  deletions = excluded.deletions,
  has_upstream = excluded.has_upstream,
  ahead = excluded.ahead,
  behind = excluded.behind,
  read_error = excluded.read_error,
  refreshed_at = excluded.refreshed_at;

-- name: GetWorktreeStatus :one
SELECT ws.* FROM worktree_status ws
JOIN worktrees w ON w.id = ws.worktree_id
WHERE w.workspace_id = sqlc.arg(workspace_id) AND w.id = sqlc.arg(worktree_id);

-- name: UpsertWorktreeForgeStatus :execrows
INSERT INTO worktree_forge_status (
  worktree_id, provider, pr_number, pr_state, pr_url, merged, fetched_at
)
SELECT id, sqlc.arg(provider), sqlc.narg(pr_number), sqlc.narg(pr_state),
  sqlc.arg(pr_url), sqlc.narg(merged), sqlc.narg(fetched_at)
FROM worktrees
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(worktree_id)
ON CONFLICT(worktree_id) DO UPDATE SET
  provider = excluded.provider,
  pr_number = excluded.pr_number,
  pr_state = excluded.pr_state,
  pr_url = excluded.pr_url,
  merged = excluded.merged,
  fetched_at = excluded.fetched_at;

-- name: GetWorktreeForgeStatus :one
SELECT fs.* FROM worktree_forge_status fs
JOIN worktrees w ON w.id = fs.worktree_id
WHERE w.workspace_id = sqlc.arg(workspace_id) AND w.id = sqlc.arg(worktree_id);

-- name: InsertWorktreeExitOperation :exec
INSERT INTO worktree_exit_ops (
  op_id, workspace_id, worktree_id, action, state, started_at, finished_at
) VALUES (
  sqlc.arg(op_id), sqlc.arg(workspace_id), sqlc.arg(worktree_id), sqlc.arg(action),
  sqlc.arg(state), sqlc.arg(started_at), sqlc.narg(finished_at)
);

-- name: FinishWorktreeExitOperation :execrows
UPDATE worktree_exit_ops SET state = sqlc.arg(state), finished_at = sqlc.arg(finished_at)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND worktree_id = sqlc.arg(worktree_id)
  AND op_id = sqlc.arg(op_id)
  AND state = 'running';

-- name: ListRunningWorktreeExitOperations :many
SELECT * FROM worktree_exit_ops
WHERE state = 'running'
ORDER BY started_at, op_id;

-- name: FailRunningWorktreeExitOperations :exec
UPDATE worktree_exit_ops SET state = 'failed', finished_at = sqlc.arg(finished_at)
WHERE state = 'running';
