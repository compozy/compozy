# All projects + User scope — slice evidence and integrated lab run

Specs: `.compozy/tasks/user-scope-work/` (prerequisite) and `.compozy/tasks/all-projects/`. Branch
`feat/all-projects`. This report consolidates the per-slice worker evidence (each originally its own
dated report on 2026-10-21) and adds the docs-slice lab run. Slice sections are worker verification,
not an integrated browser QA verdict; the trailing QA pair owns the Web journeys.

## A2 — tasks breadth

Originally `docs/qa/reports/2026-10-21-all-projects-tasks-backend.md`.

Slice A2 implements AP BR-12–15 and safety invariants 4–5. The owning cross-surface impact audit is
`.compozy/tasks/all-projects/_impact-entry.md`, consolidated by the delivery coordinator.

### Behavior

- Shared `WorkspaceBreadth` parsing normalizes repeated project filters and rejects conflicting,
  empty, and reserved selectors (T-01–T-05).
- Task catalog and task dashboard keep omission populations and envelopes. Explicit all-workspaces
  reads can narrow to project IDs and `@user`; SQL applies that selection before catalog counts and
  paging and before dashboard aggregation (T-11–T-13).
- Catalog cursor identity includes explicit breadth, normalized project selection, existing filters,
  and profile lens. Legacy query fingerprints omit the new zero-valued fields.
- Overview omission remains global tasks; explicit all selects every task. Overview does not expose
  a project multi-filter. Its attention acknowledgement population uses the same scope resolver.
- Operator guards reject ordinary agent aggregate requests. App middleware preserves the task route
  access decisions and denies overview (T-16–T-17). U5 owns the granted-agent replacement.
- `task list --all-workspaces` conflicts with `--workspace` and bounded `--scope`. Human output joins
  project names and labels User rows; JSON and JSONL retain the original envelope and workspace IDs.
- Native task tools remain pinned by their existing authority rules. No new native tools, hooks,
  configuration, external services, or runtime state directories are introduced.

### Query plan

The actual catalog SQL, including its derived canonical status/activity CTEs, was explained for both
profile lenses. Before the change, both plans contained `SCAN tasks`. Generated migration
`00144_schema.sql` adds `idx_tasks_profile ON tasks(profile_id)` without changing rows.

After migration:

```text
one profile:  SEARCH tasks USING INDEX idx_tasks_profile (profile_id=?)
all profiles: SCAN tasks
both lenses: USE TEMP B-TREE FOR ORDER BY
```

All-profiles reads genuinely select all tasks. The final order derives activity from task, run and
event timestamps; a task timestamp index cannot satisfy that order. No redundant sort index was added.
Existing indexed task-history seeks remain intact.

### Validation

Targeted suites use real SQLite for catalog populations, pagination, dashboard aggregation, migration
application, and schema equivalence. Handler tests verify envelopes and errors. The actual app-token
middleware is exercised against the production route access registry. CLI tests cover flag resolution
and the PROJECT/User table output. Heavy gates and E2E are reserved for CI by the slice brief.

The test-shape checker accepts the new cases; its findings in `global_db_task_test.go` and
`observe/tasks_test.go` are pre-existing direct-assertion test functions outside this change.

Completed checks:

- Focused Go suites in `internal/api/core`, `internal/api/httpapi`, `internal/api/spec`,
  `internal/cli`, and `internal/observe`: passed.
- Real SQLite catalog breadth, query plans, production migration streams, schema equivalence,
  migration application and atlas checksum rejection: passed.
- `go build ./...` and `go vet` on the touched backend packages: passed.
- `make codegen` and `make codegen-check`: passed; OpenAPI, Go/TypeScript clients, contracts and CLI docs regenerated.
- Repository pre-commit checks via `lint-staged --no-stash --no-hide-partially-staged`: passed.
  The regular hook is disabled for this commit because its default stash conflicts with the
  explicit no-stash instruction; the same configured checks were run directly.

### Integration seams

- U5 replaces aggregate `requireOperatorSurface` calls with the granted-agent helper.
- U3 owns migration-created `global` name aliases. The route-agnostic parser currently rejects the
  literal `global`; alias resolution must precede that reserved-name rejection when integrating U3.
