# Analysis: state-domain-storage

Read-only exploration of the slice `state-domain-storage` (ordinal `02`) for the research prompt above.

## Scope

- Slice question: across every assigned state/domain/storage Go package, identify where the 12 review features produce a concrete correctness, allocation, determinism, evidence, or maintainability gain; reject mismatches; and identify behavior-preserving refactors, with special attention to workspace isolation, storage ordering, generated-code ownership, and domain invariants.
- Primary sources: `internal/automation`, `internal/bundles`, `internal/config`, `internal/demoseed`, `internal/filesnap`, `internal/fileutil`, `internal/frontmatter`, `internal/listcursor`, `internal/memory`, `internal/modelcatalog`, `internal/network`, `internal/notifications`, `internal/observe`, `internal/redact`, `internal/resources`, `internal/settings`, `internal/situation`, `internal/skillscan`, `internal/soul`, `internal/store`, `internal/task`, `internal/vault`, `internal/windowmanager`, `internal/workref`, `internal/workspace`, and `internal/workspaceaccess`.
- Sources read in full vs. sampled: all 1,548 Go files were mechanically inventoried by package, production/test/generated classification, review-feature symbols, timers, process/network APIs, buffer use, slice/list boundaries, sort/order operations, workspace identifiers, row scanners, generated markers, and duplicate normalization shapes. The deep-read working set comprised 25 production/query sources most likely to own candidates; `internal/config/bootstrap_test.go` and `internal/store/sqlite_recovery_test.go` were also read in full. Large canonical suites were sampled around the exact owning subtests and helpers cited below. SQL generator sources were inspected where a generated mapper was a candidate; generated `sqlcgen/*.go` was classified but is not an edit target.
- Total candidate sources surveyed: 1,548 Go files in 52 Go package directories: 1,191 non-generated production files, 302 test files, and 55 generated files. No non-generated production file exceeds 500 lines. Files above 500 lines are tests or generated sqlc output; the large tests are canonical suites rather than production god files.

Package coverage matrix (every assigned Go package is enumerated):

| Assigned tree | Go package directories surveyed | Go files |
| --- | --- | ---: |
| `internal/automation` | `internal/automation` (86), `internal/automation/model` (15) | 101 |
| `internal/bundles` | `internal/bundles` (33), `internal/bundles/model` (2) | 35 |
| `internal/config` | `internal/config` (163), `internal/config/defaults` (1), `internal/config/lifecycle` (2) | 166 |
| `internal/demoseed` | `internal/demoseed` (7) | 7 |
| `internal/filesnap` | `internal/filesnap` (3) | 3 |
| `internal/fileutil` | `internal/fileutil` (11) | 11 |
| `internal/frontmatter` | `internal/frontmatter` (3) | 3 |
| `internal/listcursor` | `internal/listcursor` (2) | 2 |
| `internal/memory` | root (79), `consolidation` (6), `contract` (4), `controller` (8), `extractor` (10), `prompts` (3), `provider/local` (2), `provider/local/memstore` (2), `recall` (7), `scan` (3), `schema` (1) | 125 |
| `internal/modelcatalog` | `internal/modelcatalog` (30) | 30 |
| `internal/network` | root (40), `participation` (9), `rules` (1), `usage` (1) | 51 |
| `internal/notifications` | root (2), `presets` (10) | 12 |
| `internal/observe` | `internal/observe` (44) | 44 |
| `internal/redact` | `internal/redact` (8) | 8 |
| `internal/resources` | `internal/resources` (26) | 26 |
| `internal/settings` | `internal/settings` (72) | 72 |
| `internal/situation` | `internal/situation` (22) | 22 |
| `internal/skillscan` | `internal/skillscan` (4) | 4 |
| `internal/soul` | `internal/soul` (19) | 19 |
| `internal/store` | root (103), `globaldb` (347), `globaldb/schema` (1), `globaldb/sqlcgen` (51), `sessiondb` (31), `sessiondb/schema` (1), `sessiondb/sqlcgen` (4), `workspacedb` (3), `workspacedb/schema` (1) | 542 |
| `internal/task` | `internal/task` (145) | 145 |
| `internal/vault` | `internal/vault` (6) | 6 |
| `internal/windowmanager` | `internal/windowmanager` (80) | 80 |
| `internal/workref` | `internal/workref` (3) | 3 |
| `internal/workspace` | `internal/workspace` (25) | 25 |
| `internal/workspaceaccess` | `internal/workspaceaccess` (6) | 6 |
| **Total** | **52 package directories** | **1,548** |

## Overview

