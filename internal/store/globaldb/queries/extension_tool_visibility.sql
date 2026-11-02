-- name: ListExtensionToolVisibility :many
WITH names AS (
  SELECT name FROM extensions
  UNION
  SELECT extension_name AS name FROM extension_dev_links
  WHERE workspace_id = sqlc.arg(workspace_id)
)
SELECT names.name,
  CAST(COALESCE(enablement.enabled, 1) AS BOOLEAN) AS enabled,
  CAST(installation.extension_name IS NOT NULL AS BOOLEAN) AS installed,
  COALESCE(installation.workspace_id, '') AS installation_workspace_id,
  CAST(dev.extension_name IS NOT NULL AS BOOLEAN) AS dev_linked,
  COALESCE(dev.bundle_generation, '') AS dev_generation
FROM names
LEFT JOIN extension_profile_enablement AS enablement
  ON enablement.extension_name = names.name AND enablement.profile_id = sqlc.arg(profile_id)
LEFT JOIN extension_installations AS installation ON installation.rowid = (
  SELECT candidate.rowid FROM extension_installations AS candidate
  WHERE candidate.extension_name = names.name
    AND candidate.profile_id IN ('', sqlc.arg(profile_id))
    AND candidate.workspace_id IN ('', sqlc.arg(workspace_id))
  ORDER BY (candidate.workspace_id <> '') DESC, (candidate.profile_id <> '') DESC
  LIMIT 1
)
LEFT JOIN extension_dev_links AS dev
  ON dev.extension_name = names.name AND dev.workspace_id = sqlc.arg(workspace_id)
ORDER BY names.name;

-- name: ListExtensionWorkspaceInstallations :many
SELECT DISTINCT extension_name FROM extension_installations
WHERE workspace_id IN ('', sqlc.arg(workspace_id))
ORDER BY extension_name;
