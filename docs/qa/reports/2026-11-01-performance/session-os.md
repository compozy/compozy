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

## Dashboard connection CI follow-up (2026-11-02)

Invariant: Home retains its loaded overview while its daemon health requests fail, shows a
non-connected indicator within the existing 15-second limit, recovers when requests resume,
and reloads current overview truth after a real daemon restart. The owning layer and canonical
suite are the daemon-served Home browser flow in `web/e2e/__tests__/dashboard.spec.ts`, test
`Home preserves its overview through health loss and recovers after a daemon restart`.

CI run `41383826440`, job `125080272757`, failed this case on `14730a705771c0a3eb6e92947eddede632dff666`.
The unchanged test reproduced locally with zero retries: it remained `connected` for all 34
polls and failed the 15-second assertion (17.2 seconds total). Its network artifact contains
two failed `/api/status` requests while `/api/status/identity` remained successful. The route
interceptor only blocked the full diagnostic endpoint; Home now observes the bounded identity
endpoint. This was an outdated outage injection, not evidence that identity-query errors were
ignored in production. The correction covers both public status endpoints with the same route
handler and removes that handler for recovery. All product assertions, timeouts and the real
restart step remain unchanged. No production code changes are part of this follow-up.

The focused reproduction uses `.cache/performance/run-browser.py`, which acquires the existing
machine-wide browser lock before invoking `run-bounded.py`. It reuses the published-candidate
daemon `.cache/performance/compozy-candidate-final` and the current built `web/dist` authorized
for this diagnosis; the existing fixture provisions a fresh isolated home/workspace/port and
cleans up its daemon, replacement daemon and temporary state. Negative receipt:
`.cache/performance/session-os/dashboard-before.txt`; network/trace/screenshot artifacts are
under `.cache/performance/session-os/dashboard-before/`.

The corrected scenario passes **1/1**, with zero retries (25.9 seconds for the scenario,
31.4 seconds total). The after-run network artifact records two failed identity reads and one
failed full-status read, followed by successful health reads. The same browser assertions prove
the indicator leaves `connected`, the loaded body and four metrics remain visible, connection
recovers, and the real restart returns `ready` with a newly generated overview. The fixture's
teardown completes without errors. Receipt: `.cache/performance/session-os/dashboard-after.txt`;
network/trace/screenshot artifacts: `.cache/performance/session-os/dashboard-after/`.

Both runs use daemon SHA-256
`d700cc3f5d0424ba53c2bde275f6e9a76bd947955ccf85c95c4ce85ba307d146` and built Web index SHA-256
`e115cd296c6ba119dcd2f90b98ebfc0e403df3c6ec02fffa969ae092db18922d`. The explicit launch command is:

```sh
python3 .cache/performance/run-browser.py env \
  COMPOZY_E2E_BASE_URL= COMPOZY_E2E_WEB_SHARD= \
  COMPOZY_TEST_DAEMON_BIN="$PWD/.cache/performance/compozy-candidate-final" \
  COMPOZY_WEB_DIST_DIR="$PWD/web/dist" \
  node web/node_modules/@playwright/test/cli.js test \
  --config web/playwright.config.ts web/e2e/__tests__/dashboard.spec.ts \
  --grep 'Home preserves its overview through health loss' --retries=0 \
  --output .cache/performance/session-os/dashboard-after
```

The reproduction used the same command with `dashboard-before` as the output directory before
changing the interceptor. This is focused real-daemon/browser evidence for connection recovery
and overview retention, not full-shard or current-head CI success. Root owns the subsequent
integrated lint/typecheck/build and publication. Scenario `RT-039` records this bounded re-walk.

## Main integration audit: `8c025ff8f` (2026-11-02)

