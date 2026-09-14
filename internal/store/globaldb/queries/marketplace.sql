-- name: DeleteMarketplaceCatalogEntriesBySource :exec
DELETE FROM marketplace_catalog_entries WHERE source = sqlc.arg(source);

-- name: InsertMarketplaceCatalogEntry :exec
INSERT INTO marketplace_catalog_entries (
  source, entry_id, name, description, version, published_at, updated_at,
  digest_sha256, tier, install_slug, payload_json, fetched_at, layout, icon, installable, install_blocker, resolved_ref
) VALUES (
  sqlc.arg(source), sqlc.arg(entry_id), sqlc.arg(name), sqlc.arg(description),
  sqlc.arg(version), sqlc.narg(published_at), sqlc.narg(updated_at),
  sqlc.narg(digest_sha256), sqlc.narg(tier), sqlc.narg(install_slug),
  sqlc.arg(payload_json), sqlc.arg(fetched_at), sqlc.arg(layout), sqlc.arg(icon), sqlc.arg(installable), sqlc.arg(install_blocker), sqlc.arg(resolved_ref)
);

-- name: UpsertMarketplaceCatalogStateFresh :exec
INSERT INTO marketplace_catalog_state (
  source, manifest_version, generated_at, fetched_at, stale, last_error, revision, generation, plugins, installable,
  source_ref, document_digest, diagnostics_json, kind_of_source, document_path, owner, error_class
) VALUES (
  sqlc.arg(source), sqlc.arg(manifest_version), sqlc.narg(generated_at),
  sqlc.arg(fetched_at), 0, '', sqlc.arg(revision), sqlc.arg(generation), sqlc.arg(plugins), sqlc.arg(installable),
  sqlc.arg(source_ref), sqlc.arg(document_digest), sqlc.arg(diagnostics_json), sqlc.arg(kind_of_source),
  sqlc.arg(document_path), sqlc.arg(owner), ''
)
ON CONFLICT(source) DO UPDATE SET
  manifest_version = excluded.manifest_version,
  generated_at = excluded.generated_at,
  fetched_at = excluded.fetched_at,
  stale = 0,
  last_error = '',
  revision = excluded.revision,
  plugins = excluded.plugins,
  installable = excluded.installable,
  source_ref = excluded.source_ref,
  document_digest = excluded.document_digest,
  diagnostics_json = excluded.diagnostics_json,
  kind_of_source = excluded.kind_of_source,
  document_path = excluded.document_path,
  owner = excluded.owner,
  error_class = '';

-- name: MarkMarketplaceCatalogStateStale :execrows
UPDATE marketplace_catalog_state
SET stale = 1, last_error = sqlc.arg(last_error), error_class = sqlc.arg(error_class)
WHERE source = sqlc.arg(source) AND enabled = 1 AND generation = sqlc.arg(generation);

-- name: ListMarketplaceCatalogEntries :many
SELECT *
FROM marketplace_catalog_entries
WHERE source = sqlc.arg(source)
ORDER BY entry_id ASC
LIMIT sqlc.arg(result_limit);

-- name: GetMarketplaceCatalogEntry :one
SELECT *
FROM marketplace_catalog_entries
WHERE source = sqlc.arg(source) AND entry_id = sqlc.arg(entry_id);

-- name: GetMarketplaceExtensionByInstallSlug :one
SELECT *
FROM marketplace_catalog_entries
WHERE source = 'compozy-catalog'
  AND install_slug = sqlc.arg(install_slug)
  AND (CAST(sqlc.arg(version) AS TEXT) = '' OR version = sqlc.arg(version));

-- name: GetMarketplaceCatalogState :one
SELECT s.source, s.generation, s.revision, s.manifest_version, s.generated_at, s.fetched_at, s.stale, s.last_error,
       s.source_ref, s.config_revision, s.document_digest, s.diagnostics_json, s.kind_of_source, s.enabled,
       s.installable, s.error_class, s.document_path, s.owner,
       CAST((SELECT COUNT(*) FROM marketplace_catalog_entries e WHERE e.source = s.source) AS INTEGER) AS entry_count
FROM marketplace_catalog_state s
WHERE s.source = sqlc.arg(source);

-- name: ClaimMarketplaceCatalogGeneration :execrows
UPDATE marketplace_catalog_state SET generation = generation
WHERE source = sqlc.arg(source) AND source_ref = sqlc.arg(source_ref)
  AND enabled = 1 AND generation = sqlc.arg(generation);

-- name: ClaimMarketplaceSourceConfiguration :one
INSERT INTO marketplace_catalog_config (id, generation, revision)
VALUES (1, (SELECT COALESCE(MAX(generation), 0) FROM marketplace_catalog_state), '')
ON CONFLICT(id) DO UPDATE SET id = id
RETURNING generation, revision;

-- name: SetMarketplaceSourceConfiguration :one
UPDATE marketplace_catalog_config
SET generation = MAX(generation, (SELECT COALESCE(MAX(generation), 0) FROM marketplace_catalog_state)) + 1,
    revision = sqlc.arg(revision)
WHERE id = 1 RETURNING generation, revision;

-- name: GetMarketplaceSourceConfiguration :one
SELECT generation, revision FROM marketplace_catalog_config WHERE id = 1;

-- name: RegisterMarketplaceSource :exec
INSERT INTO marketplace_catalog_state (source, source_ref, config_revision, kind_of_source, enabled, generation, manifest_version)
VALUES (sqlc.arg(source), sqlc.arg(source_ref), sqlc.arg(config_revision), sqlc.arg(kind_of_source), sqlc.arg(enabled), sqlc.arg(generation), 0)
ON CONFLICT(source) DO UPDATE SET
  generation = CASE WHEN source_ref <> excluded.source_ref OR config_revision <> excluded.config_revision
    OR kind_of_source <> excluded.kind_of_source OR enabled <> excluded.enabled THEN excluded.generation ELSE generation END,
  stale = CASE WHEN source_ref <> excluded.source_ref OR config_revision <> excluded.config_revision THEN 1 ELSE stale END,
  source_ref = excluded.source_ref, config_revision = excluded.config_revision,
  kind_of_source = excluded.kind_of_source, enabled = excluded.enabled;

-- name: DeleteMarketplaceCatalogState :exec
DELETE FROM marketplace_catalog_state WHERE source = sqlc.arg(source);

-- name: DeleteUnconfiguredMarketplaceEntries :exec
DELETE FROM marketplace_catalog_entries WHERE source NOT IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(names)));

-- name: DeleteUnconfiguredMarketplaceStates :exec
DELETE FROM marketplace_catalog_state WHERE source NOT IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(names)));

-- name: ListMarketplaceSourceNameRetainers :many
SELECT name FROM extensions
WHERE json_extract(provenance_json, '$.source_name') = sqlc.arg(source)
  AND COALESCE(json_extract(provenance_json, '$.source_ref'), '') <> ''
  AND json_extract(provenance_json, '$.source_ref') <> sqlc.arg(source_ref)
ORDER BY name;