The review features are not uniformly applicable. The strongest applications are test-only `testing/synctest` around actual timers and scheduler goroutines; `sync.OnceFunc` for returned idempotent cleanup; `sync.OnceValues` for two existing result/error memoizers; narrowly scoped `cmp.Or` for already-normalized config fallback; and the new testing evidence APIs for diagnostic logging, test metadata, and failed SQLite artifacts. The remaining features have no honest in-slice production target: there is no `math/rand`, direct `net.Dialer`, inbound production HTTP handler, `runtime/trace`, `os.Process` handle manipulation, or read-oriented `bytes.Buffer` use in this corpus.

The highest-value non-feature refactor is the task-run storage read path. `GetTaskRun` and `ListTaskRunsByStatus` use two sqlc-generated row types and two near-identical 40-plus-field mappers, while `ListTaskRuns` already uses one canonical projection plus `scanTaskRunRecord`. This is drift-prone specifically around `workspace_id`, network participation, review lineage, claim-token redaction, and ordering. The safe direction is to make one hand-owned projection/scanner the read boundary, remove the redundant named SQL queries at their generator source, and regenerate; generated Go must never be edited directly.

Two correctness-sensitive findings deserve priority. First, exponential automation retry uses an unchecked shift/multiply while validation only requires `max_retries > 0`; sufficiently large attempts can overflow `time.Duration` and collapse the intended positive backoff. Second, `HeaderListQuery.WorkspaceID` looks at first glance like an omitted record filter, but the owning catalog already returns one bound storage identity and the canonical test intentionally lists global headers while binding the cursor to `workspace-a`. Adding a naive `header.WorkspaceID == query.WorkspaceID` filter would break the current global-memory contract. This ambiguity should be documented and covered at the catalog boundary, not “fixed” inside the paginator without a cross-surface contract decision.

The production layout is already substantially decomposed: no hand-written production source exceeds the 500-line cap. The main smells are repeated generated/manual row mappings, timer tests that poll or sleep, semantically ambiguous string primitives, and many superficially similar trim/deduplicate helpers whose differences are business rules (sort vs. preserve order, reject vs. drop empty, case-fold vs. preserve case). A generic cross-package normalizer would erase those distinctions and is rejected. Mechanical reference searches did not establish any safe dead-code deletion; dead code should not be claimed without whole-repository compile/static-analysis evidence outside this slice.

## Mechanisms / Patterns

| Review feature | Mechanism found in this slice | Verdict |
| --- | --- | --- |
| `testing/synctest` | Run timer/goroutine tests in a fake-time bubble and wait for durable blocking instead of polling/sleeping. | **APPLY** to selected canonical tests. |
| `iter.Seq` / range-over-func | Candidate lists are public/paginated, sorted, counted, sqlc-materialized, resource-owning, or must be fully authorized before mutation. | **REJECT** current candidates; no allocation win without losing an invariant. |
| `os.Process.WithHandle` | The only production subprocess path is `exec.CommandContext`; it does not construct or retain an `os.Process` handle. | **REJECT**. |
| `sync.OnceValue` / `sync.OnceFunc` | Existing first-call result/error snapshots and one returned idempotent cleanup map directly to `OnceValues`/`OnceFunc`. Stateful “first argument wins” or channel-close structs do not. | **APPLY** narrowly. |
| `math/rand/v2` | All random use is `crypto/rand` for keys, nonces, IDs, lease tokens, and workspace IDs. | **REJECT**; substitution would be a security regression. |
| `cmp.Or` | A simple normalized explicit/default config selection can use it; provider/model/reasoning selection also tracks provenance and must remain explicit. | **APPLY** once; **REJECT** broad rewrite. |
| `T.ArtifactDir` / `T.Attr` / `T.Output` | Failed SQLite families are useful artifacts; storage topology is useful metadata; discarded test loggers lose failure evidence. | **APPLY** selectively; do not replace functional `TempDir` fixtures or assertion buffers. |
| `net/http.CrossOriginProtection` | This slice owns HTTP clients and `httptest` handlers, not an inbound production server handler. | **REJECT** in-slice. |
| `runtime/trace.NewFlightRecorder` | No process trace lifecycle exists here; package-level adoption would create multiple recorders and unclear ownership. | **DEFER** to daemon/bootstrap observability ownership outside the slice. |
| `net.Dialer.DialUnix` / `DialTCP` | No direct `net.Dialer` or `net.Dial` call exists; HTTP transport owns connection establishment. | **REJECT**. |
| `bytes.Buffer.Peek` | Buffers are write-only encoders/log captures/subprocess stdout-stderr; row iteration is `database/sql`, not a buffer parser. | **REJECT**. |
| `unique` | High-cardinality workspace/session/task IDs and secrets must not be process-global interned; bounded enums would save little while changing domain types. | **REJECT** now; profile before reconsidering bounded internal-only values. |

Additional architectural patterns:

- **Bound-storage identity before pagination:** `Store.CatalogHeaders` derives scope/workspace/agent identity before reading, while `BuildHeaderListPage` sorts, counts, filters semantic fields, and binds the cursor fingerprint. Workspace isolation belongs at the store/catalog boundary; cursor binding is not automatically a row predicate.
- **Workspace identity is a storage invariant, not a display field:** task-run creation reconciles task, run, and network workspace IDs before persistence. Any mapper consolidation must preserve that field and all validation after scanning.
- **Materialize before authority-sensitive mutation:** task trees are collected before filtering/authorizing or canceling. A lazy sequence could allow partial effects before discovering an unauthorized descendant.
- **One projection, one scanner:** dynamic task-run reads already centralize the ordered projection and row scanner. Parallel sqlc row mappers duplicate the same schema contract and invite field drift.
- **Generator-owned generated code:** the 55 `Code generated ... DO NOT EDIT` files are outputs. Query changes start in `internal/store/globaldb/queries/*.sql` or `internal/store/sessiondb/queries/*.sql`, followed by the repository codegen owner.
- **Sticky memoized error semantics:** both migration-plan compilation and model-source snapshotting cache errors as well as values. `sync.OnceValues` is correct only if the replacement preserves “first result, including failure, is reused.”
- **Cancellation and fake time are separate invariants:** timer tests must prove both deadline progress and immediate context cancellation. `synctest` should remove wall-clock waits, not eliminate cancellation assertions.
- **Domain-specific normalization over a universal helper:** apparently duplicated string-set functions differ on empty values, ordering, validation, and case folding. Those differences are contracts, not noise.

## Relevant Sources

- Memory catalog identity and paging: `internal/memory/header_catalog.go:12-48`, `internal/memory/store_catalog.go:21-113`, `internal/memory/header_list.go:34-117`, `internal/memory/header_list.go:127-186`, `internal/memory/header_list.go:191-271`, and `internal/memory/store_memv2_test.go:140-247`.
- Timer/concurrency candidates: `internal/memory/consolidation/runtime.go:104-143`, `internal/memory/consolidation/runtime_test.go:177-185`, `internal/memory/consolidation/runtime_test.go:994-1005`, `internal/memory/extractor/runtime_queue.go:233-252`, `internal/memory/extractor/runtime_test.go:977-1029`, `internal/windowmanager/active_coalescer.go:14-114`, `internal/task/manager_run_records.go:292-313`, and `internal/memory/store_memv2_test.go:338-410`.
- Once candidates: `internal/redact/dynamic.go:26-42`, `internal/modelcatalog/service_refresh.go:155-183`, `internal/modelcatalog/service_test.go:2025-2085`, and `internal/store/migration_plan.go:24-66`.
- Config precedence: `internal/config/bootstrap.go:30-39`, `internal/config/bootstrap_test.go:10-24`, and the provenance-sensitive fallbacks in `internal/config/provider_resolve.go:119-174`.
- Retry correctness: `internal/automation/dispatch_retry.go:122-145`, `internal/automation/model/validate.go:199-243`, and `internal/automation/dispatch_test.go:1394-1422`.
- Task-run storage duplication and workspace binding: `internal/store/globaldb/global_db_task.go:49-61`, `internal/store/globaldb/global_db_task_runs.go:38-68`, `internal/store/globaldb/global_db_task_runs.go:217-334`, `internal/store/globaldb/task_core_mapping.go:62-189`, `internal/store/globaldb/global_db_task_run_scan.go:14-166`, and the generator owner `internal/store/globaldb/queries/task_core.sql:152-181`.
- Evidence API candidates: `internal/store/sqlite_recovery_test.go:15-188`, `internal/store/globaldb/global_db_test.go:3131-3162`, `internal/memory/catalog_migration_test.go:20-137`, `internal/memory/consolidation/runtime_test.go:983-991`, and `internal/workspace/resolver_test.go:2588-2628`.
- Rejected feature evidence: `internal/modelcatalog/live_sources.go:100-132` (`exec.CommandContext`, write-only stdout/stderr buffers), `internal/modelcatalog/live_sources.go:138-231` (HTTP client ownership), `internal/vault/crypto.go:162-284`, `internal/store/id.go:11-18`, `internal/task/lease.go:174-187`, `internal/workspace/helpers.go:131-142` (`crypto/rand`), `internal/fileutil/targzip.go:17-32` (write-only buffer), and `internal/network/manager_logging.go:180-205` (write-only compaction buffer).

## Transferable Patterns

