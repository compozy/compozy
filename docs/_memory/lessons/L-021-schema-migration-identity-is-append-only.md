# L-021 — Schema migration identity is append-only

**Class:** Persistence
**Date discovered:** 2026-05-06 (daemon restart migration integrity failure)
**Evidence sources:** Local daemon restart failure, observed `~/.agh/agh.db` `schema_migrations`
rows, `0b371eaa feat: add network threads (#105)`, `08eedb32 feat: orchestration
improvements (#106)`, and L-008 schema migration discipline

## Historical context (pre-Goose)

Restarting the local daemon failed before readiness with:

```text
store: migration 17 integrity mismatch: recorded "add_task_orchestration_profile_schema"/2026-05-05-add-task-orchestration-profile-schema, current "rebuild_network_conversation_containers"/2026-05-05-rebuild-network-conversation-containers
```

The live global database had already recorded:

```text
17 add_task_orchestration_profile_schema
18 add_task_review_gate_schema
19 add_notification_cursors
20 add_bridge_task_subscriptions
```

Current code had inserted `rebuild_network_conversation_containers` at version 17 and shifted the
existing task/bridge migrations to later numbers. The migration runner correctly refused to boot:
the persisted version/name/checksum identity no longer matched the binary.

## Root cause

Migration numbers were treated as a local ordering convenience instead of persisted contract data.
Fresh database tests still passed because the final schema could be built from the new order, but
an existing database carried the historical identity in `schema_migrations`. Once any developer,
QA, or release database can record a migration version/name/checksum, that identity is immutable.
Reordering the registry after that point breaks upgrades even when the end-state schema is valid.

## Rule

> SQLite migration identity is append-only. Do not insert before, reorder, rename, renumber, or
> change the bytes of an existing Goose SQL migration. New schema work appends the next gap-free
> file and refreshes `atlas.sum` through `make codegen`.

If an existing database reports an integrity mismatch, treat it as a safety signal. Do not weaken
runtime validation, do not accept arbitrary mismatches, and do not manually edit a
`goose_db_version_*` table. Restore exact unpublished history or append a corrective migration,
then add observed-history reopen coverage.

## Operationalization

- Before generating a migration, inspect the owning `schema/migrations/` directory, recent commits,
  and relevant ledgers/tasks for concurrently landed migrations.
- New schema work appends after the highest file version. Chronological neatness is not a
  reason to insert into the middle.
- Edit the owning declarative schema source, run `make codegen`, inspect the Atlas-planned SQL and sqlcheck result,
  and commit the updated `atlas.sum` and sqlc output together.
- Migration tests include fresh database coverage, upgrade/reopen coverage, ahead refusal,
  integrity refusal, gap-free history, and migrations-to-declarative-schema equivalence.
- Keep integrity mismatch failures strict. A mismatch means the binary and database disagree about
  history; fixing that disagreement belongs in exact unpublished history or a new migration.
- Any one-pass data transformation belongs in an ADR-backed appended Goose migration, never in
  boot-time schema repair.

## Anti-pattern

- Inserting a new migration at an older number because it "belongs" earlier in feature chronology.
- Renumbering already-recorded migrations to make a branch merge look sequential.
- Updating tests to the new fresh-DB order without seeding an old DB and reopening it.
- Handling an integrity mismatch by accepting multiple byte identities for one version.
- Manually updating rows in a live `goose_db_version_*` table to match the current binary.

## Source

- Observed local database:
  `sqlite3 /Users/pedronauck/.agh/agh.db 'SELECT version, name, checksum FROM schema_migrations ORDER BY version;'`
- Failing daemon startup:
  `error: daemon: open global database "/Users/pedronauck/.agh/agh.db": store: initialize sqlite database "/Users/pedronauck/.agh/agh.db": store: migration 17 integrity mismatch`
- Historical files `internal/store/globaldb/global_db.go` and `internal/store/schema.go` — removed
  Go registry and runner that produced the incident
- `internal/store/migrate.go` and `internal/store/migrate_integrity.go` — current Goose/Atlas path
- `internal/store/migrate_streams_test.go` — current gap-free and equivalence contracts
- `docs/_memory/lessons/L-008-schema-migrations-mandatory.md`
- `0b371eaa feat: add network threads (#105)`
- `08eedb32 feat: orchestration improvements (#106)`
