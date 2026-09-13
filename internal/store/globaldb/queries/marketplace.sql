-- name: DeleteMarketplaceCatalogEntriesBySource :exec
DELETE FROM marketplace_catalog_entries WHERE source = sqlc.arg(source);

-- name: InsertMarketplaceCatalogEntry :exec
INSERT INTO marketplace_catalog_entries (
  source, kind, entry_id, name, description, version, published_at, updated_at,
  digest_sha256, tier, install_slug, payload_json, fetched_at, layout, icon, installable, install_blocker, resolved_ref
) VALUES (
  sqlc.arg(source), sqlc.arg(kind), sqlc.arg(entry_id), sqlc.arg(name), sqlc.arg(description),
  sqlc.arg(version), sqlc.narg(published_at), sqlc.narg(updated_at),
  sqlc.narg(digest_sha256), sqlc.narg(tier), sqlc.narg(install_slug),
  sqlc.arg(payload_json), sqlc.arg(fetched_at), sqlc.arg(layout), sqlc.arg(icon), sqlc.arg(installable), sqlc.arg(install_blocker), sqlc.arg(resolved_ref)
);

-- name: UpsertMarketplaceCatalogStateFresh :exec
INSERT INTO marketplace_catalog_state (
  source, manifest_version, generated_at, fetched_at, stale, last_error, revision, generation, plugins, installable
) VALUES (
  sqlc.arg(source), sqlc.arg(manifest_version), sqlc.narg(generated_at),
  sqlc.arg(fetched_at), 0, '', sqlc.arg(revision), sqlc.arg(generation), sqlc.arg(plugins), sqlc.arg(installable)
)
ON CONFLICT(source) DO UPDATE SET
  manifest_version = excluded.manifest_version,
  generated_at = excluded.generated_at,
  fetched_at = excluded.fetched_at,
  stale = 0,
  last_error = '',
  revision = excluded.revision,
  plugins = excluded.plugins,
  installable = excluded.installable;

-- name: MarkMarketplaceCatalogStateStale :execrows
INSERT INTO marketplace_catalog_state (
  source, manifest_version, generated_at, fetched_at, stale, last_error, generation
) VALUES (
  sqlc.arg(source), 0, NULL, '', 1, sqlc.arg(last_error), sqlc.arg(generation)
)
ON CONFLICT(source) DO UPDATE SET stale = 1, last_error = excluded.last_error
WHERE marketplace_catalog_state.generation = excluded.generation;

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
WHERE source = 'compozy-catalog' AND kind = 'extension'
  AND install_slug = sqlc.arg(install_slug)
  AND (CAST(sqlc.arg(version) AS TEXT) = '' OR version = sqlc.arg(version));

-- name: ListMarketplaceSkillsByInstallSlugs :many
SELECT *
FROM marketplace_catalog_entries
WHERE kind = 'skill'
  AND install_slug IN (sqlc.slice(install_slugs))
ORDER BY install_slug ASC, entry_id ASC;

-- name: GetMarketplaceCatalogState :one
SELECT s.source, s.generation, s.revision, s.manifest_version, s.generated_at, s.fetched_at, s.stale, s.last_error,
       CAST((SELECT COUNT(*) FROM marketplace_catalog_entries e WHERE e.source = s.source) AS INTEGER) AS entry_count
FROM marketplace_catalog_state s
WHERE s.source = sqlc.arg(source);

-- name: ClaimMarketplaceCatalogGeneration :execrows
INSERT INTO marketplace_catalog_state (source, manifest_version, generation)
VALUES (sqlc.arg(source), 0, sqlc.arg(generation))
ON CONFLICT(source) DO UPDATE SET generation = excluded.generation
WHERE marketplace_catalog_state.generation = excluded.generation;

-- name: AdvanceMarketplaceCatalogGeneration :one
INSERT INTO marketplace_catalog_state (source, manifest_version, generation, stale)
VALUES (sqlc.arg(source), 0, 1, 1)
ON CONFLICT(source) DO UPDATE SET generation = marketplace_catalog_state.generation + 1, stale = 1
RETURNING generation;