1. **Replace wall-clock polling with `testing/synctest` in timer-owned canonical suites — APPLY.**
   - What/where: convert the ticker subtest in `internal/memory/consolidation/runtime_test.go:167-186`, throttle-flush polling helpers in `internal/memory/extractor/runtime_test.go:977-1029`, and positive grace-period coverage for `internal/task/manager_run_records.go:292-313` to synctest bubbles. Add a timer-specific subtest to the existing window-manager service suite for the quiet-period/reset behavior in `internal/windowmanager/active_coalescer.go:53-114`; the current tests manually flush pending state (`internal/windowmanager/service_test.go:1015-1026`) and do not prove the two-second timer path. The mutex-blocking negative wait in `internal/memory/store_memv2_test.go:338-410` is also a candidate for `synctest.Wait` instead of a 250 ms timer.
   - Replaces/augments: `time.Sleep`, polling tickers, fixed real-time negative assertions, and untested positive timer expiration. Production clocks and scheduler code remain unchanged.
   - Benefit: deterministic tests, no 10/30/250 ms wall-clock tax, stronger proof that rescheduling, cancellation, and goroutine quiescence work under `-race`.
   - Invariant/business risk: keep one test for immediate context cancellation; preserve per-workspace timer keys so one workspace cannot cancel or flush another; do not place real network-backed `httptest.Server` timeout tests in a synctest bubble unless their blocking behavior is proven compatible.
   - Canonical test owner: `TestRuntime`/existing subtests in `internal/memory/consolidation/runtime_test.go`, the existing extractor runtime suite, `internal/windowmanager/service_test.go`, `internal/task/manager_test.go`, and `TestMemoryHeaderListPage` in `internal/memory/store_memv2_test.go`. Do not add separate one-off timer test files.
   - Effort: **M** across the selected suites. Cross-surface impact: none; test-only.

2. **Use `sync.OnceFunc` for the dynamic-secret cleanup closure — APPLY.**
   - What/where: `RegisterDynamicSecret` currently allocates a local `sync.Once` and returns a wrapper calling `once.Do` (`internal/redact/dynamic.go:26-42`). Return `sync.OnceFunc(func() { dynamicSecrets.unregister(secret) })` after registration.
   - Replaces/augments: the hand-written idempotent closure; the short-secret no-op remains a no-op.
   - Benefit: directly expresses the contract and removes mutable closure plumbing without changing registry locking or snapshot ordering.
   - Invariant/business risk: cleanup remains idempotent, removes exactly one registration reference, and must not intern or retain the secret after final unregister.
   - Canonical test owner: the existing “Should redact registered dynamic secrets until cleanup” subtest at `internal/redact/redact_test.go:142-145`; augment it by calling cleanup twice if not already asserted.
   - Effort: **XS**. Cross-surface impact: none; secret redaction behavior is unchanged.

3. **Represent model-source snapshot memoization with `sync.OnceValues` — APPLY.**
   - What/where: `sourceRefreshSnapshot` stores `once`, `rows`, and `err`, then clones/filter rows on every provider view (`internal/modelcatalog/service_refresh.go:155-183`). Construct a memoized `func() ([]ModelRow, error)` with `sync.OnceValues`, capture the globalized `ListOptions` and refresh context at snapshot construction, and continue cloning before provider filtering.
   - Replaces/augments: three mutable cache fields and the `once.Do` block.
   - Benefit: makes “one source read produces both rows and error” atomic and explicit; reduces the chance that future code reads a cached field without the once barrier.
   - Invariant/business risk: the first result and first error are sticky; the source is called exactly once for one global refresh snapshot; cached rows are never exposed for mutation; provider filtering happens after cloning; cancellation continues to use the refresh call's context, not a background context.
   - Canonical test owner: `TestCatalogServiceRefreshConcurrency`, especially “Should coalesce provider-scoped refreshes inside a concurrent global refresh” at `internal/modelcatalog/service_test.go:2025-2085`.
   - Effort: **S**. Cross-surface impact: none if call counts, provider ordering, and status errors remain identical.

4. **Represent migration-plan cache entries with `sync.OnceValues` — APPLY.**
   - What/where: `migrationPlanCacheEntry` stores `once`, `plan`, and `err` and `prepareMigrationPlan` mutates both fields (`internal/store/migration_plan.go:24-51`). Store a memoized `func() (*migrationPlan, error)` created under `migrationPlanCacheStore.mu` for the digest.
   - Replaces/augments: mutable result/error fields and the external `once.Do` call.
   - Benefit: one immutable callable per content digest; value/error publication remains synchronized by the standard primitive.
   - Invariant/business risk: a compile failure remains cached for that digest; the digest must uniquely identify the entire migration directory; concurrent callers compile once; migration order and Goose transaction mode remain unchanged.
   - Canonical test owner: existing `TestApplyMigrationStream`, `TestMigrationDirectoryValidation`, and migration stream concurrency/reopen cases in `internal/store/migrate_test.go` and `internal/store/migrate_streams_test.go`. Add a subtest to the owning migration suite only if a call-count seam is necessary; do not create a new cache-only test file.
   - Effort: **S**. Cross-surface impact: no schema or migration bytes change.

