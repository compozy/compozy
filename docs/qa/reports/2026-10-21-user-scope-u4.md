# U4 User runtime and migration evidence

Scope: USW step 5, BR-3/15/16 and the migration portions of BR-1. This is worker verification,
not an integrated transport/browser QA verdict. The branch incorporates A4 `2ad164266` and U5
`9308af676` through the coordinator-requested merges.

## Implemented boundaries

- Task bridges/roles, global agent automations, Loop judges, managed Goal sessions and their action
  registry select User explicitly. Removed `globalWorkspacePath` wiring. Project Loop targets remain
  project-owned, and U5 origin/grant propagation remains intact (T-17).
- Pre-Goose context identifies COMPOZY_HOME/operator-home registrations and profiles from SQL,
  journal rows and layout partitions. One transactional data migration normalizes legacy User
  templates, applies dispositions, re-keys ownership, merges counters/cursors and archives collisions
  plus workspace-only state. Reserved names rename deterministically with boundary aliases
  (T-18–20, T-30–31). The live schema inventory includes A4 automation run attribution; NULL remains
  unknown, empty string means User, and provenance is unchanged. U5 grant/access values survive both
  re-key and recovery archive serialization.
- Boot reconciliation runs before Loop recovery/User writers. It copies journal rows per profile
  with a durable row ledger, verifies blobs and rewritten paths, retains the source as `.migrated`,
  and copies layouts without replacing existing User state. Registration is removed last through
  a live schema reference assertion, never cascade deletion. Session event ownership stamps remain
  unchanged (T-21, T-32, T-38).
- `global` resolves a migration-created project alias on data surfaces and otherwise returns
  `workspace_reserved`; window-manager global partition semantics remain unchanged. Aliases warn
  through v0.4.0, with removal in v0.5.0 (T-39).

## Migration artifacts

- Global generated tail: `00151_schema.sql`.
- Global handwritten data transaction: `00152_user_scope_data.sql`.
- Workspace generated tail: `00005_schema.sql` (journal import ledger).
- Existing A4 00148/00149 and U5 00150 are preserved byte-for-byte. `userScopeMigrationVersion` is
  152. Any later renumber must update that constant and the U4 fixture prefix references together.
- `make codegen` regenerates declarative migrations, SQL bindings and the other owning generators;
  `atlas.sum` includes the separate data migration. Local open-design lint drift is excluded.

## Verification

The owning existing suites cover:

- `TestUserScopeMigration`: real SQLite prefix upgrade; multiple profiles including journal-only
  ownership; User template/config collisions; sessions/folders/placements; admitted task terminal
  command preservation; usage/palette sums and cursor maximum; automation rename; extension archive;
  run attribution tri-state; U5 grant/access preservation; schema-derived inventory; unknown
  references blocking registration deletion.
- `TestUserScopeMigrationNamesAndKeepProject`: reserved-name aliases, global selector precedence,
  and Loop-target `keep_project` disposition.
- `TestLegacyJournalImport`: real SQLite and files, two profiles, recording/command/artifact copy,
  corrupt-blob refusal followed by repair/retry, preexisting User rows, retained source, ledger
  deduplication and rewritten readable paths.
- `TestUserScopeLayoutMigration`: real clientstate persistence, existing destination precedence,
  retained source, resume after a prior layout copy, registration removed only after both stores
  verify, and repeated reconciliation without changing the target revision.
- Existing task bridge/role, global automation, Loop-target automation, Goal composition, binder,
  judge and workspace-breadth suites; production migration stream fresh/reopen/ahead and schema
  equivalence checks.
- Integration-tagged `TestManagerIntegrationUserLifecycle` (real ACP process) and A4 attribution
  migration/cursor suites.

A4's new regression first failed on both missing run re-key and missing inventory disposition;
production SQL/inventory were then fixed. No test was weakened. No host user data was opened or
migrated; all state verification uses temporary fixture homes.

## Final check receipts

All commands below passed after incorporating U5 and regenerating the final tail:

- `make codegen` and `make codegen-check`.
- `go build ./...`.
- `go vet ./internal/store/globaldb ./internal/store/workspacedb ./internal/clientstate ./internal/daemon ./internal/automation ./internal/api/core`.
- Focused Go tests in globaldb, workspacedb, daemon, core, automation, store and session, selecting
  the migration/runtime suites above plus `TestTaskExecutionProfileGrantCeiling`,
  `TestAutomationGrantCeiling`, `TestSessionWorkspaceGrant`, `TestNativeGrantSelection` and
  `TestGrantedAggregateProfileScope` (`-count=1`). All seven packages passed.
- `go test -tags=integration ./internal/store/globaldb ./internal/session -run 'TestGlobalDBAutomationAttributionMigration|TestGlobalDBAutomationRunAttributionAndCursor|TestManagerIntegrationUserLifecycle' -count=1`.
- Scoped pinned `golangci-lint v2.13.1 fmt` and `git diff --check`. The formatter build emitted a
  macOS linker deprecation warning for `-bind_at_load`; source checks reported no errors.

Local logs: `/tmp/u4-final-tests.log`, `/tmp/u4-final-vet.log`,
`/tmp/u4-final-integration.log`, `/tmp/u4-u5-codegen.log`, `/tmp/u4-final-codegen-check.log`.

## Assumptions and integration ownership

- `task_run_terminal_commands.workspace_id` retains its existing non-null empty-string User
  representation. Tasks, task runs and blocks use NULL. The transaction temporarily removes and
  reinstalls the run-finalization guard so admitted commands survive the re-key.
- Collision archives retain original payloads for recovery. A conflicting preexisting User journal
  row ID fails safely with the source retained rather than overwriting it.
- Current schema has no coordination tables; workspace-only extension/model context state is
  archived. The inventory test fails if a new workspace-reference table appears.
- The fixture suites provide focused behavioral evidence, not exhaustive fault injection at every
  crash boundary or a full transport/browser upgrade walk. The updated global-scope QA scenario,
  integrated Web/CLI journeys, full CI gates and review remain with integration.
- Per the worker brief, no `make gate`, `make gate-full`, full Go suite or E2E suite is run locally.
  Commits disable Husky to avoid lint-staged's automatic stash; scoped formatting and checks run
  explicitly. Existing unrelated test-convention heuristic findings are not cleaned up.
