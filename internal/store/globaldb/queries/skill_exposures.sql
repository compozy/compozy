-- name: CreateSkillExposure :one
INSERT INTO skill_exposures (
  skill_name, canonical_dir, target_slug, link_path, link_target,
  owner_scope, workspace_id, created_at, updated_at
) VALUES (
  sqlc.arg(skill_name), sqlc.arg(canonical_dir), sqlc.arg(target_slug),
  sqlc.arg(link_path), sqlc.arg(link_target), sqlc.arg(owner_scope),
  sqlc.narg(workspace_id), sqlc.arg(created_at), sqlc.arg(updated_at)
)
RETURNING id, skill_name, canonical_dir, target_slug, link_path, link_target,
          owner_scope, workspace_id, created_at, updated_at;

-- name: GetSkillExposureByOwnerTarget :one
SELECT id, skill_name, canonical_dir, target_slug, link_path, link_target,
       owner_scope, workspace_id, created_at, updated_at
FROM skill_exposures
WHERE skill_name = sqlc.arg(skill_name)
  AND owner_scope = sqlc.arg(owner_scope)
  AND workspace_id IS sqlc.narg(workspace_id)
  AND target_slug = sqlc.arg(target_slug);

-- name: ListSkillExposuresByOwner :many
SELECT id, skill_name, canonical_dir, target_slug, link_path, link_target,
       owner_scope, workspace_id, created_at, updated_at
FROM skill_exposures
WHERE skill_name = sqlc.arg(skill_name)
  AND owner_scope = sqlc.arg(owner_scope)
  AND workspace_id IS sqlc.narg(workspace_id)
ORDER BY target_slug ASC, id ASC;

-- name: ListSkillExposuresByCanonicalDir :many
SELECT id, skill_name, canonical_dir, target_slug, link_path, link_target,
       owner_scope, workspace_id, created_at, updated_at
FROM skill_exposures
WHERE canonical_dir = sqlc.arg(canonical_dir)
ORDER BY target_slug ASC, id ASC;

-- name: DeleteSkillExposure :exec
DELETE FROM skill_exposures WHERE id = sqlc.arg(id);