5. **Use `cmp.Or` only for normalized fallback with no provenance — APPLY narrowly.**
   - What/where: `ResolveAgentName` has two identical trim/non-empty branches (`internal/config/bootstrap.go:30-39`). Compute the first non-zero value from `strings.TrimSpace(name)` and `strings.TrimSpace(defaults.Agent)`, then retain the existing required-value error.
   - Replaces/augments: simple explicit/default selection.
   - Benefit: the precedence rule is one expression and remains easy to scan.
   - Invariant/business risk: trim each candidate before `cmp.Or`; explicit name wins; empty/whitespace values fall through; error text is unchanged. Do not apply to provider/model/reasoning selection, which records `RuntimeValueSource` at `internal/config/provider_resolve.go:119-174`.
   - Canonical test owner: `TestResolveAgentNameFallsBackToDefaults` at `internal/config/bootstrap_test.go:10-24`; add explicit-name and all-whitespace subtests in that suite.
   - Effort: **XS**. Cross-surface impact: no config key/default/docs change; behavior remains identical.

6. **Use the new testing evidence APIs as evidence, not fixture storage — APPLY selectively.**
   - What/where:
     - Route non-asserted test logs now sent to `io.Discard` through `t.Output()` by making package test logger helpers accept `*testing.T` (`internal/memory/consolidation/runtime_test.go:983-991`, `internal/workspace/resolver_test.go:2588-2628`, `internal/network/test_helpers_test.go:1-10`, `internal/bundles/service_test.go:20-30`). Keep `bytes.Buffer` where the log text itself is asserted.
     - Add `T.Attr` at migration/storage topology entry points, such as the global DB test helper (`internal/store/globaldb/global_db_test.go:3131-3162`) and shared global+memory stream cases (`internal/memory/catalog_migration_test.go:20-137`), with stable low-cardinality attributes such as storage scope and migration stream. Do not attach workspace/session IDs that create high-cardinality CI dimensions.
     - On failure, copy the SQLite database family relevant to corruption/recovery tests into `t.ArtifactDir()` (`internal/store/sqlite_recovery_test.go:15-188`). The current `TempDir` remains the live fixture; artifact copies are diagnostic evidence only.
   - Replaces/augments: discarded diagnostics and ad hoc inability to inspect a failed DB/WAL/SHM family. It does not replace `TempDir`, `t.Log`, or assertion buffers.
   - Benefit: useful failure output, machine-readable test classification, and reproducible storage evidence without making successful tests noisy.
   - Invariant/business risk: never copy secrets or provider credentials; close/checkpoint handles as required before copying; artifact creation must not mask the primary test failure; attributes must be stable.
   - Canonical test owner: the existing package suites named above.
   - Effort: **S-M**. Cross-surface impact: test infrastructure only.

7. **Make automation exponential backoff overflow-safe — APPLY (correctness refactor).**
   - What/where: `retryDelay` multiplies by `1 << (attempt-1)` without checking shift/multiplication overflow (`internal/automation/dispatch_retry.go:132-145`), while validation only checks `MaxRetries > 0` and positive duration (`internal/automation/model/validate.go:199-243`). Add checked exponential growth and return a validation/runtime error before a positive delay wraps to zero or negative.
   - Replaces/augments: unchecked integer arithmetic. No jitter or new policy is introduced.
   - Benefit: prevents a malformed but currently accepted retry configuration from converting “backoff” into immediate retry or a nonsensical duration.
   - Invariant/business risk: valid existing retry schedules remain byte-for-byte equivalent; attempt 1 is the base delay; delays are monotonic and positive; define whether overflow is rejected during config validation, runtime calculation, or both. A hard `max_retries` cap would be a public config rule and needs an explicit product decision; checked arithmetic alone is behavior-preserving for valid values.
   - Canonical test owner: `TestRetryDelayHelpersAndContextAwareSleep` at `internal/automation/dispatch_test.go:1394-1422` plus the existing `RetryConfig.Validate` table in `internal/automation/validate_test.go`.
   - Effort: **S**. Cross-surface impact: user-visible error only for previously overflowing configuration; audit CLI/config diagnostics and official Compozy skill only if a new documented maximum is introduced.

