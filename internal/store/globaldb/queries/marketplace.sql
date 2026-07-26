-- name: DeleteMarketplaceCatalogEntriesByKind :exec
DELETE FROM marketplace_catalog_entries WHERE kind = sqlc.arg(kind);

-- name: InsertMarketplaceCatalogEntry :exec
INSERT INTO marketplace_catalog_entries (
  kind, entry_id, name, description, version, published_at, updated_at,
  digest_sha256, tier, install_slug, payload_json, fetched_at
) VALUES (
  sqlc.arg(kind), sqlc.arg(entry_id), sqlc.arg(name), sqlc.arg(description),
  sqlc.arg(version), sqlc.narg(published_at), sqlc.narg(updated_at),
  sqlc.narg(digest_sha256), sqlc.narg(tier), sqlc.narg(install_slug),
  sqlc.arg(payload_json), sqlc.arg(fetched_at)
);

-- name: UpsertMarketplaceCatalogStateFresh :exec
INSERT INTO marketplace_catalog_state (
  kind, manifest_version, generated_at, fetched_at, stale, last_error
) VALUES (
  sqlc.arg(kind), sqlc.arg(manifest_version), sqlc.narg(generated_at),
  sqlc.arg(fetched_at), 0, ''
)
ON CONFLICT(kind) DO UPDATE SET
  manifest_version = excluded.manifest_version,
  generated_at = excluded.generated_at,
  fetched_at = excluded.fetched_at,
  stale = 0,
  last_error = '';

-- name: MarkMarketplaceCatalogStateStale :exec
INSERT INTO marketplace_catalog_state (
  kind, manifest_version, generated_at, fetched_at, stale, last_error
) VALUES (
  sqlc.arg(kind), 0, NULL, '', 1, sqlc.arg(last_error)
)
ON CONFLICT(kind) DO UPDATE SET stale = 1, last_error = excluded.last_error;

-- name: ListMarketplaceCatalogEntries :many
SELECT kind, entry_id, name, description, version, published_at, updated_at,
       digest_sha256, tier, install_slug, payload_json, fetched_at
FROM marketplace_catalog_entries
WHERE kind = sqlc.arg(kind)
ORDER BY entry_id ASC
LIMIT sqlc.arg(result_limit);

-- name: GetMarketplaceCatalogEntry :one
SELECT kind, entry_id, name, description, version, published_at, updated_at,
       digest_sha256, tier, install_slug, payload_json, fetched_at
FROM marketplace_catalog_entries
WHERE kind = sqlc.arg(kind) AND entry_id = sqlc.arg(entry_id);

-- name: GetMarketplaceExtensionByInstallSlug :one
SELECT kind, entry_id, name, description, version, published_at, updated_at,
       digest_sha256, tier, install_slug, payload_json, fetched_at
FROM marketplace_catalog_entries
WHERE kind = 'extension'
  AND install_slug = sqlc.arg(install_slug)
  AND (CAST(sqlc.arg(version) AS TEXT) = '' OR version = sqlc.arg(version));

-- name: ListMarketplaceSkillsByInstallSlugs :many
SELECT kind, entry_id, name, description, version, published_at, updated_at,
       digest_sha256, tier, install_slug, payload_json, fetched_at
FROM marketplace_catalog_entries
WHERE kind = 'skill'
  AND install_slug IN (sqlc.slice(install_slugs))
ORDER BY install_slug ASC, entry_id ASC;

-- name: GetMarketplaceCatalogState :one
SELECT s.kind, s.manifest_version, s.generated_at, s.fetched_at, s.stale, s.last_error,
       CAST((SELECT COUNT(*) FROM marketplace_catalog_entries e WHERE e.kind = s.kind) AS INTEGER) AS entry_count
FROM marketplace_catalog_state s
WHERE s.kind = sqlc.arg(kind);