- Frontend consumers, the Web journey and consolidated operator skill/release documentation belong
  to their assigned slices. Task/overview query contracts and generated clients are co-shipped here.

## A3 — Loop aggregates

Originally `docs/qa/reports/2026-10-21-all-projects-a3.md`.

Scope: loop runs, human requests, definition catalog, their CLI reads, and the Loop leg of the
operator notification bell. Branch: `ap/a3`. This is slice evidence, not an integrated Web QA verdict.

### Contract coverage

- BR-2/14; T-01/04/06–09/16/17/29: three new app-denied operator HTTP/UDS routes require
  `all_workspaces=true`. Registered projects plus User are selected within the profile lens.
  Runs use `(created_at DESC, id DESC)`; workspace operational ordering stays available.
  Cursors bind profile, breadth, filters, and order. `origin=session` retains the existing Goal filter.
- BR-14/15; T-30/31: requests preserve expiry/opened-at/rowid order, distinguish tied generations/items,
  carry workspace ownership, and join the run's profile before count/page. Cursor reuse across breadth
  or profiles is rejected. Pending and resolved lifecycle suites remain passing.
- BR-15; T-10/32: definitions return `{entries, errors, truncated}` with `source.layer`, up to
  200 per workspace and 2,000 total. User definitions resolve independently of project overrides;
  project-inherited rows require recent project runs. Summary ownership is exact, including zero projects.
  A broken project's definition does not blank healthy projects.
- BR-2/3: the bell issues aggregate requests/node-inventory reads under explicit all-profiles scope,
  de-duplicates request-backed waits using each row's workspace, and labels empty ownership `User`.
- CLI list/runs/requests accept mutually exclusive `--all-workspaces` and `--workspace`. Human tables
  identify projects using the workspace catalog; JSON keeps ownership and aggregate error metadata.
  Native list tools retain ordinary workspace pinning.

### Schema and query plans

`make codegen` generated `internal/store/globaldb/schema/migrations/00146_schema.sql` and refreshed
`atlas.sum` and sqlc. No hand-written data migration or existing migration edit. The three additions are:

- `idx_loop_runs_profile_created(profile_id, created_at DESC, id DESC)`;
- `idx_loop_runs_created(created_at DESC, id DESC)` for the all-profiles recency walk;
- `idx_loop_requests_state_expiry(state, expires_at, opened_at)`.

`TestGlobalDBLoopAggregateQueryPlans` ran `EXPLAIN QUERY PLAN` against the real migrated SQLite schema:

| Read | Observed plan |
| --- | --- |
| Runs, one profile | `SEARCH loop_runs USING INDEX idx_loop_runs_profile_created (profile_id=?)` |
| Runs, all profiles | `SCAN loop_runs USING INDEX idx_loop_runs_created` in recency order; no temporary sort |
| Requests, one profile | `SEARCH request USING INDEX idx_loop_requests_state_expiry (state=?)`; indexed run ID join; profile predicate before page |
| Requests, all profiles | Same request index; covering run ID join |

All reads check registered workspace membership through the workspace primary-key index. Requests
still use a temporary B-tree for the specified null-expiry-first expression ordering; the new index
reduces the candidate scan by state without changing that order. The runs' registered-workspace
predicate uses a correlated existence check to preserve the recency index walk rather than an OR
union followed by a temporary sort.

### Verification

Passed targeted Go checks on the changed source:

- `internal/store/globaldb`: `TestGlobalDBLoopAggregate*`, existing workspace run/origin filtering,
  operational pagination, User isolation, request lifecycle, and node-inventory suites. Real SQLite.
- `internal/daemon`: `TestDaemonLoopCatalog*` and `TestLoopRunList*`; attribution, per-project errors,
  bounds, zero-project User definitions, profile/breadth/order cursor rejection, and 500-row lookahead.
- `internal/api/core`: `TestLoopAggregateHandlers`, `TestBaseHandlersAttentionOperatorScope`, and
  `TestAttentionNotification*`.
- `internal/api/httpapi`, `internal/api/udsapi`: `TestRegisterRoutesCoversTechSpecEndpoints`;
  HTTP `TestAppTokenLoopAggregateAdmission` uses the actual app-access middleware.