8. **Consolidate task-run reads on the existing projection/scanner and regenerate sqlc — APPLY as a generator-owned refactor.**
   - What/where: use `taskRunSelectColumnsSQL` and `scanTaskRunRecord` for `GetTaskRun`, `ListTaskRuns`, and `ListTaskRunsByStatus` (`internal/store/globaldb/global_db_task.go:49-61`, `internal/store/globaldb/global_db_task_run_scan.go:14-166`). Remove the duplicate named reads from the generator source `internal/store/globaldb/queries/task_core.sql:152-181`, then regenerate. Delete `taskRunFromGenerated` and `taskRunFromStatusGenerated` only after all callers migrate (`internal/store/globaldb/task_core_mapping.go:62-189`). Never hand-edit `internal/store/globaldb/sqlcgen/task_core.sql.go`.
   - Replaces/augments: two duplicated sqlc projections, two generated row types, and two near-identical manual mappings. Insert/update sqlc ownership may remain unchanged.
   - Benefit: one ordered projection and one validated decoder own workspace, network, review, token, JSON, and timestamp semantics; future columns cannot silently land in one read path only.
   - Invariant/business risk: preserve `'' AS claim_token` redaction, `workspace_id`, `network_*`, review lineage, `queued_at ASC, id ASC` for status lists, not-found error mapping, row close/error handling, capability hydration, and the post-scan `Run.Validate`. Workspace binding on write (`internal/store/globaldb/global_db_task_runs.go:38-68`) must remain authoritative.
   - Canonical test owner: existing task-run cases in `internal/store/globaldb/global_db_task_test.go:2347-2419`, reopen coverage in `internal/store/globaldb/global_db_task_integration_test.go:140-207`, and row-close failure coverage in `internal/store/globaldb/rows_close_error_test.go`.
   - Effort: **M**. Cross-surface impact: intended none; storage ordering and workspace isolation require exact regression assertions. Codegen output changes are expected and must come from the query source.

9. **Move package-wide fallback helpers out of feature-owned files — APPLY at low priority.**
   - What/where: root `internal/memory` defines `firstNonEmpty` in `catalog_document.go` but uses it from catalog scan and dreaming; `internal/memory/extractor` defines its package-wide fallback in `events.go` but uses it from runtime queue/turn code (`internal/memory/catalog_document.go:31-38`, `internal/memory/extractor/events.go:62-68`, `internal/memory/extractor/runtime_queue.go:38-52`). Move each unchanged helper to a narrowly named package helper file; do not export or centralize it across packages.
   - Replaces/augments: accidental file ownership/coupling, not the algorithm.
   - Benefit: feature files regain one responsibility and shared normalization ownership becomes visible.
   - Invariant/business risk: the memory helper returns trimmed values, whereas `internal/soul/persistence.go:366-372` intentionally returns the original nonblank value. Do not merge those semantics.
   - Canonical test owner: existing memory catalog, dream, and extractor runtime suites; no implementation-detail-only test is needed.
   - Effort: **XS-S**. Cross-surface impact: none.

10. **Clarify the memory header `WorkspaceID` contract before changing filtering — DEFER.**
    - What/where: `HeaderListQuery.WorkspaceID` is normalized and fingerprinted (`internal/memory/header_list.go:127-130`, `internal/memory/header_list.go:191-204`) but not checked by `headerMatchesListQuery` (`internal/memory/header_list.go:170-186`). However, `Store.CatalogHeaders` explicitly returns headers for one bound storage identity (`internal/memory/header_catalog.go:12-48`), and the canonical test passes `ScopeGlobal` plus `WorkspaceID: "workspace-a"` while expecting a global header (`internal/memory/store_memv2_test.go:224-245`). This is consistent with workspace ID being cursor/request context, not a row predicate.
    - Replaces/augments: add contract comments and an end-to-end isolation assertion at the catalog caller; consider a less ambiguous field name only as a coordinated breaking change.
    - Benefit: prevents a future “obvious fix” from hiding global memories or, conversely, a caller from feeding mixed identities into a paginator that assumes pre-scoped input.
    - Invariant/business risk: global memories remain visible in a workspace context; workspace/agent stores cannot leak headers from another bound root; cursors cannot be replayed under a different workspace context.
    - Canonical test owner: `TestMemoryHeaderListPage` and the bound-agent catalog cases at `internal/memory/store_memv2_test.go:430-490`, plus the consuming API/native-tool suite outside this slice.
    - Effort: **S** for documentation/tests, **M-L** for a public rename. Cross-surface impact: memory CLI/HTTP/UDS/native-tool list contracts must be audited before any semantic or field rename; official Compozy skill impact is possible if public request fields change.

## Risks / Mismatches

