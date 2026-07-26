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
| Add a unique constraint | YES | Risk of breaking existing data — surface in techspec. |
| New table | YES | Add the complete table and indexes to the owning declarative source. |
| Drop a table | YES | Greenfield-alpha: hard cut. Mention the delete target in the techspec. |
| Add a row (seed data) | YES | Add bounded data SQL to the unpublished migration tail and test repeated open. |
| Change default value | YES | Existing rows are unaffected by SQLite default changes — explicit backfill if needed. |
| Touch struct field that round-trips through SQLite | YES (column add/rename) | The Go struct change is just the front of the migration. |
| In-memory cache shape change | NO | This skill does not apply. |
| `internal/memory/MEMORY.md` schema | NO | Markdown is the source of truth; FTS5 catalog is derived (see `docs/_memory/analysis/analysis_codex_plans.md`). Reindex via `internal/memory/consolidation`. |

## Greenfield rule

If the migration would require a "preserve old behavior" branch, the answer is "delete the old thing." Hard-cut renames sweep code, storage, APIs, CLI, extensions, specs, RFCs, AND `.compozy/tasks/*` artifacts in the same change.

Any one-pass data transformation belongs in one appended Goose SQL migration and an ADR-backed contract. Do not add boot-time schema repair or dual-shape runtime branches.
