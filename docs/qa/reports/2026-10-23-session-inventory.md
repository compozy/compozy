# Session inventory performance — issue #933

## Scope and method

The baseline is commit `e10354845e8893c93789e8251623ffaee78538d2`.
`BenchmarkManagerIdleInventory` uses 279 stopped sessions, a real temporary SQLite
catalog, and isolated metadata directories. One untimed call warms the inventory.
Counters wrap the existing persistence interfaces; each counted catalog read maps to
one SQLite query in this fixture. Metadata reads count file reads/parses, not kernel
syscalls. No production daemon or user data is involved.

The baseline was run through a Go `-overlay` containing the base versions of
`query.go`, `query_metadata.go`, and `query_workspace.go`. Its reaper inventory runs
that original `ListAll` and filters live states, matching the old sweep's population
cost. It does not benchmark stop execution or hooks. The after case uses `ListLive`.

```sh
go test ./internal/session/ -run '^$' -bench '^BenchmarkManagerIdleInventory$' -benchtime=300ms -count=1
```

Results on macOS arm64, Apple M4 Max, Go 1.27.1, under concurrent machine load:

| Warm operation | Before | After | Before / after |
| --- | ---: | ---: | ---: |
| `ListAll` wall time | 121.713 ms | 4.908 ms | 24.8x |
| `ListAll` catalog queries | 1,116 | 1 | 1,116x |
| `ListAll` metadata reads/parses | 279 | 0 | eliminated |
| `ListAll` allocated bytes | 11,406,669 | 2,589,442 | 4.4x |
| Reaper inventory wall time | 118.780 ms | 0.252 ms | 470.5x |
| Reaper inventory catalog queries | 1,116 | 3 | 372x |
| Reaper inventory metadata reads/parses | 279 | 0 | eliminated |
| Reaper inventory allocated bytes | 11,415,472 | 28,102 | 406.2x |

Separate one-second CPU profiles measured cumulative `Manager.ListAll` samples of
860 ms across 10 calls before, versus 850 ms across 251 calls after (including the
single warm-up call). This is approximately 86 versus 3.39 ms sampled CPU per call;
10 ms sampling and host contention limit precision. Profiled benchmark wall times
were 142.316 versus 5.036 ms/op. These are isolated benchmark results, not a claim
about the user's daemon CPU percentage or an exact kernel syscall count.

## Behavior and ownership

- List reads no longer classify crashes, write metadata/catalog state, create stop
  receipts, or restore receipt state as a side effect.
- The existing boot composition invokes `RecoverPendingStops`, which now explicitly
  reconciles inactive metadata before settling receipts. Explicit resume, repair,
  health recovery, and recovered-stop preparation retain their required recovery.
- The manager caches parsed metadata by file identity, mtime and size. Atomic
  replacement, modification, removal and corrupt replacement invalidate it. A full
  inventory evicts entries whose directories disappeared. Returned projections own
  their mutable fields; catalog owner proofs, transcript epochs and attention come
  from the current batch rather than the metadata cache. Workspace registrations
  are resolved once per distinct workspace per list; User-scope ownership retains
  its creation-witness checks. These additional reads are absent from the benchmark
  fixture and are not included in its catalog-query counts.
- Workspace lists read only their scoped catalog population. `ListAll` retains its
  internal all-profile inventory and ascending creation-time/ID ordering. Public
  page/cursor/filter contracts are unchanged.
- The reaper reads live catalog states plus the in-memory overlay, including retained
  live sessions after restart. Only missing parents needed by spawned sessions get
  individual status reads. A parent read failure never becomes an orphan decision.
- No schema, persisted format, public DTO, config key, hook, or native tool changes.
  The append-only event ledger remains authoritative. Existing SQL query builders
  supply the batch; no new handwritten static SQL or migration is required.

## Caller audit

The base tree contains 23 textual `.ListAll(` calls outside `*_test.go`: 15 session
production calls, five unrelated MCP allocation calls, and three API test stubs.

| Session caller | Population needed / disposition |
| --- | --- |
| `daemon/spawn_reaper.go` | Live sessions and referenced parents only; narrowed here. |
| `session/stop_recovered_boot.go`, `daemon/boot_session_repair.go` | Full retained inventory at boot, including stopped rows with receipts or repairable transcripts; retained. |
| `session/spawn.go` | Live children plus complete ancestry for governance proof, including stopped ancestors; retained batch. |
| `session/manager_delete_staging.go` | Complete retained workspace inventory for staged deletion/fencing; retained, avoiding concurrent teardown ownership changes. |
| `extension/host_api_sessions.go` | Historical sessions; optional workspace and archive filters. Retained existing public behavior. |
| `daemon/coordinator_runtime_session.go` | Healthy coordinator candidates; could use live inventory, but is an admission path, not the periodic reaper. Retained existing adapter contract. |
| `daemon/task_role_sessions.go` | Starting/active role candidates; retained existing role adapter contract. |
| `daemon/review_router_existing.go` | Active reviewer candidates; retained existing router adapter contract. |
| `daemon/review_router_selection.go` (two calls) | One original-worker/requester identity, including stopped history; a point-read adapter can narrow this independently. Retained existing exclusion semantics. |
| `daemon/native_tool_workspace_calls.go` | One workspace's historical sessions; retained existing workspace projection semantics. |
| `daemon/settings.go`, `api/core/status.go` | Exact total/state/badge population, including historical rows; retained. |
| `api/core/support_snapshots.go` | Full support inventory, including historical sessions; retained. |

## Verification and limits

Focused session tests cover cache reuse/invalidation, caller mutation, fresh catalog
epochs, scope/order, side-effect-free reads, explicit crash reconciliation, resume,
repair, health recovery, and verified stop receipt replay. The canonical daemon
reaper cases cover TTL, orphan, parent-stop, system workers and subagent exceptions.
`TestManagerIntegrationCrashRecoveryRejectsDeadRuntimeAttachment` passed with real
ACP subprocess and SQLite boundaries in an isolated temporary home.

The operator prohibited local broad gates, full builds/suites, race/coverage and E2E.
CI owns those delivery checks. Local package lint could not start: the installed
`golangci-lint` was built with Go 1.26, below the repository's Go 1.27.1 requirement.
The test-shape checker passes the benchmark and reaper suites; its eight findings
in unchanged older query tests predate this diff. The changed/new cases follow the
canonical subtest shape. No browser/provider QA verdict is promoted by these tests.