- **`iter.Seq` / range-over-func — REJECT for current boundaries.** `BuildHeaderListPage` needs full filtering, deterministic sort, total count, stable cursor cut, and `HasMore` (`internal/memory/header_list.go:76-117`). `Store.CatalogHeaders` closes database rows and returns a complete bound snapshot. Task trees must be fully collected and authorized before mutation (`internal/task/manager_run_records.go:79-102`, `internal/task/manager_task_triage_cancel.go:110-140`). sqlc list methods materialize by contract; exposing a sequence over `*sql.Rows` would move close/error responsibility to callers and make early termination hazardous. Filesystem scan results also couple ordered paths with a snapshot map (`internal/skillscan/scan.go:26-35`). A sequence that internally collects/sorts first removes no meaningful allocation.
- **`os.Process.WithHandle` — REJECT.** The only production process start is `exec.CommandContext` with `Run` in `internal/modelcatalog/live_sources.go:100-132`. There is no raw handle adoption, `os.FindProcess`, or retained process whose handle lifetime needs transfer. Reaching through `os/exec` would add platform coupling.
- **`math/rand/v2` — REJECT.** Randomness in `internal/vault/crypto.go:162-284`, `internal/store/id.go:11-18`, `internal/task/lease.go:174-187`, and `internal/workspace/helpers.go:131-142` is security/identity material from `crypto/rand`. `math/rand/v2` is not a substitute and there is no simulation/shuffle workload in scope that needs it.
- **Broad `cmp.Or` conversion — REJECT.** Provider/model/reasoning selection changes provenance fields based on which fallback wins (`internal/config/provider_resolve.go:119-174`). Collapsing it into `cmp.Or` would either lose provenance or require a second duplicated decision tree. Other `firstNonEmpty` helpers trim, normalize enum values, or intentionally preserve the original spelling; zero-value comparison alone is not their contract.
- **Bulk `TempDir` to `ArtifactDir` conversion — REJECT.** The many `TempDir` uses are live filesystem/database fixtures whose automatic cleanup is desirable. `ArtifactDir` is appropriate only for diagnostic copies that the operator needs after a failure. Likewise, buffers used to assert redaction/logging must not be replaced by `T.Output`.
- **`net/http.CrossOriginProtection` — REJECT in-slice.** `internal/modelcatalog/live_sources.go:138-231` owns outbound clients. The only handlers found are `httptest.NewServer` callbacks in modelcatalog tests (`internal/modelcatalog/live_sources_test.go:414-434`, `internal/modelcatalog/live_sources_test.go:801-824`). Cross-origin protection belongs around the real inbound HTTP mux outside this slice, where trusted-origin and UDS behavior can be assessed.
- **`runtime/trace.NewFlightRecorder` — DEFER.** Starting a recorder from `internal/observe`, a repository, or a scheduler would create unclear process lifetime, memory budgeting, dump authorization, and secret-redaction ownership. Flight recording should be one daemon-level facility with config lifecycle, CLI/HTTP/UDS retrieval, bounded storage, and security review. No in-slice package is that owner.
- **`net.Dialer.DialUnix` / `DialTCP` — REJECT.** No direct dialing exists in the 1,548 Go files. Model discovery uses `http.Client`, so address-family choice is a transport concern. Adding explicit dial methods would bypass proxy/TLS/transport behavior for no demonstrated gain.
- **`bytes.Buffer.Peek` — REJECT.** `internal/fileutil/targzip.go:17-32`, `internal/network/manager_logging.go:180-205`, config rendering, prompt rendering, and subprocess stdout/stderr only write and then consume the entire buffer/string. There is no current parse decision that benefits from non-consuming lookahead.
- **`unique` — REJECT.** Interning workspace/session/task/run IDs would retain unbounded values process-wide and undermine lifecycle-based reclamation. Interning dynamic secrets from `internal/redact/dynamic.go:26-73` would be a direct security retention bug. Converting public domain string fields to `unique.Handle[string]` would break JSON/storage/API types. Bounded status/kind enums are repeated, but their strings are short and the handle conversion cost/contract churn has no profile-backed benefit.
- **Naive workspace filtering inside `BuildHeaderListPage` — REJECT pending contract decision.** The input contract is already one bound identity, and global scope intentionally coexists with a workspace-bound cursor. Filtering every header by query workspace would hide global records. Instead, prove isolation at `CatalogHeaders` and the consuming surface.
- **One generic trim/deduplicate helper — REJECT.** Examples differ materially: task capability normalization preserves empty values so validation can reject them (`internal/store/globaldb/global_db_task_helpers.go:213-232`), bridge capabilities drop empties and sort (`internal/store/globaldb/global_db_bridge_targets.go:461-479`), session exclusions optionally case-fold and preserve input order (`internal/store/globaldb/global_db_session_page.go:176-205`), and network peer IDs validate then sort (`internal/store/network_validation.go:125-145`). Generalizing them behind options would replace explicit domain policy with boolean/strategy complexity.
- **Editing generated sqlc Go — REJECT.** The 55 generated files are outputs. For the task-run consolidation, `internal/store/globaldb/queries/task_core.sql:152-181` is the source owner; generated `internal/store/globaldb/sqlcgen/task_core.sql.go` changes only through code generation.
- **Stateful `sync.Once` blanket conversion — REJECT.** `redact.Engine.SnapshotEnabled` accepts the first runtime argument and freezes process state (`internal/redact/engine.go:20-53`); a package-initialized `OnceValue` cannot represent that first-call argument without changing semantics. Channel-close guards embedded in subscribers are also already clear and should only move to `OnceFunc` when it reduces, rather than relocates, state.
- **Dead-code deletion — DEFER.** Mechanical declaration/reference scans did not prove a safe delete target. Whole-repository compile/static analysis and external interface consumers were outside this scoped write; claiming dead code from local textual references would be unsafe.