- `internal/api/spec`: `TestAppAccessPolicy`.
- `internal/cli`: `TestLoopAllWorkspacesCommands`, existing command/profile/catalog suites, and
  `TestUnixSocketClientLoopCatalog`.
- `internal/store`: `TestProductionMigrationStreams*`, `TestApplyMigrationStream`, and
  `TestApplyRejectsAtlasSumDrift` (fresh/reopen/ahead/integrity coverage).
- `internal/loop`: `TestCoordinatorRunnerShouldParkAskRequests`.
- `go build ./...`; `go vet` on core/spec/cli/daemon/loop/globaldb; changed-test convention checks;
  `git diff --check`.

`make codegen` and `make codegen-check` passed. OpenAPI, TypeScript/Go SDKs, and CLI reference pages
co-ship. The generator also changed only a local minified variable (`css2` to `css`) in the bundled
OpenDesign linter under the installed Bun version; this is generator output with no source behavior change.
`bunx turbo run typecheck --filter=@compozy/client` passed (2/2 tasks, including its codegen-check prerequisite).

No `make gate`, `make gate-full`, full `go test ./...`, E2E, or local race suite was run: the brief
assigns heavy gates to CI and limits race runs to concurrency changes. This slice introduces no
concurrency change. No push or PR publication is part of this worker assignment.

### Integration notes

- U5 replaces `requireOperatorSurface` with the granted-agent guard. No unrestricted grant is simulated.
- A4 owns terminal aggregate reads; the bell's terminal fan-out remains unchanged.
- UW owns aggregate consumers and the “Definitions for {profile}” toolbar. Following the explicit DX
  route contract, `/api/loops` rejects `all_profiles`; the UI selects the acting concrete profile for
  definitions even while other lists display all profiles.
- Existing filter names are preserved (`status`, `run_id`); no aliases for the prose-only `state`/`run`
  variants were introduced for run/request filters.
- New attribution fields may be omitted for User, as allowed by the DX contract, preserving additive
  generated consumer compatibility. Project items always carry their workspace ID.
- The old workspace catalog retains its string `source`. Only the new aggregate uses `source.layer`.
- The integrated browser re-walk in `LP-web-attention-loop-rows` remains pending, explicitly marked
  in the scenario. Backend and real middleware checks do not claim browser evidence.

## A4 — automation and resource aggregates

Originally `docs/qa/reports/2026-10-21-all-projects-a4.md`.

Scope: BR-13/14/15, Safety Invariant 6, T-14/15/17/33–35/38. This report covers the backend and CLI slice; integrated browser scenarios and heavy gates remain orchestration/CI-owned.

### Implementation

- Automation runs persist the source definition's workspace and `recorded` attribution in schedule claims (including skipped fires), webhook claims, and dispatch reservations. Updates do not rewrite attribution. `00148_schema.sql` adds the columns; `00149_automation_run_attribution.sql` derives historical ownership only from surviving source definitions. Unknown ownership stays SQL NULL. Startup logs derived/unknown counts after the migration.
- Automation jobs/triggers accept explicit all-workspace breadth and repeated workspace filters. Run pagination binds profile/source filters and retains a distinct null-start cursor position. Legacy omitted-breadth run limits preserve their previous behavior.
- Suggestions, worktrees, terminals and agents have app-denied aggregate HTTP/UDS reads. Worktrees use stored status without Git discovery. Terminal aggregates are local-only, include User scope, and replay with a selection-bound cursor. Close/delete events carry owner identity. The bell reads terminal inputs once across the population.
- Agent aggregate pagination uses name and workspace together, with metrics read for each owner. CLI aggregate lists show project names and User; run history follows continuation pages. Generated API contracts, OpenAPI, TS/Go SDKs and CLI documentation co-ship.

### Verification

Passed:

- `make codegen` and `make codegen-check` (with an isolated Mage cache).
- Real SQLite upgrade: `TestGlobalDBAutomationAttributionMigration`, including project and User sources and an orphan; full migration stream verified.
- Store cursor/immutability: `TestGlobalDBAutomationRunAttributionAndCursor`, plus existing run filter coverage.
- Real runtime integration: `TestDispatcherIntegrationRunLifecycleStateTransitionsPersist` and `TestSchedulerIntegrationFastScheduleDispatchesThroughDispatcher`.
- Persistent webhook restart: `TestManagerContractWebhookReplay`.
- Real HTTP Gateway: `TestGatewayPrivateTierExcludesLocalAuthorityRoutesIT063IT064`; both terminal aggregate paths return 404. Cases use independent authenticated servers so parallel requests do not exhaust a shared auth limiter. Unregistered routes now use ordinary authentication rather than consuming a stream ticket based solely on their suffix.
- Focused core/API/spec/CLI automation, agent aggregate, terminal catalog, attention, app-access and Gateway classification suites. T-33 pages the same name across User and two projects and asserts separate metrics. App-access policy lists every new operation explicitly.
- `go test -race ./internal/api/core ./internal/terminal -run 'TestTerminalCatalog|TestTerminalAggregate|Test.*Reap'`.
- `go vet` on changed API, automation, CLI, globaldb, terminal and worktree packages; `git diff --check`.

Test invariants live in existing owning suites. The test-shape helper passes the new agent and terminal cases; existing top-level, direct-assertion tests in the automation/store integration files still produce pre-existing heuristic findings. They were not reorganized as part of this slice.

### Integration seams and limitations

- U5 must change the single call in `requireAggregateSurface` from `requireOperatorSurface` to `requireOperatorOrGrantedAgent`; grant admission is intentionally not implemented here.
- U4 may collide with migration numbers 148/149. Renumber the schema and data migration tail together, regenerate `atlas.sum`, and update the attribution-log version check in `automation_attribution_migration.go` and the migration-prefix test boundary.
- `extensions/open-design/lint.gen.mjs` was regenerated by the repository generator and deliberately excluded from this commit per the dispatch instruction.
- Browser QA scenarios have backend steps and this evidence link; no integrated Web walk or heavy CI gate is claimed.

Cross-surface impact is recorded once in `docs/_memory/change-impact.md` under A4.

## U3 — User Loops, Goals, terminals, heartbeat

Originally `docs/qa/reports/2026-10-21-user-scope-u3.md`.

Scope: USW step 4, BR-1/BR-8, T-11 through T-16 and the terminal-journal portion of T-28.
This is worker verification, not a full transport/browser QA verdict.

### Implemented boundaries

- User Loop runs use an empty workspace with an explicit profile. Inventory queries retain exact
  workspace equality, including all-profile reads; request payloads expose `profile_id`.
- User configuration, executed snapshots, UI annotations, admission keys and queue promotion are
  profile-scoped. Project sidecars retain the empty profile key.
- Watch delivery compares exact workspace identities. Goal checkpoint, binding and outbox keys
  accept User runs; the canonical daemon command suite starts and projects a User Goal.
- User terminals resolve profile configuration without project registration. Journal commands,
  recordings and artifacts use the reserved `user-<profile_id>` workspace database directory.
  Profile deletion journals terminal closure/removal and supports lifecycle retry.
- Heartbeat accepts an eligible User session.

### Verification

Passed:

- `make codegen` (SQL bindings, migration, OpenAPI, TypeScript/Go clients, CLI documentation).
- `go build ./...`.
- Focused `go vet` for core/contract, daemon, heartbeat, loop, profile, store/globaldb/workspacedb,
  terminal and journal packages.
- `TestGlobalDBLoopUserScope`: real SQLite config/snapshot/annotation/admission isolation, exact User
  run and request inventories, owner labels, and promotion without changing the other profile's queue.
- Existing project run inventory and concurrent admission regression suites.
- Race detector on User isolation/queue and concurrent admission suites (`go test -race`).
- `TestLoopWatchEventsObserverShouldMatchAndWake`, including project/User separation.
- Canonical Goal command lifecycle, including User start/projection, and Goal key/outbox contracts.
- Terminal manager User open/profile fence/reconnect and real journal record/search/artifact/removal.
- Profile deletion journal failure followed by `RetryOp`, then idempotent recovery.
- Eligible User heartbeat and shared HTTP/UDS User route guard tests.
- `TestProductionMigrationStreamsFreshReopenAndAhead`, including schema equivalence.
- With `-tags integration`: Loop configure/executed-snapshot persistence and Goal session outbox suites.
- Changed-file formatting, generated TypeScript lint and `git diff --check`.

No `make gate`, full Go test suite or E2E run was executed, per the slice brief.

### Integration seams

