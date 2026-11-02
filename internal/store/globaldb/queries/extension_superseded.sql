-- name: GetBundledMigrationSource :one
SELECT source FROM extensions WHERE name = sqlc.arg(name);

-- name: HasIncompatibleBundledMigrationState :one
SELECT EXISTS(SELECT 1 FROM extension_inputs WHERE extension_inputs.extension = sqlc.arg(name)) OR
 EXISTS(SELECT 1 FROM extension_env_bindings WHERE extension_env_bindings.extension_name = sqlc.arg(name)) OR
 EXISTS(SELECT 1 FROM extension_mcp_overrides WHERE extension_mcp_overrides.extension = sqlc.arg(name)) OR
 EXISTS(SELECT 1 FROM extension_profile_markers WHERE extension_profile_markers.extension_name = sqlc.arg(name)) OR
 EXISTS(SELECT 1 FROM extension_dev_links WHERE extension_dev_links.extension_name = sqlc.arg(name)) OR
 NOT EXISTS(SELECT 1 FROM extension_installations WHERE extension_installations.extension_name = sqlc.arg(name));

-- name: HasBundledMigrationEnablementConflict :one
SELECT EXISTS(SELECT 1 FROM profiles p WHERE
 COALESCE((SELECT enabled FROM extension_profile_enablement
 WHERE extension_profile_enablement.extension_name = sqlc.arg(predecessor) AND extension_profile_enablement.profile_id = p.id), 1)
 <> COALESCE((SELECT enabled FROM extension_profile_enablement
 WHERE extension_profile_enablement.extension_name = sqlc.arg(successor) AND extension_profile_enablement.profile_id = p.id), 1));

-- name: DeleteBundledMigrationInstallations :exec
DELETE FROM extension_installations WHERE extension_installations.extension_name = sqlc.arg(name);

-- name: DeleteBundledMigrationEnablement :exec
DELETE FROM extension_profile_enablement WHERE extension_name = sqlc.arg(name);

-- name: CopyBundledMigrationInstallations :exec
INSERT INTO extension_installations (extension_name, profile_id, workspace_id, created_at)
 SELECT sqlc.arg(successor), profile_id, workspace_id, created_at FROM extension_installations
 WHERE extension_installations.extension_name = sqlc.arg(predecessor)
 ON CONFLICT(extension_name, profile_id, workspace_id) DO NOTHING;

-- name: CopyBundledMigrationEnablement :exec
INSERT INTO extension_profile_enablement (extension_name, profile_id, enabled)
 SELECT sqlc.arg(successor), profile_id, enabled FROM extension_profile_enablement
 WHERE extension_profile_enablement.extension_name = sqlc.arg(predecessor)
 ON CONFLICT(extension_name, profile_id) DO NOTHING;

-- name: DeleteSupersededBundledExtension :exec
DELETE FROM extensions WHERE name = sqlc.arg(name) AND source = 'bundled';