## Open Questions

1. The active Go/toolchain version and experiment policy are outside the authorized slice. Before implementation, confirm that `testing/synctest`, `sync.OnceValues`/`OnceFunc`, `cmp.Or`, and `T.ArtifactDir`/`T.Attr`/`T.Output` are available in the repository's declared toolchain and accepted by all CI platforms.
2. Is `HeaderListQuery.WorkspaceID` formally a cursor/request-context binding, as the current global-scope test implies, or should workspace-scoped calls additionally enforce it as a record predicate? Resolve this at the consuming memory API/native-tool contract before changing behavior or naming.
3. Which daemon-level package owns a potential `runtime/trace.NewFlightRecorder` lifecycle, and what agent-manageable CLI/HTTP/UDS surface, config keys, retention limit, access control, and redaction rules would govern it? No package in this slice can answer that safely.
4. Does CI retain `T.ArtifactDir` output automatically, or must the test runner publish it? The SQLite recovery recommendation depends on that operational contract and must avoid copying credential-bearing databases.
5. For the task-run mapper consolidation, confirm the repository's sqlc configuration/codegen command outside this slice and whether removing `GetTaskRun`/`ListTaskRunsByStatus` named queries affects any generated interface consumer outside `internal/store/globaldb`.

## Evidence

- `internal/automation/dispatch_retry.go:122-145`
- `internal/automation/dispatch_test.go:1085-1142`
- `internal/automation/dispatch_test.go:1394-1422`
- `internal/automation/model/validate.go:199-243`
- `internal/config/bootstrap.go:30-39`
- `internal/config/bootstrap_test.go:10-24`
- `internal/config/provider_resolve.go:119-174`
- `internal/fileutil/targzip.go:17-32`
- `internal/memory/catalog_document.go:31-38`
- `internal/memory/consolidation/runtime.go:104-143`
- `internal/memory/consolidation/runtime_test.go:167-186`
- `internal/memory/consolidation/runtime_test.go:983-1005`
- `internal/memory/extractor/events.go:62-68`
- `internal/memory/extractor/runtime_queue.go:35-52`
- `internal/memory/extractor/runtime_queue.go:233-252`
- `internal/memory/extractor/runtime_test.go:977-1029`
- `internal/memory/header_catalog.go:12-48`
- `internal/memory/header_list.go:34-117`
- `internal/memory/header_list.go:127-204`
- `internal/memory/header_list.go:231-271`
- `internal/memory/store_catalog.go:21-113`
- `internal/memory/store_memv2_test.go:140-247`
- `internal/memory/store_memv2_test.go:338-410`
- `internal/memory/store_memv2_test.go:430-490`
- `internal/modelcatalog/live_sources.go:100-132`
- `internal/modelcatalog/live_sources.go:138-231`
- `internal/modelcatalog/live_sources_test.go:414-434`
- `internal/modelcatalog/live_sources_test.go:801-824`
- `internal/modelcatalog/service_refresh.go:155-183`
- `internal/modelcatalog/service_test.go:2025-2085`
- `internal/network/manager_logging.go:180-205`
- `internal/redact/dynamic.go:26-73`
- `internal/redact/engine.go:20-53`
- `internal/redact/redact_test.go:142-145`
- `internal/skillscan/scan.go:26-35`
- `internal/store/globaldb/global_db_session_page.go:176-205`
- `internal/store/globaldb/global_db_task.go:49-61`
- `internal/store/globaldb/global_db_task_helpers.go:213-232`
- `internal/store/globaldb/global_db_task_run_scan.go:14-166`
- `internal/store/globaldb/global_db_task_runs.go:38-68`
- `internal/store/globaldb/global_db_task_runs.go:217-334`
- `internal/store/globaldb/global_db_task_test.go:2347-2419`
- `internal/store/globaldb/global_db_task_integration_test.go:140-207`
- `internal/store/globaldb/global_db_test.go:3131-3162`
- `internal/store/globaldb/queries/task_core.sql:152-181`
- `internal/store/globaldb/task_core_mapping.go:62-189`
- `internal/store/id.go:11-18`
- `internal/store/migrate_test.go:22-291`
- `internal/store/migrate_test.go:293-579`
- `internal/store/migration_plan.go:24-122`
- `internal/store/network_validation.go:125-145`
- `internal/store/sqlite_recovery_test.go:15-188`
- `internal/task/lease.go:174-187`
- `internal/task/manager_run_records.go:79-102`
- `internal/task/manager_run_records.go:292-313`
- `internal/task/manager_task_triage_cancel.go:110-140`
- `internal/vault/crypto.go:162-284`
- `internal/windowmanager/active_coalescer.go:14-114`
- `internal/windowmanager/service_test.go:1015-1026`
- `internal/workspace/helpers.go:131-142`
- `internal/workspace/resolver_test.go:2588-2628`
