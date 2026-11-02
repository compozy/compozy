# Session runtime and OS performance audit

Baseline: `de5ab2f4d`. Checkout: `perf/responsiveness-audit-20261101`. Measurements use synthetic local fixtures, not production state. Raw receipts are under `.cache/performance/session-os/`.

## Measurement and test ownership

The invariants and suites below were selected before changing their tests.

| Invariant | Owning layer and canonical suite |
| --- | --- |
| Live rows replace durable rows before scope filtering, sorting, counting and pagination, including active identity that differs from persisted workspace/profile. Out-of-scope children cannot leak through durable relation counts. | Session catalog: `internal/session/query_test.go`, `TestManagerListPageOverlaysActiveAndBindsCursor`; benchmark in existing `perf_bench_test.go`. |
| Returned capabilities are isolated from the live process and future snapshots; live capabilities override retained capabilities. | Session runtime: `session_test.go`, `TestSessionInfoCopiesCapabilities`. |
| Starting, active and stopping sessions request process sampling; only non-stopped current tool activity requests the fast cadence. Changes are observed on the next read. | Session manager: `manager_test.go`, `TestManagerProcessActivity`. |
| Focused/recent eligible windows own the two-window live budget. Fallback uses descending layer then ascending ID, excluding minimized, inactive-stack and other-desktop windows. | OS hook: `hooks/__tests__/os-interaction-hooks.test.tsx`. |
| Dynamic secrets remain redacted until every registration is released, with idempotent cleanup. | Existing `redact_test.go`, `TestStringUsesDynamicSecretsAndRemainsIdempotent`; no duplicate correctness suite added. |
| Closed About initiates no full status request or poll; opening still shows daemon/config detail and closing stops polling. Home's connection gate remains truthful. | Real Query hooks and HTTP adapter stub: `components/__tests__/os-about-dialog.test.tsx`; existing `apps/dashboard/__tests__/dashboard-window.test.tsx`. |

## Measurements

Go: darwin/arm64, Apple M4 Max, `GOMAXPROCS=2`, `GOFLAGS=-p=2`, five samples at `-benchtime=200ms`. Timings are medians. Validation is serialized, but other host activity remains outside the task lock. These are fixture costs, not measured end-user latency.

| Workload | Before | After | Interpretation |
| --- | --- | --- | --- |
| Live session snapshot, 128 model options | 5,977 ns; 34,512 B; 12 allocs | 2,234 ns; 12,400 B; 6 allocs | Avoids retained capabilities that will be replaced and another copy of an already-owned process snapshot; 64.1% fewer bytes. |
| One workspace, 1,024 active sessions / 32 workspaces / 128 model options | 7,309,303 ns; 37,024,600 B; 13,391 allocs | 222,926 ns; 591,560 B; 314 allocs | Snapshots only 32 scoped sessions, retaining all 1,024 durable exclusion IDs; 98.4% fewer bytes. |
| All workspaces, same population | 37,596,416 B; 15,377 allocs | 14,953,728 B; 9,233 allocs | Capability-copy saving remains. Final timing samples varied from 6.1 to 22.4 ms, so no all-workspace timing claim is made. |
| Process demand, 256 idle active sessions / 128 model options | 901,974 ns; 3,179,008 B; 1,538 allocs | 4,986 ns; 2,304 B; 1 alloc | Baseline already includes the capability fix. New demand read visits state/current-tool scalars without hydrating capabilities; 99.93% fewer allocated bytes. |
| Register/release another reference to one of 1,024 registered secrets | 209,715 ns; 36,912 B; 4 allocs | 50.69 ns; 0 B; 0 allocs | Refcount-only updates avoid rebuilding an unchanged immutable secret set. |

Receipts: `baseline.txt`, `caps-after.txt`, `scoped-after.txt`, `process-before.txt`, `session-final.txt`, `redact-before.txt`, `redact-after.txt`. Capability-only intermediate scoped result: 14,381,912 B/op; scope-first projection reduced it further to 591,560 B/op.

The OS diagnostic uses the actual source `liveDataOwners` function, Bun TypeScript transpilation, fixed state fixtures, 100 warmups and seven samples of 1,000 selections. Both sides use explicit host Bun 1.4.4. Before source is frozen from `de5ab2f4d`; output owner IDs match.

| Fallback population | Before median | After median |
| --- | --- | --- |
| 64 windows | 3.8765 microseconds | 0.851125 microseconds |
| 256 windows | 20.660083 microseconds | 5.375791 microseconds |

This measures selection plus an output check, not React commits/browser interaction. The deterministic fixture uses `w0..wN`, focus `w1`, only `w1` in focus order, layer `(i*17)%31`, every fourth window on another desktop, every fifth minimized and every seventh inactive in its stack. Receipts: `bench-live-data.ts`, `window-live-data-before.ts`, `os-before-stable.txt`, `os-after.txt`. Delivery checks use repository-pinned Bun 1.4.0 independently of the diagnostic.