The incoming baseline is `8c025ff8fdd3a9a79e4f0e63f54b3bd144a4773b`. The earlier measurements
above remain historical; they do not measure this Stays-on/identity contract. This review owns
201 incoming paths: ACP (23), session (111), nested contextusage (2), transcript (5), and OS (60).
The eight overlapping paths are session `catalog_page.go`, `manager.go`, `manager_test.go`,
`perf_bench_test.go`, `query_test.go`, `session_info.go`, OS dashboard's window test, and
`session-window-view.tsx`. Generated contracts and wider API/store/agent authorities belong to
the other integration owners; this is not a claim of line-by-line review of all incoming tests.

The query conflict adds independent cases to `TestManagerListPageOverlaysActiveAndBindsCursor`.
Both additions are retained: the new active/durable instance-main exclusion case and the two
performance cases rejecting stale durable rows/relations after a live workspace/profile change.
Only conflict separators were removed and our registration argument was updated to the incoming
`*store.SessionInfo` API; assertions were not weakened.

Validation invariants and owning suites before execution:

- Session snapshots retain isolated capabilities and the new instance/initiator identity; the
  existing session info, manager activity and catalog query suites own snapshot/scope behavior.
- The manager catalog excludes triage and instance mains before totals and keeps the correct
  live/durable relation overlay; the existing query and permission suites own these rules.
- ACP read-only triage and permission expiry keep their pending-request ownership; the existing
  ACP start/permission/handler suites own this boundary. Context usage maintenance and report
  projection remain in their existing package suites.
- OS keeps urgent session navigation, stale-content guards, shared connection identity, About
  gating and live-window eligibility while preserving incoming instance/history UI; existing
  OS session, dashboard, About, interaction and incoming agent-location suites own these flows.

Re-measurement uses a fresh external `git archive` of the incoming commit at
`/private/tmp/compozy-performance-main8c-session-n5hikopt/source`, with the same benchmark source supplied
to both revisions via Go overlays. The 128-option capability, 1,024-session/32-workspace catalog,
and 256-session process-demand fixtures retain their assertions. A fixture-only optional
interface selects the available production demand path: the old full `List` reduction or the
new scalar `ProcessActivity`. Both sides use that identical fixture and the shared two-CPU
bounded runner. No production file in the archived baseline is edited. This isolates the
incremental optimization from upstream changes; it does not measure API instance-facet
enumeration/hydration newly added by Stays-on.

### Delta dispositions

| Area | Incoming paths | Review and integration disposition |
| --- | ---: | --- |
| `internal/acp` | 23: 19 production, 4 tests | Inspected execution-policy propagation, read-only triage negotiation and tool/file/terminal guards, mode-change prompt fencing, permission timeout resolution and first-winner ownership, event expiry, and parent-cancellation diagnostics. `CapsSnapshot` still returns an isolated capability graph; the session clone reduction does not remove that boundary. No production overlap or repair. |
| `internal/session` | 111: 75 production, 36 tests | Reviewed catalog filters/facets/cursor population, authoritative instance metadata, pointer-based create/register contracts, initiator propagation, instance ceiling/launch checks, main-session lifecycle guards/rotation, report dispatch/recovery, triage cleanup, permission activity and soul profile ownership. The six overlapping session files retain the performance paths and incoming contracts. Test-only pointer migrations map to their existing lifecycle/query/permission suites; they are not claimed as independent line-by-line test reviews. |
| `internal/session/contextusage` | 2: 1 production, 1 test | Reviewed the new maintenance decision boundary for unknown usage, invalid size/threshold and compact-versus-rotate selection. The owning decision suite passes; no optimization or integration repair. |
| `internal/transcript` | 5: 4 production, 1 test | Reviewed report classification, assigned identity, redacted data-part projection and preservation of an in-flight assistant boundary. `TestReportProjection` passes with the race detector. Reports remain outside the assistant fold cache; the store owner separately checked that dispatch boundary. No repair. |
| `web/src/systems/os` | 60: 39 production, 17 tests, 4 stories; six removals included | Reviewed agent team/profile/editor routing and legacy destinations, window-scoped selection, instance history/report composition, activity navigation, create-screen wiring, identity labels/notifications, dock ordering and tab-view extraction. Removed detail/settings files and tests are mapped to the incoming agent profile/editor authority. Stories follow those component authorities; no screenshot-equivalence claim. The overlapping session view keeps the incoming superseded/history branch and the urgent-identity boundary. The overlapping dashboard test retains connection gating and incoming Activity props. |

