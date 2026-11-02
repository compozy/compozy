# Events, settings, and supporting packages performance audit

Baseline: `de5ab2f4d`. Worktree: `perf/responsiveness-audit-20261101`.

## Test ownership before changes

- Slash-command parsing invariant: return the first admitted occurrence of each exact skill with browser-compatible UTF-16 offsets, regardless of skipped paths, unknown markers, duplicate markers, Unicode, or malformed UTF-8. Owning layer: pure command domain. Canonical suite: `internal/command/command_test.go`, `TestParseSkillInvocations`; benchmark belongs in the same suite and measures actual parser work, outside catalog construction.

- Toolset discovery invariant: every expansion in one list uses the same current provider snapshot; a later list observes provider changes, and empty catalogs do not probe providers. Owning layer: tool registry. Canonical suite: `internal/tools/registry_test.go`; benchmark measures full `RuntimeRegistry.ListToolsets` with dynamic descriptors.

- Restart observer invariant: active-session counts and restart requirements stay current independently, while the shared status cache retains the complete response for other consumers. Owning layer: Settings restart hook. Canonical suite: `web/src/systems/settings/hooks/__tests__/use-settings-restart.test.tsx`; existing notice tests own warning/failure presentation. A separate local Profiler harness measures irrelevant status updates without asserting render counts in the behavioral suite.

## Package audit

| Package | Inspected owners and conclusion |
| --- | --- |
| `internal/events` | `registry.go`, `registry_queries.go`, `registry_metadata.go`, registry entry files. This is immutable event metadata, not the fanout/replay implementation. Hot `Lookup` calls already use a read-only map; the small component/outcome scans are request filters. No extra goroutines, cache, or index justified. |
| `internal/settings` | `collections.go`, provider model/auth projections, MCP collection status/probe workers, session environment status, section loading, config apply state. MCP status probes already use four cancellation-aware workers and join them. Provider classification is declarative/binary resolution rather than an auth subprocess probe; no speculative worker pool added. Settings mutations preserve their ordered persistence/apply mutex. |
| `internal/config` | `config_load.go`, provider resolution, persistence transactions, profile overlay discovery, capability digest and MCP JSON readers. Fresh layered configuration is an authorization/identity boundary; no cross-request config cache added without a proven invalidation owner. Filesystem walks belong to explicit lifecycle/validation paths. |
| `internal/profile` | `catalog.go`, `selection.go`, lifecycle plans/journal and mutations. Profile list already batches ownership counts by table and credential requirements by profile set; lightweight identity/name reads avoid those counts. Lifecycle operations require ordered ownership checks and writes. No per-profile query loop introduced or retained in the list path. |
| `internal/vault` | `service.go`, secret metadata/resolve boundaries. Metadata avoids decryption; extension metadata already delegates ownership filtering to the store. No plaintext cache or concurrent key-provider calls introduced. Generic metadata list still uses store records; changing its wire/persistence owner requires a separate measured store result. |
| `internal/command` | `parser.go`, catalog construction and canonical parser tests. Found repeated full-prefix rune/UTF-16 conversion per emitted skill; benchmark and replacement are recorded below. No goroutine is suitable for an ordered, pure text scan. |
| `internal/cmdpalette` (including `corecmds`) | `catalog.go`, `registry.go`, context/descriptor projection, view sessions/events and core descriptors. Snapshot binding interfaces already batch reads. Provider ordering and error/degradation semantics are deterministic; no parallel provider dispatch added without provider thread-safety evidence. View lifetime/snapshot changes remain protected by their owning locks. |
| `internal/tools` (including `builtin`) | `registry.go`, `registry_index.go`, MCP reliability/retirement, native descriptor catalog, schema digest and policy projection. Found full provider discovery/index rebuild per toolset. A request-scoped shared index removes redundant I/O while retaining fresh snapshots between requests. Native descriptors already reuse validated immutable state; dynamic descriptors retain validation. |
| `internal/toolmeta` | `render.go`, `types.go`, preview helpers and existing redaction tests. Redaction precedes capping by contract. Full rune conversion in preview capping is a possible large-input follow-up, but not changed without a full Render measurement; truncating before redaction would be incorrect. |
| `internal/toolruntime` | `registry.go`, `registry_lifecycle.go`, `registry_observation.go`, interrupt/reconciliation and memory store. Observations copy under read lock and validate outside it; durable mutation ordering is explicitly serialized. No blanket parallel reconciliation or unlocked mutation introduced. |
| `internal/soul` | resolution/parsing, projection, managed authoring path and persistence checks. Pure parse/projection and explicit file authorization own these reads; authoring has compare-token and ownership dependencies. No background work or stale resolution cache justified. |
| `internal/mention` | `parser.go`, `code.go`, reconciliation. Candidate storage is capped and UTF-16 offsets already use a monotonic cursor. No duplicate optimization or goroutine added. |
| `internal/hooks` | `pool.go`, `dispatch_async.go`, `pipeline.go`, binding snapshots, deterministic ordering and existing dispatch benchmarks. Async work is already bounded (four workers, queue of 64), observes drops and drains with cancellation. Synchronous hooks intentionally compose in order; parallel execution would break patch semantics. Immutable binding construction already occurs outside the swap lock. |
| `internal/deadentity` | `service.go`, per-entity operation/state ownership, publication and workspace retirement. Independent entities have independent operation serialization and cancellation-aware waiting. Success skips durable clears when there is no dead mark. No global speculative cache or extra goroutines added. |