## Package-by-package audit

### internal/session

Inspected manager/lifecycle/start/stop, `session_info.go`, capability adapters, `catalog_page*.go`, `catalog_board.go`, `catalog_relations.go`, facets, list/query hydration, context reads, prompt/recorder paths and stream ownership. Three changes were retained:

1. `Session.Info` retains the ownership copy required by `AgentProcess.CapsSnapshot`, instead of redundantly cloning retained and live capabilities. Custom-driver boundaries still copy their supplied snapshot. No shared mutable capability slices escape.
2. Catalog reads copy session pointers under the manager lock, release it, then read scope/public-role identity under each session lock before constructing a full snapshot. The same lock protects selected identity and snapshot. Excluded live IDs remain in the durable exclusion set and relation map, preventing stale persisted identities from reappearing. Existing filters, counts, PR projection and stable final page sorting remain authoritative. No new cache.
3. `ProcessActivity` provides scalar sampling demand to the daemon bridge, releasing the manager lock before session reads and returning early for tool activity. The daemon/sessionprocess owner preserves the 250 ms demand check and defers full owner hydration to actual scans.

`ListAll` retains filesystem inventory/durable reconciliation. A proposed separate status aggregate was discarded with the API owner: it would duplicate hydration semantics after consumer polling moved to the existing identity endpoint. Full diagnostics remain available.

Concurrency: no goroutines added. Per-run handles own cancellation and joining. Parallel full snapshots would increase allocation/lock pressure; removal gives the measured gain. Recorder finalization calls `Close` and `setRecorder(nil)`, so stopped sessions do not retain the normal closed recorder. Store ownership separately covers transcript-fold retention in live database handles.

### internal/session/contextusage

Inspected `derive.go`, `attribution.go`, `turns.go` and caller paths. Ordering/attribution occur per context read, preserving late-confirmation/sent-time semantics. Attribution has a possible rows-by-drop scan on pathological histories, but no material product workload was established for a change. No speculative cache or rewrite retained.

### internal/session/inputqueue

Inspected `queue.go`, `mutation.go`, `batch.go`, `synthetic*.go`, `steer.go`, `mentions.go` and `clear.go`. Durable atomic mutation/claim owns ordering/admission. No parallel writer or duplicate queue state added; no actionable measured bottleneck.

### web/src/systems/os

Inspected shell/desktop selectors, window-manager store/projection, app loading/preloading, palette subscriptions, attention/session visibility, live-data budget, Home and About. Existing scoped selectors, closed-palette unsubscribe and two-window budget were retained.

Fallback selection previously sorted every window. At most two scans now select eligible owners with the same focus order and layer/ID tie rules, avoiding sorted-entry allocation without memoization or derived state. Final integrated lint requested explicit first/second owner comparisons inside the scan; the fixed two-owner budget makes these checks equivalent without allocating a Set or changing the measured selection algorithm.

Home uses the public connection-status hook backed by `/api/status/identity` in the status owner's change. About enables its full status observers only while open. The real QueryClient baseline (`about-before-only.txt`) observed **four full status requests during 30 seconds with About closed**; the new zero-request assertion failed exactly on that count, while both rendering cases passed. After the change (`os-validation.txt`), it makes **zero requests while closed for 30 seconds**, one on opening, a second after ten open seconds, and no further request during 30 seconds after closing. This uses fake time and the HTTP adapter boundary; it is not a browser network/latency measurement. Pending/resolved identity rows and Home's connection gate pass unchanged.

### internal/acp

Inspected process capabilities/config state, ingestion, broadcasting, prompt/fork lifetimes, child tasks and routing. Ingestion already bounds work at 1,024 events / 8 MiB and coalesces text within 4,096-byte chunks. Live broadcast defaults to 64 buffered events per subscriber with explicit degradation/replay. Child tasks are tracked/joined; prompt/fork maps have run-scoped lifetimes. ACP/custom-driver capability boundaries require ownership copies. No measured additional bottleneck justified changes or more goroutines.

### internal/agentidentity

Inspected `identity.go`, `snapshot.go`, `credentials.go` and errors. Resolution validates caller/session/workspace identity using authoritative lookup and copies lineage. Caching authorization reads would risk freshness without demonstrated savings. No change.

### internal/admission

Inspected atomic gate/work classification and platform load monitor. Sampling defaults to five seconds with one joined monitor, bounded waits and cancellation. More samplers/per-request goroutines would duplicate work. No actionable measured bottleneck; no change.

### internal/subagentid

Inspected `id.go`: deterministic hashing of bounded identity inputs; no retained population or I/O. No change.

### internal/runtimeoption