- U2: the only session-domain edit permits a creation profile with empty workspace when its profile
  is explicit. Reconcile that validation with U2's session runtime implementation. The User Goal test
  uses this seam; full session creation and WM remain U2-owned.
- U4: `00145_schema.sql` only rebuilds the four sidecar tables; legacy pseudo-workspace normalization,
  per-profile copying, aliases and global task/automation repointing remain U4-owned. Renumber the
  generated migration if another slice generated the same number.
- Profile recovery now runs after terminal initialization, so pending User journal removal can execute.
- User journal reads require a concrete profile. Live terminal inventories support owner-labeled
  aggregate reads; a cross-profile journal merge is not introduced by this slice.
- Integrated Web/CLI QA and official skill examples must be reconciled with the remaining slices.
  Scenario additions are recorded in the Loop lifecycle, Goal command and terminal profile scenarios.

## U4 — runtime repoint and pseudo-workspace migration

Originally `docs/qa/reports/2026-10-21-user-scope-u4.md`.

Scope: USW step 5, BR-3/15/16 and the migration portions of BR-1. This is worker verification,
not an integrated transport/browser QA verdict. The branch incorporates A4 `2ad164266` and U5
`9308af676` through the coordinator-requested merges.

### Implemented boundaries

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

### Migration artifacts

- Global generated tail: `00151_schema.sql`.
- Global handwritten data transaction: `00152_user_scope_data.sql`.
- Workspace generated tail: `00005_schema.sql` (journal import ledger).
- Existing A4 00148/00149 and U5 00150 are preserved byte-for-byte. `userScopeMigrationVersion` is
  152. Any later renumber must update that constant and the U4 fixture prefix references together.
- `make codegen` regenerates declarative migrations, SQL bindings and the other owning generators;
  `atlas.sum` includes the separate data migration. Local open-design lint drift is excluded.

### Verification

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

### Final check receipts

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

### Assumptions and integration ownership

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

## U5 — unrestricted agent grant

Originally `docs/qa/reports/2026-10-21-unrestricted-grant-u5.md`.

Scope: User Scope Work BR-9–13; T-22–27/33–35/37. This is slice evidence, not an integrated browser verdict.

### Implemented boundaries

Strict operator-authored AGENT.md grant, untrusted extension stripping, immutable session grant,
authenticated immediate origin, persisted deferred ceilings, atomic narrowing on executable edits,
policy/audit branch, profile selection, aggregate guards and desktop restrictions. Generated API,
SDK and native descriptors co-ship. Web badge/editor and pseudo-workspace migration are separate slices.

### Verification

- `make codegen`: passed, including SQL, OpenAPI, SDK and native catalog generation.
- `go build ./...`: passed.
- Scoped `go vet` on workspaceaccess/config/session/automation/task/loop/api/core/daemon/store/globaldb: passed.
- Config `TestAgentWorkspaceAccess`, extension `TestLoadAgentResources`, policy `TestUnrestrictedGrant` and `TestUserScopeAccessPolicy`: passed.
- Session `TestSessionWorkspaceGrant`: passed, including resume after definition change.
- Automation `TestAutomationGrantCeiling`: passed against SQLite, including executable edit narrowing and operator re-save.
- Globaldb `TestTaskExecutionProfileGrantCeiling`: passed; granted agent cannot restore a previously narrowed ceiling.
- Integration-tagged `TestGlobalDBTaskPersistenceSurvivesReopenWithGlobalAndWorkspaceTasks`: passed against SQLite after reopen.
- Native `TestNativeGrantSelection`, `TestNativeUserSession`, `TestNativeUserWorkspaceReference`, `TestNativeWindowGlobalPartition`, `TestNativeAgentCreate`: passed through the registry or owning boundary.
- Existing scoped agent HTTP mutations, native workspace, task profile, Loop profile and migration stream/schema equivalence checks: passed.

### Existing failure and remaining integration evidence

The broader targeted `TestLoop.*` daemon selection found
`TestLoopWatchEventsObserverShouldApplyDoorbellIsolation/Should_reject_known_foreign_workspace_and_allow_unknown_workspace_payloads`.
The base implementation already compares workspace IDs exactly; the base test expects an empty workspace to match a project.
U3 requires exact matching for User isolation. Neither that test nor the implementation was changed by U5.
The integration owner must reconcile that existing expectation with U3.