The scope filter still runs under each session's read lock, and excluded live IDs still suppress
durable rows and relationships. Incoming triage filtering and `ExcludeInstanceMain` remain in
the final population rules. `InstanceID`, `InitiatorKind`, and `InitiatorID` remain in `Info`;
catalog projection still applies authoritative instance metadata. The new held-permission work
signal is retained in manager initialization, while process sampling continues to read only
liveness/activity scalars. No new goroutine, cache or lifecycle owner was introduced here.

OS's two-owner live-data budget, closed-About gating and lightweight connection identity are
unchanged. Urgent route identity and the stale-session archive guard survive alongside the
incoming instance-history surface. New agent-team/instance queries and identity rendering are
upstream workload changes; the earlier selector/browser measurements do not establish their
overall cost. API instance-facet enumeration and per-row hydration remain outside the internal
catalog fixture, as confirmed by the API integration owner.

The first OS run passed all tests but exposed a fixture mismatch: the incoming Activity
navigation calls `useNavigate`, while the old dashboard unit harness lacked a router. The
existing dashboard suite now renders a real memory `RouterProvider`, matching the nearby
Settings suite. Its original eight assertions remain unchanged and pass without the prior
router-context warnings. Integrated Go lint then found `infoLocked` had grown to 82 lines,
exceeding its 80-line limit after the new identity fields. The capability selection/copy block
is now the small `capabilitiesSnapshotLocked` helper. The same session read lock covers both
calls; live capabilities still use the process-owned snapshot and stored capabilities still
receive one defensive clone. All `Info` fields remain unchanged. These two fixture repairs
and this extraction are the only integration repairs in this ownership.

### Current comparison against incoming main

Five samples per case, `-benchtime=200ms`, same Darwin/arm64 host and bounded two-CPU controls.
Numbers are sample medians, not application latency or a whole-Stays-on performance claim.

| Fixture | Main `8c` ns/op | Merged ns/op | Main B/op | Merged B/op | Main allocations | Merged allocations |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Live capability snapshot, 128 model options | 3,924 | 1,469 | 34,512 | 12,400 | 12 | 6 |
| Active catalog, one workspace out of 32 / 1,024 sessions | 4,982,295 | 130,352 | 37,073,752 | 593,608 | 13,391 | 314 |
| Active catalog, all 1,024 sessions | 5,150,044 | 2,281,060 | 37,645,570 | 15,002,880 | 15,377 | 9,233 |
| Process sampling demand, 256 idle active sessions | 1,059,006 | 3,032 | 8,839,680 | 2,304 | 3,074 | 1 |

The process-demand pair compares the complete old `List` path to the merged scalar projection;
it includes the effect of the retained capability optimization. It is intentionally distinct
from the historical incremental demand measurement above, whose baseline already had that
capability change. New incoming fields and other upstream changes are present on both sides
of this current pair.

Identical benchmark-source SHA-256:
`f0bd9f72802241b51036089ff7cda4b19ff6ea2a7020708c0e2099b25cef4caa`.
Raw receipts: `.cache/performance/session-os/main8c-bench-before-canonical.txt` and
`main8c-bench-after-refactor.txt`; all 20 samples per revision and derived medians are retained in
`main8c-bench-final-summary.json`. The earlier merged run remains in `main8c-bench-after.txt`
and `main8c-bench-summary.json`; all four final allocation/byte medians are unchanged by the
helper extraction. Source/overlay paths are in `main8c-baseline.json`. Both commands
use the bounded runner and this shared selection:

```sh
go test [-C <baseline-source>] -overlay <revision-overlay.json> ./internal/session \
  -run '^$' -bench 'Benchmark(SessionInfoLiveCapabilities|ManagerActiveCatalogScope|ManagerProcessDemand)$' \
  -benchmem -benchtime=200ms -count=5
```

The first baseline attempt used macOS's `/tmp` alias in the overlay, so Go did not discover the
added benchmarks. That PASS-only receipt (`main8c-bench-before.txt`) contains no measurements
and is excluded. The accepted baseline uses canonical `/private/tmp` paths and discovers every
named benchmark. No benchmark population, assertion or budget was changed to obtain results.

### Focused integration verification

- Race-enabled Go selection passes in session (16.470 s), ACP (4.733 s), contextusage (1.463 s)
  and transcript (1.435 s). `main8c-go-focused.txt` covers the conflict's three cases, capability
  isolation, demand transitions, workspace/public filtering, instance permissions, triage policy
  and startup, permission expiry/first winner, report prompt delivery, superseded prompt refusal,
  maintenance decisions and report projection. Real SQLite and existing deterministic process
  fixtures are used where the owning suites require them.
- Root Turbo focused OS validation passes **278/278 tests in 13 suites** (30.46 s), including
  session identity/visibility/history, content/controller composition, About, live-data owner
  selection, dashboard connection gating, agent routes/topbar, attention, deck, automation and
  registry. Receipt: `main8c-os-focused.txt`. After adding the real dashboard router fixture, the
  changed suite passes again **8/8** (3.47 s) without router warnings:
  `main8c-dashboard-final.txt`. Unchanged suites were not repeated.
- A source comparison confirms both conflict-addition blocks survive (with only the required
  pointer adaptation) and the prior capability/activity assertion functions are unchanged.
- After the helper extraction, `TestSessionInfoCopiesCapabilities` passes again with race
  detection (2.914 s), and the four existing benchmark cases pass with five samples each
  (5.354 s). Receipts: `main8c-info-refactor-test.txt` and `main8c-bench-after-refactor.txt`.
- Bounded `make go-lint`, scoped to `./internal/session/...` with formatting restricted to
  `session_info.go`, passes source policy and both normal/integration build-tag lanes with
  zero issues (`main8c-info-refactor-lint.txt`). The earlier dashboard Oxfmt/Oxlint checks and
  query gofmt check pass (`main8c-final-style.txt`). The standalone test-shape heuristic reports
  the same nine pre-existing inline-case diagnostics in both the incoming and merged query
  suite; it is not presented as a clean heuristic run or repaired outside this scope.

Final owned-source freeze at `2026-11-02T08:31:09Z`:
`main8c-owner-freeze.json` records 211 paths (the 201 incoming paths plus ten prior owned
changes). Its sorted path/hash-map SHA-256 is
`4c44c9cf214ce9429d4aee0fd551574a2294cb96650084451a467afa55b0fa31`.
The final `session_info.go` SHA-256 is
`6b080e669d67154e5d8426604019222682817faafdedce254dc3140a2b1d131f`.
Only that file differs from the preserved pre-extraction owner freeze
(`main8c-owner-before-info-extraction.json`). No source edits followed the final checks.

These are focused merge-integration checks on the worktree above `e62862f48362b9cc6da311df0078b0be914a5734`,
not a new full-suite, platform-parity, browser-latency or current-head CI verdict. Root owns
the merged runtime/browser run, generated-contract checks, integrated delivery gates and commit.
Root reports its integrated browser walk passed 64/64 selections and eight streams with 600
chunks and no errors on the preceding source snapshot recorded in
`.cache/performance/main-integration-source-freeze.json`. That walk was not repeated after
this behavior-preserving extraction; the final focused Info/benchmark checks above cover the
changed function, and final-head CI remains the enclosing delivery authority.