Inspected `selection.go`: validates, copies, deduplicates and sorts the small configured option list, not the session population. Ownership and deterministic validation remain necessary. No measured win supported a rewrite.

### internal/speed

Inspected `types.go`: typed parsing and small optional resolution copies; no population scan, background work or growing cache. No change.

### internal/reasoning

Inspected `effort.go`: normalized single-value validation with one Unicode scan and no retained state. No change.

### internal/transcript

Inspected incremental projector/fold, pruning, navigation, codecs and UI projection, including `sessiondb/session_event_write.go` lifetime. Projector maps belong to one write/batch transaction, not an indefinitely growing daemon cache. `EntryFold` avoids replaying every event; reconstruction/pruning copies enforce ownership/output boundaries. Fold retention inside SessionDB belongs to the store audit. No package change/new goroutine.

### internal/redact

Inspected dynamic registry and exact/JSON/log paths. Registering another reference and releasing a nonfinal reference rebuilt the same sorted snapshot under the mutex. Publication now occurs only when membership changes. Atomic readers, longest-first matching, required short secrets and cleanup semantics remain; canonical refcount/idempotence cases pass with race detection. Masking rules and output secrecy are unchanged.

### internal/retention

Inspected `periodic.go`: one cancellable worker invokes sweeps sequentially and joins on shutdown. Concurrent sweeps would increase SQLite/filesystem contention and overlap cleanup. No measured bottleneck justified another worker or change.

## Validation and integration boundaries

All commands use `python3 .cache/performance/run-bounded.py`. Focused commands:

```sh
go test ./internal/session -run '^$' -bench 'Benchmark(SessionInfoLiveCapabilities|ManagerActiveCatalogScope)' -benchmem -benchtime=200ms -count=5
CGO_ENABLED=1 go test -race ./internal/session -run 'Test(SessionInfoCopiesCapabilities|ManagerProcessActivity|ManagerListPageOverlaysActiveAndBindsCursor|SessionMatchesListQuery|NormalizeListQuery)$' -count=1 -parallel=2
go test ./internal/session -run '^$' -bench '^BenchmarkManagerProcessDemand$' -benchmem -benchtime=200ms -count=5
go test -race ./internal/redact -run '^TestStringUsesDynamicSecretsAndRemainsIdempotent$' -count=1
go test ./internal/redact -run '^$' -bench '^BenchmarkDynamicSecretReferences$' -benchmem -benchtime=200ms -count=5
bunx turbo run test --only --filter=./web -- --maxWorkers=1 src/systems/os/components/__tests__/os-about-dialog.test.tsx src/systems/os/hooks/__tests__/os-interaction-hooks.test.tsx src/systems/os/apps/dashboard/__tests__/dashboard-window.test.tsx
```

The final focused session race run passes in 8.522 seconds, including real SQLite catalog/relations assertions, capability ownership and process-demand transitions (`session-final.txt`). Redaction's canonical dynamic-secret suite passes with race detection (`redact-after.txt`). The final Turbo `--only` run uses verified Bun 1.4.0 and passes **79/79 tests across three OS suites**, including polling, window eligibility and Home connection gating (`os-validation.txt`). Scoped oxfmt and `git diff --check` also pass.

After the final explicit two-owner comparison adjustment, scoped Oxfmt and Oxlint with `--deny-warnings` pass, and the owning interaction suite passes again **68/68** through Turbo (`os-final-lint.txt`). No further source changes followed that check.

Failed intermediate receipts are preserved separately: the new catalog fixture initially omitted required lineage Kind and runtime status, failing SQLite validation before its assertion; both fields were corrected without weakening assertions. The next session run encountered a concurrent store/codegen intermediate state (`ListTaskEventsForTasks` not yet generated), before tests ran, and was repeated successfully after generation. An initial Turbo invocation without `--only` automatically ran codegen-check and stopped before About tests because the frozen baseline directory appeared as an out-of-package sqlcgen importer; root moved frozen sources outside the checkout behind a symlink. These attempts are not successful validation evidence.

The test-convention checker was run once per changed Go test file. It reported 20 pre-existing inline-case warnings in manager/session/query suites, none in the new cases; benchmark and redaction files pass. Existing unrelated test shapes were preserved. Raw output: `test-conventions.txt`.

No wire/schema/config/native-tool contract changes originate here. Native session readers retain ordering, scope and capability behavior; no migration is needed. Isolation is covered by active/durable overlay tests. Official skills/public docs need no operation/shape changes. Root owns consolidated impact, generators, QA scenario updates, real browser/daemon integration, delivery gates and current-head CI.

Affected journeys: `ET-web-catalog-navigation`, `ET-web-shell-shortcuts-about-dialogs`, `E2E-023` shell/session responsiveness. This worker does not claim browser latency, retained-heap improvement, production behavior, full-suite success or PR delivery from focused measurements.