The integrated Web/CLI scenario, full dispatcher authority matrix and CI heavy gates remain integration-owned.
No heavy gate, E2E suite or full Go test suite was run under the worker contract.


### A4 integration

Merged `feat/all-projects` at `2ad164266`. Preserved A4 migrations 00148/00149 and regenerated
U5 as `00150_schema.sql`. `requireAggregateSurface` now delegates to
`requireOperatorOrGrantedAgent`, covering the newly integrated automation, suggestions,
worktree, terminal and agent aggregates with the same aggregate audit target.

Passed after integration: codegen; `TestGrantedAggregateProfileScope`; the existing agent,
Loop, task, overview and terminal aggregate handler suites; automation list/attribution cursor
suites; and integration-tagged `TestGlobalDBAutomationAttributionMigration` plus task reopen.
The new HTTP guard test checks granted `all_profiles` admission and the aggregate policy target,
and rejects an ordinary session. The native registry test checks cross-profile User creation
and propagation of the authenticated creator ceiling.

## D — docs-slice lab run (real daemon, 2026-10-21)

Isolated lab: binary built from `ap/docs`, `COMPOZY_HOME=/tmp/apd/home`, HTTP `localhost:47931`, two
registered Git projects (`checkout-api`, `marketing-site`), Claude provider (`claude-haiku-5-5`).
Commands were run with a clean environment (no inherited `COMPOZY_SESSION_ID`) so they act as the
operator.

| Journey | Result |
| --- | --- |
| `session new --user --agent general` | **Failed before fix:** `session: reply watch sender scope is required`; a stopped session record was left behind. Fixed in this branch (reply-watch listing accepts the User scope and filters it exactly). Passes after fix: `scope: "user"`, no `workspace_id`, runtime `unbound`. |
| `session prompt <user-session>` | **Failed before fix:** CLI `cli: session … has no workspace_id`, then daemon `session: prompt workspace is unavailable`. Fixed (CLI routes empty-workspace sessions through `@user`; admitted prompts accept the User scope). Passes after fix: agent replied with `/Users/pedronauck`. |
| Daemon stop/start, prompt again | Pass: the resumed User session recalled the previous turn. |
| `session list --workspace @user`, `--all-workspaces` | Pass: User sessions listed; aggregate shows projects + User. Gap: the human table has no `PROJECT` column (it shows the work directory), unlike the other aggregate lists. |
| `agent list`, `loop list`, `task list`, `automation jobs`, `worktree list`, `terminal list --all-workspaces` | Pass: `PROJECT` column shows `User` and project names. |
| Granted agent (`workspace_access: unrestricted`, `approve-reads`) | Pass: `agent info` prints `workspace_access: unrestricted`; sessions carry `effective_workspace_access: "unrestricted"`; a native `compozy__session_list` on `checkout-api` from a User session was allowed without a prompt and audited (`workspace.access_granted`, `source: agent_grant`, `target_kind: workspace`). |
| Reading the grant audit | HTTP `GET /api/logs?type=workspace.access_granted` works. Gap: `compozy logs --workspace @user` returns `user_scope_unsupported`, and `compozy logs` outside a project fails workspace resolution, so User-actor audit is not readable from the CLI. |
| Breadth errors | Pass: `/api/loop-runs` without breadth → `workspace_breadth_invalid`; `all_workspaces` + selector → `workspace_breadth_invalid`; `workspace_ids=global` → `workspace_reserved`; `session list --workspace global` → `workspace_reserved`. |
| Registration guards | Pass: registering `COMPOZY_HOME` → home-folder refusal; name `global` → `workspace_name_reserved`. |
| `config set shell.sessions.scope all-workspaces` | Pass with a UX gap: the deprecation warning is printed five times per command. `config get shell.sessions.group_by` then reports the derived `project`. |
| Reserved WM partitions | `layout get --workspace @user` → partition `user`; HTTP `/api/workspaces/global/window-manager` → partition `global`. CLI `--workspace global` → `workspace_reserved` (by design: data selector). |

Not walked here (Web slices still in progress): the topbar globe, switcher User destination, Group
by, Project filter, composer pickers, open-in-place, Unrestricted badge, agent editor Access field.
