# Migration Decision Matrix

Use the matrix below to decide what kind of artifact your change needs.

| Change | Migration required? | Notes |
|--------|---------------------|-------|
| Add a NOT NULL column to existing table | YES | Provide a default expressible in SQL or backfill in the same transaction. |
| Add a NULLable column | YES | Even nullable adds need to be migrated, not implicit. |
| Drop a column | YES | SQLite supports `ALTER TABLE DROP COLUMN` since 3.35; check the project's minimum version before using. |
| Rename a column | YES | Use `CREATE NEW + INSERT INTO ... SELECT + DROP + RENAME` if direct rename isn't supported. |
| Add an index | YES | Add it beside its table in the owning declarative source; let Atlas plan the appended SQL. |
| Drop an index | YES | Treat Atlas destructive diagnostics as a design review input. |
| Add a CHECK constraint | YES | Same as table-rebuild for older SQLite versions. |
| Add a unique constraint | YES | Risk of breaking existing data — surface in the spec. |
| New table | YES | Add the complete table and indexes to the owning declarative source. |
| Drop a table | YES | Migrate live rows into their replacement first; dropping user data needs ADR-recorded sign-off (SD-013). Mention the delete target in the spec. |
| Add a row (seed data) | YES | Add bounded data SQL to the unpublished migration tail and test repeated open. |
| Change default value | YES | Existing rows are unaffected by SQLite default changes — explicit backfill if needed. |
| Touch struct field that round-trips through SQLite | YES (column add/rename) | The Go struct change is just the front of the migration. |
| In-memory cache shape change | NO | This skill does not apply. |
| `internal/memory/MEMORY.md` schema | NO | Markdown is the source of truth; FTS5 catalog is derived (see `docs/_memory/analysis/analysis_codex_plans.md`). Reindex via `internal/memory/consolidation`. |

## Compatibility rule (SD-013)

Stored rows are user state: every shape change carries the transformation that keeps them readable, so the new binary opens the old database without loss. Data loss is a spec decision with the user's recorded sign-off, never a migration default.

The compatibility lives in the appended Goose SQL, not in Go: one migration transforms the data once, and the runtime reads only the new shape. Do not add boot-time schema repair, dual-shape runtime branches, or `if oldShape` code paths. Internal renames of Go types, queries, and packages still sweep every consumer in the same change.