## Frontend systems audit

| System | Inspected owners and conclusion |
| --- | --- |
| `settings` | Query factories, all collection/section/page hooks, restart/updates, MCP authorization, runtime catalog and layout data. Independent layout reads already run concurrently; section mutations invalidate canonical keys; polling exists only with mounted observers. Restart consumers observed the full `/status` payload while reading two fields. A local Query selector now observes only active-session count and restart requirement, with the measured reduction below. The full cache envelope, polling interval, auth freshness, operation fallback and snooze semantics remain intact. |
| `profiles` | Selection/view stores, scope hook, query options and lifecycle event stream. The scope hook creates two remembered-selection observers, and stale-on-mount selection can refetch on successive mounts. This was reported to the root investigator; changing it requires preserving reload/external-switch/aggregate-destination semantics. Profile list/identity catalog is already shared and the icon catalog is build-stable. No speculative freshness change retained. |
| `vault` | Query factories, page flow/selection, list/inspection/create data. Cross-filter full inventory is already lazy, enabled only for actions needing collision/identity truth. Query namespaces preserve ownership and placeholder rows never cross namespaces. No extra full inventory fetch or plaintext state cache introduced. |
| `onboarding` | Status/directory query factories, wizard/workspace/default-model hooks, path field lifecycle. Queries are scoped and cancellable, directory reads gated, onboarding status has a 30-second stale window. This first-run flow does not multiply with active sessions; no defensible hot-path optimization found. |

## Measurements

| Workload | Median time before → after | Bytes/op before → after | Allocs/op before → after |
| --- | --- | --- | --- |
| `BenchmarkParseSkillInvocations/skills_1` | 0.0395 → 0.0138 ms | 68,240 → 144 | 5 → 1 |
| `BenchmarkParseSkillInvocations/skills_32` | 14.0494 → 0.4021 ms | 34,226,336 → 30,368 | 148 → 20 |
| `BenchmarkParseSkillInvocations/skills_128` | 217.0287 → 1.6986 ms | 527,536,032 → 132,512 | 542 → 30 |
| `BenchmarkRegistryListToolsets/toolsets_1` | 0.2760 → 0.3388 ms | 361,827 → 361,815 | 4,018 → 4,018 |
| `BenchmarkRegistryListToolsets/toolsets_16` | 4.2777 → 0.8382 ms | 5,789,959 → 732,565 | 64,268 → 4,319 |
| `BenchmarkRegistryListToolsets/toolsets_32` | 8.8528 → 2.1617 ms | 11,578,960 → 1,128,208 | 128,534 → 4,640 |

