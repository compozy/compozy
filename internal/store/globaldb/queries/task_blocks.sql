-- name: ClearTaskBlock :execrows
UPDATE task_blocks
SET cleared_at = sqlc.arg(cleared_at),
    cleared_by_kind = sqlc.arg(cleared_by_kind),
    cleared_by_ref = sqlc.arg(cleared_by_ref),
    clear_note = sqlc.narg(clear_note)
WHERE id = sqlc.arg(id)
  AND task_id = sqlc.arg(task_id)
  AND ((sqlc.narg(workspace_id) IS NULL AND workspace_id IS NULL) OR workspace_id = sqlc.narg(workspace_id))
  AND cleared_at IS NULL;

-- name: ListAllTaskBlocks :many
SELECT id, workspace_id, task_id, kind, reason, details_json,
       created_by_kind, created_by_ref, created_at, expires_at, cleared_at,
       cleared_by_kind, cleared_by_ref, clear_note
FROM task_blocks
WHERE task_id = sqlc.arg(task_id)
  AND ((sqlc.narg(workspace_id) IS NULL AND workspace_id IS NULL) OR workspace_id = sqlc.narg(workspace_id))
ORDER BY created_at ASC, id ASC;

-- name: ListOpenTaskBlocks :many
SELECT id, workspace_id, task_id, kind, reason, details_json,
       created_by_kind, created_by_ref, created_at, expires_at, cleared_at,
       cleared_by_kind, cleared_by_ref, clear_note
FROM task_blocks
WHERE task_id = sqlc.arg(task_id)
  AND ((sqlc.narg(workspace_id) IS NULL AND workspace_id IS NULL) OR workspace_id = sqlc.narg(workspace_id))
  AND cleared_at IS NULL
  AND (expires_at IS NULL OR expires_at > sqlc.arg(now))
ORDER BY created_at ASC, id ASC;

-- name: HasOpenTaskBlocks :one
SELECT EXISTS(
  SELECT 1
  FROM task_blocks
  WHERE task_id = sqlc.arg(task_id)
    AND ((sqlc.narg(workspace_id) IS NULL AND workspace_id IS NULL) OR workspace_id = sqlc.narg(workspace_id))
    AND cleared_at IS NULL
    AND (expires_at IS NULL OR expires_at > sqlc.arg(now))
);

-- name: SetTaskWakeCreator :execrows
UPDATE tasks
SET wake_creator = sqlc.arg(wake_creator), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: InsertTaskBlock :exec
INSERT INTO task_blocks (
  id, workspace_id, task_id, kind, reason, details_json, created_by_kind, created_by_ref,
  created_at, expires_at, cleared_at, cleared_by_kind, cleared_by_ref, clear_note
) VALUES (
  sqlc.arg(id), sqlc.narg(workspace_id), sqlc.arg(task_id), sqlc.arg(kind), sqlc.arg(reason),
  sqlc.narg(details_json), sqlc.arg(created_by_kind), sqlc.arg(created_by_ref),
  sqlc.arg(created_at), sqlc.narg(expires_at), NULL, NULL, NULL, NULL
);

-- name: HasClearedTaskBlockKind :one
SELECT EXISTS(
  SELECT 1
  FROM task_blocks
  WHERE task_id = sqlc.arg(task_id)
    AND kind = sqlc.arg(kind)
    AND ((sqlc.narg(workspace_id) IS NULL AND workspace_id IS NULL) OR workspace_id = sqlc.narg(workspace_id))
    AND cleared_at IS NOT NULL
);

-- name: ListExpiredTaskBlockCandidates :many
SELECT task_id, id
FROM task_blocks
WHERE cleared_at IS NULL
  AND expires_at IS NOT NULL
  AND expires_at <= sqlc.arg(now)
ORDER BY task_id ASC, created_at ASC, id ASC;

-- name: ListExpiredTaskBlockIDs :many
SELECT id
FROM task_blocks
WHERE task_id = sqlc.arg(task_id)
  AND cleared_at IS NULL
  AND expires_at IS NOT NULL
  AND expires_at <= sqlc.arg(now)
  AND ((sqlc.narg(workspace_id) IS NULL AND workspace_id IS NULL) OR workspace_id = sqlc.narg(workspace_id))
ORDER BY created_at ASC, id ASC;

-- name: ExpireTaskBlock :execrows
UPDATE task_blocks
SET cleared_at = sqlc.arg(cleared_at),
    cleared_by_kind = sqlc.arg(cleared_by_kind),
    cleared_by_ref = sqlc.arg(cleared_by_ref),
    clear_note = NULL
WHERE id = sqlc.arg(id)
  AND task_id = sqlc.arg(task_id)
  AND cleared_at IS NULL
  AND expires_at IS NOT NULL
  AND expires_at <= sqlc.arg(now)
  AND ((sqlc.narg(workspace_id) IS NULL AND workspace_id IS NULL) OR workspace_id = sqlc.narg(workspace_id));

-- name: GetTaskBlock :one
SELECT id, workspace_id, task_id, kind, reason, details_json,
       created_by_kind, created_by_ref, created_at, expires_at, cleared_at,
       cleared_by_kind, cleared_by_ref, clear_note
FROM task_blocks
WHERE id = sqlc.arg(id)
  AND task_id = sqlc.arg(task_id)
  AND ((sqlc.narg(workspace_id) IS NULL AND workspace_id IS NULL) OR workspace_id = sqlc.narg(workspace_id));

-- name: UpsertTaskBlockRecurrence :exec
INSERT INTO task_block_recurrences (task_id, kind, count, updated_at)
VALUES (sqlc.arg(task_id), sqlc.arg(kind), sqlc.arg(count), sqlc.arg(updated_at))
ON CONFLICT(task_id, kind) DO UPDATE SET
  count = excluded.count,
  updated_at = excluded.updated_at;

-- name: IncrementTaskBlockRecurrence :exec
INSERT INTO task_block_recurrences (task_id, kind, count, updated_at)
VALUES (sqlc.arg(task_id), sqlc.arg(kind), 1, sqlc.arg(updated_at))
ON CONFLICT(task_id, kind) DO UPDATE SET
  count = count + 1,
  updated_at = excluded.updated_at;

-- name: DeleteTaskBlockRecurrences :exec
DELETE FROM task_block_recurrences WHERE task_id = sqlc.arg(task_id);

-- name: GetTaskBlockRecurrence :one
SELECT task_id, kind, count, updated_at
FROM task_block_recurrences
WHERE task_id = sqlc.arg(task_id) AND kind = sqlc.arg(kind);

-- name: MarkTaskNeedsAttentionIfClear :execrows
UPDATE tasks
SET needs_attention_reason = sqlc.arg(reason),
    needs_attention_at = sqlc.arg(marked_at),
    needs_attention_by_kind = sqlc.arg(actor_kind),
    needs_attention_by_ref = sqlc.arg(actor_ref),
    updated_at = sqlc.arg(marked_at)
WHERE id = sqlc.arg(id) AND needs_attention_at IS NULL;
