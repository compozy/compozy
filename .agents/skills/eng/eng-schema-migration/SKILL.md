---
name: eng-schema-migration
description: Authors append-only Goose SQL migrations for Compozy SQLite streams and keeps declarative schema sources, atlas.sum, Atlas checks, sqlc output, and migration tests synchronized. Use for SQLite table, column, index, constraint, trigger, or seed-data changes under internal/store or internal/memory. Do not use for in-memory structures, derived Markdown memory, or non-SQLite caches.
trigger: implicit
---

# Compozy Schema Migration

## Procedure

1. Read `references/migration-decision.md` and classify the changed datum and owning stream: `global` and `memory` share `compozy.db`; `session` owns each `events.db`; `workspace` owns workspace observability databases.
2. Inspect the owner's declarative schema source (`schema/schema.sql` or `schema/definitions/*.sql`), `schema/migrations/`, `schema/migrations/atlas.sum`, `migration_stream.go`, sqlc query catalog, and canonical migration/open tests. Select the next gap-free five-digit version. Never edit, rename, renumber, reorder, or delete an existing migration or its checksum entry.
3. Read `references/migration-template.md`. Edit the owning declarative source, then run `make codegen`. Inspect the newly appended Goose SQL, Atlas sqlcheck result, refreshed `atlas.sum`, and regenerated sqlc output. If the generated tail is wrong, correct the declarative schema and regenerate; add bounded data transformation SQL only to the unpublished tail, then rerun `make codegen`.
4. Update affected static queries in the owning sqlc catalog. Keep generated `sqlcgen` types inside the owner package and map them to domain types at the repository boundary.
5. Read `references/migration-test-patterns.md`. Extend the canonical suites for fresh apply, reopen with preserved data, ahead-version refusal, integrity failure, sequential history, and migrations-to-declarative-schema equivalence. When global or memory changes, also prove their shared-file table ownership remains disjoint.
6. If recovery or refusal guidance changes, move the whole stopped SQLite family (`.db`, `-wal`, `-shm`, and sibling databases) to cold storage; never move or edit one live file. Prefer a newer compatible binary for `schema_ahead` when state must be preserved.
7. Run scoped `-race` tests for the owning stream, `make codegen-check`, `make lint`, then `make gate` before push. Exact-head PR CI owns full completion verification.

## Error Handling

- Stop on any `atlas.sum` mismatch or edited historical byte. Restore the exact unpublished history or append a new migration; never weaken validation or edit a Goose version table.
- Resolve destructive Atlas diagnostics in the design. Do not suppress sqlcheck to force generation.
- User data survives every migration (SD-013): transform rows in the appended SQL instead of dropping them; a migration that drops or truncates user rows needs the user's sign-off recorded in an ADR plus a release-note `Migration notes` block. Document delete targets in the spec/ADR. Do not ship dual schemas or open-ended repair branches.
- Treat a pre-Goose marker as `legacy_database` and a recorded version above the embedded head as `schema_ahead`; neither condition authorizes in-place mutation.
