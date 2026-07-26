# L-008 — Schema migrations are required even on fresh DBs

**Class:** Persistence
**Date discovered:** 2026-04-25 (Hermes BUG-002, Critical)
**Evidence sources:** Hermes BUG-002 + multiple Hermes/autonomy review issues

## Context

The Hermes track widened the `memory_operation_log` table to add `scope`, `workspace_root`, `filename` columns. In the historical pre-Goose implementation, fresh installs passed through `storepkg.EnsureSchema`, but existing databases retained the old five-column table and `agh memory write` failed with `no such column: scope`.

CodeRabbit flagged it as Critical. The historical fix was migration v6 in the removed Go runner. The current implementation carries the lesson forward through per-stream declarative schemas and Goose SQL migrations.

## Root cause

`EnsureSchema`-style boot reconciliation has a fundamental gap: it creates tables that don't exist but does not mutate tables that do. Any column/index/constraint addition needs a real migration; a migration is required _even when fresh installs already work_, because upgrade is a first-class scenario in alpha.

A second contributor was two competing schema paths. The current architecture removes that ambiguity: each stream owns one declarative source (a cohesive `schema.sql` or ordered domain fragments), one gap-free Goose directory, and one `atlas.sum`.

## Rule

> Any SQLite table, column, index, or constraint change MUST update the owning declarative schema source and append the next Goose SQL migration. `EnsureSchema`-style boot reconciliation is forbidden. Test fresh apply and reopen with preserved data.

## Operationalization

- Use `store.Apply` with the embedded `MigrationStream` for every SQLite owner; global and memory remain distinct streams in shared `agh.db`.
- Run `make codegen` after editing the declarative source; inspect the appended SQL, Atlas sqlcheck output, refreshed `atlas.sum`, and regenerated sqlc code.
- Keep existing migration bytes and checksums immutable; append the next gap-free five-digit version.
- Extend the canonical fresh/reopen/ahead/integrity/equivalence suites and run `make codegen-check`.
- Move `.db`, `-wal`, `-shm`, and sibling databases together on recovery or hard-cut refusal.

## Allowed exception

In greenfield alpha, a hard-cut rename + table rewrite without compat migration is allowed when:

1. The change is documented in the techspec's "Delete Targets" section.
2. All callers of the old shape are deleted in the same change.
3. Per-developer wipe of local SQLite is acceptable cost.

## Anti-pattern

- `CREATE TABLE IF NOT EXISTS new_columns ...` then expecting the table to grow.
- Hand-editing a Goose version table or an existing migration to repair history.
- Tests that only cover fresh-DB.

## Source

- `.codex/ledger/2026-04-25-MEMORY-hermes-qa-execution.md` (BUG-002)
- `.compozy/tasks/hermes/reviews-001/issue_020.md` (Critical)
- `.compozy/tasks/refac-v2/reviews-001/issue_001.md` (WAL/SHM Critical)
- `.compozy/tasks/autonomous/memory/task_07.md` (claim/lease schema v7)
- `../analysis/analysis_global_runs.md` lesson L1, `../analysis/analysis_local_runs.md` lesson LL-2