Both commands use the task serializer with `GOMAXPROCS=2`, `GOFLAGS=-p=2`, five repetitions, and a 200 ms per-benchmark target. The slash workload inserts 512 copies of `context 🚀 ` before each distinct skill; the largest prompt is approximately 0.85 MB. The toolset workload uses 128 dynamic descriptors and varies toolset count; the benchmark does not synthesize network delay. The production change reduces provider discovery from one full pass per toolset to one pass per request, without caching across requests.

Commands:

```sh
python3 .cache/performance/run-bounded.py go test ./internal/command -run '^TestParseSkillInvocations$' -bench '^BenchmarkParseSkillInvocations$' -benchtime=200ms -count=5
python3 .cache/performance/run-bounded.py go test ./internal/tools -run '^$' -bench '^BenchmarkRegistryListToolsets$' -benchtime=200ms -count=5
```

The combined after command targets the same benchmarks in both packages. Raw local receipts: `.cache/performance/events-settings/{command-before,toolsets-before,after}.log`. The initial command benchmark fixture omitted its required source kind and failed before executing; the fixture was corrected before the recorded baseline.

Timing was noisy under shared-host load. In particular, the one-toolset control showed 0.276 → 0.339 ms while allocations were unchanged; the changed source performs the same one index build for that case. The 32-toolset after range was 1.436–6.657 ms, versus 8.349–9.154 ms before. The multi-toolset elapsed-time improvement and the approximately 90% byte / 96% allocation reduction clearly exceed that noise. No universal endpoint/browser latency or field improvement is inferred from these microbenchmarks.

### Settings restart observer

A React Profiler around the real `useSettingsRestart` hook received 100 separate
status-cache updates that changed only `generated_at` and health uptime. The
baseline committed 100 times; the narrowed observer committed zero times. One
subsequent relevant update changed the active-session count to 120 and the
restart requirement to true; both versions committed once and exposed the new
values. Both diagnostic runs used Bun 1.4.0, the same QueryClient/fixture, JSDOM,
and a one-millisecond notification settle per update. This measures eliminated
observer work, not browser interaction latency; the harness elapsed times are
not used for speed claims.

Harness and local receipts:
`.cache/performance/events-settings/benchmark-settings-status.tsx`,
`settings-before.log`, and `settings-after.log` in the same directory. Command:

```sh
python3 .cache/performance/run-bounded.py bun .cache/performance/events-settings/benchmark-settings-status.tsx
```

## Verification

- Passed: `CGO_ENABLED=1 go test -race ./internal/command ./internal/tools -run '^(TestParseSkillInvocations|TestRuntimeRegistry)' -count=1 -parallel=2`, through the task serializer. Canonical parser and registry ownership/projection tests ran with the race detector.
- Passed: test-shape heuristic on `internal/command/command_test.go`.
- Existing baseline limitation: the heuristic flags `TestRuntimeRegistryResolverRevalidatesProjectionAndDispatch` for assertions outside `t.Run`. The identical finding is present on the unmodified `de5ab2f4d` file, checked from `.cache/performance/events-settings/registry_baseline_test.go`; the new toolset cases use named parallel subtests. No unrelated test was rewritten.
- Passed: the focused frontend hook/notice suites through root Turbo, 2 files / 14 tests. Command: `python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/settings/hooks/__tests__/use-settings-restart.test.tsx src/systems/settings/components/__tests__/settings-restart-notice.test.tsx --maxWorkers=1`. Receipt: `.cache/performance/events-settings/settings-tests.log`. The two changed frontend files were formatted with `oxfmt` through the same serializer.
- Passed: scoped `git diff --check` after formatting.
- Production source sizes remain under 500 lines. No schema, DTO, config, native tool ID, extension/hook protocol, or official skill change is needed.
- Root delivery owns scoped lint/typecheck, full current-head CI, and real application/transport verification. Relevant existing surfaces: `ET-session-slash-commands-inline`, `ET-compozy-native-tool-invocation`, `MS-web-settings-takeover-redesign`, and `internal/api/core/tools_test.go` toolset routes. Dated QA-impact notes were appended to all three scenarios without changing historical verdicts. These local unit/benchmark receipts do not claim those broader runs.
