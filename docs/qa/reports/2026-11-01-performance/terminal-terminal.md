# Terminal performance audit

Baseline: `de5ab2f4d`. Scope: `internal/terminal/**`, `web/src/systems/terminal/**`, `internal/{procutil,subprocess,managedsidecar,shellwait,outboundpolicy,attachments,filesnap,fileutil}` and `web/src/systems/tool-approvals/**`.

## Measurement and coverage plan

- Invariant: bounded terminal retention preserves replay bytes, absolute sequence offsets, typed redacted-input markers and escape/UTF-8-safe trim boundaries, while old snapshots stay immutable after further writes.
- Owning layer: terminal replay ring.
- Canonical suite: `internal/terminal/ring_test.go`, `TestRingReplayContract`; add the steady-state append benchmark to this same file.
- Suspected cost: `Ring.appendSegmentLocked` concatenates a growing retained string for each write; `trim` copies it again once capacity is reached. Measure at the default 1 MiB retention with small echo and coalesced output sizes.
- Frontend audit is separate. Existing stream leases and bounded render/gap queues are retained; no distinct measured bottleneck justified changing them.

## Status

Five bounded changes are implemented. Focused Go race, real-filesystem attachment suites and frontend behavioral suites pass. No end-to-end user latency is inferred from these local measurements; root owns real runtime/scenario validation and current-head CI.

## Attachment measurement plan

- Invariant: publishing an attachment still enforces age/count/byte retention in deterministic oldest-first order, preserves pending-input pins, detects corrupt sidecars/content, and serializes against deletion leases.
- Owning layer: filesystem attachment retention.
- Canonical suite: `internal/attachments/store_fs_test.go`, `TestFilesystemAttachmentStoreRetention` and corruption/round-trip cases. The real-filesystem benchmark belongs in that same suite file.
- Measure fresh Put at 128 and 512 retained objects. Read-path sweeps remain authoritative; only redundant traversal inside one mutation lock may be removed.
- Read-path follow-up invariant: Open returns an owned reader positioned at byte zero whose content digest and MIME/dimensions match the retained metadata, with the same scope/retention checks and cancellation cleanup. The owning suite remains `TestFilesystemAttachmentStoreRoundTrip` / `MetadataIntegrity`; baseline `BenchmarkFilesystemAttachmentStoreRead` measures Stat and Open of 64 KiB and 4 MiB real PDF fixtures before any read-path edit.

## Frontend terminal state plan

- Invariant: the current catalog owns terminal pane lifetime; state for deleted/expired terminals is released while still-present panes retain state and old-scope events cannot change the current store.
- Owning layer: terminal store plus catalog cleanup hook.
- Canonical suites: `stores/__tests__/terminal-store.test.ts` for transition and `components/__tests__/terminal-window-app.test.tsx` for existing window/attachment integration.
- Baseline measurement fixture: 1,000 terminal identities rotated through one scope, followed by 1,000 presence updates; report pane count, serialized state bytes (not heap) and update CPU. Original source is captured before editing for paired same-toolchain comparison.

## Implemented changes

1. `internal/terminal/ring.go`: retain mutable byte slices behind the existing ring lock and copy only at read boundaries. Contiguous output remains one segment, redacted markers remain typed, and replay sequence semantics stay unchanged. Trimming advances the retained slice instead of copying the complete scrollback. An oversized append releases its oversized backing allocation. Cleared segment slots release references.
2. The trim boundary's newline preference now searches only the already-authoritative next 256 bytes. Previously it scanned the entire remaining scrollback and then rejected results beyond those 256 bytes.
3. `internal/attachments/store_fs_put.go` / `store_fs_sweep.go`: a Put scans the filesystem once and reuses that per-operation retained list while holding the same mutation mutex. The second reservation pass still rereads pins and applies current age/count/byte rules; no metadata cache or stale window is introduced.
4. `internal/attachments/store_fs.go` / `store_fs_paths.go`: Open validates content, digest and sidecar metadata from the same descriptor returned to the caller, then rewinds it. It preserves the authoritative sweep and returns a file-backed reader; there is no persistent content buffer/cache. Error and cancellation exits close the descriptor; success transfers ownership to the caller.
5. `terminal-store.ts` / `use-terminal-scope-cleanup.ts`: catalog reconciliation releases pane state for terminals no longer present in the current scope. The existing emulator cleanup remains; still-present panes preserve their reference and connection state. Late events for a removed pane stay ignored.

## Terminal ring measurements

Apple M4 Max, Darwin/arm64, Go 1.27.3, `GOMAXPROCS=2`, default 1 MiB scrollback. Five 200 ms samples per row, medians. Fixtures are in the existing `ring_test.go`.

| Path | Baseline | Buffer reuse only | Buffer reuse + bounded newline search |
| --- | ---: | ---: | ---: |
| Append 256 bytes, ns/op | 466,240 | 14,731 | 280.7 |
| Append 256 bytes, B/op | 3,162,368 | 1,284 | 1,249 |
| Append 8 KiB, ns/op | 366,578 | 24,786 | 8,216 |
| Append 8 KiB, B/op | 3,170,304 | 39,912 | 39,948 |

These are isolated ring measurements, not claims about end-to-end click latency. Repeated steady-state appends amortize the occasional byte-buffer growth; `0 allocs/op` is integer-rounded and does not mean the buffer never allocates.

A complete replay now materializes an immutable string before the consumer renders its bytes: 2,097,216 B/op versus 1,048,640 B/op for `ReplayFrom(0).Bytes()`. A 256-byte replay adds 256 copied bytes. `Snapshot()` remains one 1 MiB allocation. A same-process comparison used the original ring source at `de5ab2f4d` under renamed private symbols through a Go overlay, and the current production ring. Three 200 ms samples per row, medians:

| Paired path | Baseline ns/op | Current ns/op | Baseline B/op | Current B/op |
| --- | ---: | ---: | ---: | ---: |
| Snapshot full retention | 394,641 | 316,996 | 1,048,576 | 1,048,576 |
| Replay full retention then render bytes | 222,503 | 343,244 | 1,048,640 | 2,097,216 |
| 100 × 256-byte writes then one full replay | 44,865,812 | 299,129 | 317,293,648 | 2,231,788 |

Snapshot timings overlap substantially across samples; no snapshot speedup is claimed. Complete replay is about 54% slower in this paired run because it copies at the immutable read boundary. The output-heavy mixed fixture is 99.3% faster with 99.3% fewer allocated bytes despite that tradeoff. The fixture models write-heavy output and does not prove that every workload improves. No additional wire segmentation, pooling, cache or goroutine was added.

Commands:

```sh
python3 .cache/performance/run-bounded.py go test ./internal/terminal -run '^$' -bench '^BenchmarkRing(Append|Read)Retention$' -benchmem -benchtime=200ms -count=5
python3 .cache/performance/run-bounded.py go test ./internal/attachments -run '^TestFilesystemAttachmentStore' -bench '^BenchmarkFilesystemAttachmentStorePutRetained$' -benchmem -benchtime=200ms -count=5
```

## Attachment Put measurements

Real temporary filesystem, one session directory per retained object, content plus canonical sidecar, deterministic time and oldest-first eviction at the configured count limit. Five 200 ms samples per row, medians; the corrected fixture uses `KindFile`. Baseline uses the original Put/sweep files in a Go overlay and exactly the same benchmark source/toolchain as after.

| Retained objects | Baseline ns/op | Current ns/op | Baseline B/op | Current B/op | Baseline/current allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| 128 | 51,590,812 | 40,274,325 | 1,536,814 | 782,249 | 9,195 / 4,730 |
| 512 | 123,970,208 | 77,965,222 | 6,099,600 | 3,069,445 | 36,096 / 18,194 |

This removes one complete filesystem traversal per new Put: 21.9% and 37.1% less elapsed operation time in these runs and about half the allocations. Host filesystem timing varies; count and allocation reduction are the stable mechanism. Both retention passes still reread pins because durable queue pins can change independently of the attachment mutex. This filesystem fixture uses no external pin source: production SQL pin lookups are unchanged, so these numbers do not imply that SQL cost is halved.

## Attachment read-path investigation

The original Open called `statLocked` → `readMeta` → `readAttachmentFile`, which read and hashed the entire payload to validate metadata, then reopened the path and hashed the returned descriptor again. The second hash cannot simply be removed: it checks the descriptor after the path may have changed. The implemented change validates and returns one descriptor instead. Canonical round-trip checks still verify the reader starts at offset zero, while metadata-integrity and foreign-workspace checks now explicitly exercise both Stat and Open.

Stat/Open still perform the complete authoritative retention sweep, including sidecar/size validation of all retained objects. Pins are fetched once per distinct session within each pass; the daemon adapter calls `ListPendingSessionInputs` for that session. Batch lookup would cross the daemon/global DB API and needs its own real queue workload. No pin cache, background-only retention or retention freshness change is introduced here.

The real-file baseline is 127,966 ns / 85,581 B per Stat and 164,238 ns / 119,044 B per Open for 64 KiB, and 1,856,745 ns / 4,216,350 B per Stat versus 3,517,741 ns / 4,248,756 B per Open for 4 MiB. These fixtures have one retained target; they isolate repeated content verification and do not include the global-sweep scaling measured in Put.

The initial separate after run showed substantial shared-host noise: unchanged Stat rose to 586,811 ns at 64 KiB and 2,839,618 ns at 4 MiB. It is retained in the raw evidence and is not presented as a Stat regression/speedup. A same-process comparison then used the original Open/stat/hash functions from `de5ab2f4d` under renamed symbols alongside current Open. Both variants share the unchanged retention and metadata-validation behavior and the same fixture; three 200 ms samples per row:

| Open payload | Baseline ns/op | Current ns/op | Baseline B/op | Current B/op | Baseline/current allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| 64 KiB | 175,571 | 125,424 | 119,338 | 85,343 | 84 / 73 |
| 4 MiB | 3,385,035 | 1,803,381 | 4,249,484 | 4,216,137 | 88 / 78 |

The final paired result is 28.6% / 46.7% less elapsed Open work with lower allocations. The returned reader owns only the open file; the temporary verification buffer does not remain attached to the consumer. File-backed streaming, hash/MIME/dimensions checks, scope errors and authoritative sweep remain. The package's real-filesystem suites pass after the change, followed by a focused race run (1.873 s).

Commands:

```sh
python3 .cache/performance/run-bounded.py go test ./internal/attachments -run '^TestFilesystemAttachmentStore' -bench '^BenchmarkFilesystemAttachmentStoreRead$' -benchmem -benchtime=200ms -count=5
python3 .cache/performance/run-bounded.py go test -overlay=.cache/performance/terminal-terminal/attachment-open-paired-overlay.json ./internal/attachments -run '^$' -bench '^BenchmarkFilesystemAttachmentStoreOpenPaired$' -benchmem -benchtime=200ms -count=3
python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race ./internal/attachments -run '^TestFilesystemAttachmentStore' -count=1 -parallel=2
```

## Frontend terminal state measurements

The original store source was saved before editing and run with the same installed XState runtime. Both measurements use pinned Bun 1.4.0. The five-sample fixture opens 1,000 terminal identities in a single scope, reconciles each catalog change when the event exists, then measures 1,000 presence updates. Serialized bytes are an object-size proxy, not a heap measurement. Final timer excludes the serialization itself.

| Metric | Baseline | Current |
| --- | ---: | ---: |
| Retained pane records after 1,000 catalog rotations | 1,000 | 1 |
| Serialized state bytes | 192,817 | 230 |
| Elapsed work for 1,000 presence updates, median ms | 20.548 | 0.191709 |

This removes obsolete pane state and the repeated copying of those obsolete entries during live updates. The timing is an isolated event-store workload, not render latency or a heap profile. Before/after sample ranges were 20.075–21.359 ms and 0.191–0.738 ms.

Raw evidence: `.cache/performance/terminal-terminal/{ring-before,ring-read-before,ring-after,ring-final,ring-paired,attachments-before,attachments-after,attachments-read-before,attachments-read-after,attachments-open-paired,store-before,store-after}.txt`.

## Individual package and system audit

| Package / system | Inspected paths and finding | Disposition / concurrency |
| --- | --- | --- |
| `internal/terminal` | `ring.go`, `session_output.go`, `session_reader.go`, `session_handle.go`, coalescer, manager/aggregate/subscription/reaper. Repeated history copying and unbounded newline search confirmed. | Ring fixes above. Keep ordered per-session output and existing mutex; new goroutines would contend for that lock and disturb ordering. |
| `internal/terminal/journal` | `lane.go`, `query.go`, `output_tail.go`, `live_output_tail.go`, record lifecycle. Query is cursor/limit bounded; live tails are capped at 256 entries, each bounded to 16 KiB. | No measured additional change. Journal lane already serializes writes with cancellation/retry and bounded admission; parallel writes would weaken ordering. |
| `internal/terminal/pty` | `pty_unix.go`, `pipe.go`, IO lifecycle, environment and platform entry points. Process/read ownership and join paths already explicit. | No subprocess or platform behavior change; retain mandatory lifecycle ordering. |
| `internal/terminal/vt` | `actor.go`: mailbox is byte bounded, rebuild has chunk cancellation, screen commands are serialized and emulator drain is joined. | No new goroutines. Allocation before overflow admission is a possible smaller follow-up, not a claimed improvement. |
| `internal/terminal/wire` | `flow_queue.go`, `flow_group.go`, codec/flow suites. ACK/drop watermarks and gap reconciliation bound the stream. | Retain flow control and sequence behavior; no speculative buffering changes. |
| `internal/terminal/idgen` | `id.go`: one 16-byte cryptographic identity read per new entity. | No hot-path bottleneck found. |
| `internal/attachments` | `store_fs.go`, `store_fs_put.go`, `store_fs_sweep.go`, paths/read validation, sweeper. Put traversed the full retained population twice; Stat/Open also sweep globally. | Remove duplicate Put traversal and duplicate Open read/hash. Stat/Open retain their authoritative sweep and full corruption checks. Extra concurrency cannot safely bypass the mutation/deletion lease. |
| `internal/procutil` | Native Darwin/Linux process scanning, enrichment, StartedAt, account/login shell lookup, environment, process identity. Cheap lineage scan is already separate from environment/enrichment; Darwin lsof is batched and bounded. | No change. Per-PID `ps` in StartedAt is a potential launch cost, but its second-granular persisted identity contract makes an unmeasured native replacement unsafe. No production process data was sampled. |
| `internal/subprocess` | Transport inbound handling, stderr pipeline, boundedBuffer, process lifecycle/shutdown and existing perf suite. Stderr is bounded; handlers already use a joined WaitGroup. | No measured additional change. Keep redaction/order, shutdown ownership and transport authority. |
| `internal/managedsidecar` | Path resolution and read/write/remove through no-follow directory handles. | No change. Filesystem checks protect identity/containment; skipping them is not an acceptable speedup. |
| `internal/shellwait` | POSIX AST and PowerShell scan, nested shell/loop detection, hook decode/redirect. | No change; synchronous local parse has no I/O to overlap. Nested shell analysis already has a depth bound. |
| `internal/outboundpolicy` | DNS resolution, complete-answer validation, pinned-IP dial, redirect policy and shared client transport. | No change. Parallel address racing would require measured network evidence and losing-connection cleanup while retaining full DNS validation; no speculative worker pool. |
| `internal/filesnap` | Snapshot metadata/change tokens and fallback fingerprinting. Darwin/Linux use native change tokens instead of full hashes. | No local edit. Anchored-file helper implementation and tests are owned by `extension_marketplace`; see [its report](./extension-marketplace.md). |
| `internal/fileutil` | Atomic publish/remove/sync, rooted no-follow writes and deterministic bounded tar/gzip streaming. | No change. Durability and security checks remain; parallel archive traversal would alter deterministic order and raise memory use without evidence. |
| `web/src/systems/terminal` | Store and scope cleanup, catalog/aggregate leases, query factories, recording clock, wire client, bounded emulator/gap queues, retained-output panes. | Release obsolete pane state. Preserve one stream per owner, server-state keys, ordered emulator operations, bounded buffers and cancellation. |
| `web/src/systems/tool-approvals` | List query, panel/model, mutation hooks, adapters and component shape. List is workspace keyed, enabled only for an actual workspace, AbortSignal aware and has 30 s staleness. Mutations invalidate only their workspace. | No additional bottleneck found; no speculative memoization/cache. |

## Validation and delivery boundaries

- `TestRingReplayContract`: existing cases and new immutable caller/snapshot, repeated UTF-8/escape, concurrent append/replay cases pass in the benchmark run; the focused race run passed.
- `TestFilesystemAttachmentStoreRetention`: adds a fresh-pin change between the two retention passes, proving reuse does not evict an attachment pinned before reservation. Existing real-filesystem round-trip, retention and corruption suites remain authoritative.
- Frontend canonical store/window suites passed through root Turborepo: two files, 50 tests. `--only` was authorized for focused iteration; root owns integrated codegen/current-head CI. An earlier attempt before baseline checkout relocation stopped in codegen because its walker found duplicated generated ownership under `.cache/performance/baseline`; that was infrastructure setup, not a test pass. The baseline has since been moved outside the checkout.
- Race validation passed: `CGO_ENABLED=1 go test -race ./internal/terminal ./internal/attachments -run '^(TestRingReplayContract|TestSessionTailReadContract|TestSessionAttachReplayAndResizeContract|TestFilesystemAttachmentStore.*)$' -count=1 -parallel=2` (terminal 2.305 s, attachments 2.369 s), through the bounded runner.
- Frontend command: `python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/terminal/stores/__tests__/terminal-store.test.ts src/systems/terminal/components/__tests__/terminal-window-app.test.tsx --maxWorkers=1`.
- All filesystem attachment suites also passed in the final Put and Open benchmark invocations. The final Open race rerun passed in 1.873 s. The attachment test-shape checker passed after the read-path coverage update.
- Test-convention checker: attachments suite passes. Ring checker reports the pre-existing `got, _ = incomplete.Snapshot()` line as discarded error; `Snapshot` returns `([]byte, uint64)`, so this is a demonstrated false positive (sequence, not error).
- Relevant QA: `ET-terminal-stream-resilience`, `ET-terminal-browser-lifecycle`, `ET-session-attachment-multiple-drop`, `RT-session-queued-attachment-dispatch`. Root owns the real runtime/browser walk and current-head CI; local measurements are not that evidence.
- No native-tool IDs, routes/DTOs, hooks, extension contracts, config, schemas or official skill text change. Workspace/profile boundaries remain unchanged. Root owns the single cross-surface impact entry and scenario updates.

## Supplemental Go-directory inventory audit — 2026-11-02

This closes eight explicit Go-containing directory entries through read-only static inspection.
All eight Go sources were read in full (393 lines total), together with the schema corpus and the
consumer paths named below. No fixture was executed, no benchmark/build/test was run for this
supplement, and no source was changed. Fixture workload costs are not production improvements.

| Directory | Exact inspected files | Role | CPU, memory and concurrency conclusion |
| --- | --- | --- | --- |
| `desktop/schema` | `desktop/schema/schema.go`; `desktop/schema/app-state.schema.json`; `desktop/schema/fixtures/app-config.json`; `desktop/schema/fixtures/error.json`; `desktop/schema/fixtures/home-resolution.json`; `desktop/schema/fixtures/product.json` | Production cross-language contract data, plus a test corpus. `schema.go` embeds only `app-state.schema.json`; the four corpus files are not runtime work loops. | The embedded definition is 10,185 bytes, loaded with the binary rather than reread per session. Its Go consumer, `internal/cli/app_types.go`, already compiles the schema once via `sync.OnceValues`; individual status records are still decoded/validated per request. This directory owns no goroutine, stream or subscription, and its static data does not grow with session count. No additional cache, concurrency or schema simplification is justified by this inspection. Consumer test references inspected: `internal/cli/app_test.go` and `internal/config/app_test.go`. |
| `desktop/e2e/fixtures/appstage` | `desktop/e2e/fixtures/appstage/main.go`; consumer `desktop/e2e/_electron/__tests__/updates.spec.ts` (`stageAppOperation`); command ownership helper `desktop/e2e/_electron/process.ts` | E2E-only operation seeder, guarded by `//go:build e2e`. It requires an explicit home and complete artifact identity. | One PID-start lookup, two generated identities, one acquire and two ordered operation transitions form a finite setup command. There is no accumulating in-process collection or fixture-owned goroutine. Revision/lease transitions must stay serial; bypassing persistence or parallelizing them would weaken the test. The consumer awaits process completion. No many-session production hot path exists here, and no production speedup is claimed. |
| `desktop/e2e/fixtures/runtimeupdate` | `desktop/e2e/fixtures/runtimeupdate/main.go`; consumer `desktop/e2e/_electron/__tests__/shell.spec.ts` (`buildRuntimeUpdateCoordinator`, E2E-018 runtime swap); command helper `desktop/e2e/_electron/process.ts` | E2E-only runtime-swap driver, guarded by `//go:build e2e`, using an explicit home and replacement binary. | `fileDigest` reads and hashes the whole replacement once: transient memory is O(binary size), not a session-dependent cache. Six configured sleeps deliberately expose ordered update phases; eliminating them would change the observable test. The fixture launches one coordinator with synchronous `CommandContext(...).Run()` and no Go worker fan-out. Its main uses a background context and the sleeps do not provide cancellation; the inspected E2E has a finalizer that kills and joins the fixture process during its protected assertion block. This is harness lifecycle scope, not evidence of a production allocation or cancellation improvement. No fixture hash streaming, phase batching or process redesign was applied. |
| `internal/terminal/testdata/flood` | `internal/terminal/testdata/flood/main.go` | Deliberate terminal-output stress executable under `testdata`, not a production terminal implementation. | Reuses one 33-byte payload in an unbounded stdout-write loop and exits on write failure. Payload memory stays fixed; write throughput and pipe/PTY backpressure are the test stimulus. There are no fixture-owned goroutines or retained-output buffers. Runtime duration belongs to the invoking harness/pipe lifecycle. Throttling, batching away the small writes or shortening the flood would not be a production performance gain. |
| `internal/terminal/testdata/hidden_echo` | `internal/terminal/testdata/hidden_echo/main.go` | Hidden-input fixture: prints a prompt, reads a password without terminal echo, then reports only its byte count. | A single blocking `term.ReadPassword` owns input work; the secret buffer grows with supplied input and is not a globally bounded production queue. The executable does not spawn workers or retain successive prompts. Replacing the read with an asynchronous loop or changing input limits would change the fixture rather than optimize the product. No change. |
| `internal/terminal/testdata/mid_escape` | `internal/terminal/testdata/mid_escape/main.go` | Split-escape regression fixture. | Four fixed byte fragments split an ANSI color escape across writes, each followed by a deliberate 10 ms sleep. Fixed payload memory, finite sequential work and no goroutines. Combining fragments, removing sleeps or writing concurrently would remove the boundary conditions the fixture is meant to exercise. No production optimization or measured claim. |
| `internal/terminal/testdata/prompt` | `internal/terminal/testdata/prompt/main.go` | Interactive prompt/echo fixture. | One buffered stdin read waits for a newline, then echoes once and exits. Heap use scales with that supplied line; there is no repeated history, subscription or worker lifetime to optimize. Waiting for input is intentional, and adding concurrency or a new cap would change the fixture contract. No change. |
| `internal/terminal/testdata/tui` | `internal/terminal/testdata/tui/main.go`; consumer `web/e2e/__tests__/terminal.spec.ts` (E2E-014 alternate-screen TUI) | Alternate-screen fixture built by the browser E2E, optionally held until one input byte arrives. | Emits fixed primary/alternate-screen sequences, optionally blocks on one buffered byte, restores the primary screen, then exits. Memory is small and independent of session history; there are no fixture-owned goroutines. The hold makes watcher/resize assertions possible. Removing it or parallelizing screen writes would destroy ordering/fidelity coverage, not improve production responsiveness. No change. |

Disposition: `desktop/schema` has an already shared, one-time compiled production contract; the
other seven entries are fixture executables. No concrete production performance defect was found
in this inventory supplement. Blocking fixture input, intentional flooding, binary-sized test
hashing and visible phase delays remain explicitly classified as test workload/lifecycle costs.

## PR #1141 shared-root retention triage — 2026-11-02

Review thread: `PRRT_kwDORy7nkc6yiUWd`, concerning the retained-inventory reuse in
`internal/attachments/store_fs_put.go`. Current source inspected:
`1665fb0c1c20c3244bd0d8ca2dc992458eeabcbc`; comparison source: `de5ab2f4d`.

The finding describes a reproducible change when two independently opened stores mutate one
root. It does not identify a supported production concurrency regression. The sole non-test
constructor call is `internal/daemon/boot_session_attachments.go:54`, using the home-derived
`SessionAttachmentsDir` (`internal/config/home.go:252`). `boot_components.go` runs
`bootRuntimeFoundation` before attachment construction; `boot_runtime_foundation.go` acquires
the singleton daemon lock first. `internal/daemon/lock.go` uses `flock.TryLock` and rejects a
second owner. The constructed object is shared by its sweeper, HTTP dependencies
(`runtime_dependencies.go:37`), native tools (`native_tools_dependencies_builder.go:31`) and
session manager (`session_manager_deps.go:102`). Its mutex serializes mutation, retention and
scope-deletion leases. The existing `ScopeLeaseGuard` documentation already limits its deletion
fence to "this store instance". No independently active same-root writer path was found in the
shipped wiring; sequential reopen remains supported and each operation still lists afresh.

Diagnostic question and owning suite: filesystem retention accounting belongs to the existing
`TestFilesystemAttachmentStoreRetention` suite in `internal/attachments/store_fs_test.go`.
A temporary Go overlay added two controlled interleavings inside that suite, using real files,
two store objects, `MaxCount=2`, one seed object and two new publications. The existing pin-source
and file-write seams scheduled the second store's publication at exact boundaries. The final
count was read directly from the raw inventory: `Stat` would sweep again and mask the result.
These diagnostic cases assert successful publication and hook execution, and log the count;
they do not assert an unsupported global cap. No tracked source or test file was changed.
The baseline overlay substitutes only the original Put/sweep files, isolating inventory reuse.

| Second store publication boundary | Original Put/sweep retained count | Current retained count | Causal conclusion |
| --- | ---: | ---: | --- |
| After the first inventory, before capacity reservation | 2 | 3 | Reusing the inventory widens this cross-instance window. |
| After capacity reservation, before the first store publishes | 3 | 3 | The original relist also had a cross-instance check/publish race. |

Both bounded runs passed (baseline 0.480 s; current 0.455 s), including the unchanged canonical
`Should refresh pins again before reserving capacity for a new attachment` regression. Fresh
queue-pin lookups remain authoritative on both retention passes; no pin cache or retention rule
was changed. These are deterministic filesystem interleaving results, not race-detector runs or
new performance measurements. The earlier paired Put measurements remain unchanged.

```sh
python3 .cache/performance/run-bounded.py go test -overlay=.cache/performance/terminal-terminal/pr1141-shared-root-baseline-overlay.json ./internal/attachments -run '^TestFilesystemAttachmentStoreRetention$/(Should_diagnose_shared_root|Should_refresh_pins_again)' -count=1 -parallel=2 -v
python3 .cache/performance/run-bounded.py go test -overlay=.cache/performance/terminal-terminal/pr1141-shared-root-current-overlay.json ./internal/attachments -run '^TestFilesystemAttachmentStoreRetention$/(Should_diagnose_shared_root|Should_refresh_pins_again)' -count=1 -parallel=2 -v
```

Raw receipts: `.cache/performance/terminal-terminal/pr1141-shared-root-{baseline,current}.txt`.
The matching overlay JSON files and `pr1141-shared-root-tests.txt` retain the diagnostic fixture.

Disposition accepted by the root owner: keep production unchanged. Precisely, this optimization
widens an unsupported cross-instance window, while neither version promised an atomic
cross-instance count cap. Restoring the second list would close only the first demonstrated
window, discard the measured traversal improvement, and leave the second race. Supporting
independent concurrent stores would require shared coordination across sweep, publish, removal
and recovery; introducing that behavior is outside this finding. No new lock or ownership
contract was added. Existing pinned content may also intentionally exceed ordinary sweep
bounds; this triage does not reinterpret retention as an unconditional absolute cap.

## Main integration delta audit — 2026-11-02

Scope: incoming `8c025ff8fdd3a9a79e4f0e63f54b3bd144a4773b`, composed with the performance
branch after its prior CI run. The ownership inventory is
`.cache/performance/main-integration-paths.json`: three incoming terminal files and three
tool-approval files, with no overlapping performance edits in those six paths. This is an
integration audit, not a new optimization or a general review of the incoming feature.

Validation invariants, layers and canonical suites identified before execution:

- Exact approval command, risk disclosure, unchanged grant identity and individual revoke belong
  to the terminal presentation layer and `components/__tests__/terminal-approvals.test.tsx`.
- Journal selection, aggregate owner attribution and actor identity belong to the terminal
  journal presentation layer and `components/__tests__/terminal-journal-panel.test.tsx`.
- Workspace-bound remembered decisions, wider-rule restrictions and revoke targets belong to
  the tool-approval component/view-model boundary and
  `components/__tests__/tool-approval-grants-section.test.tsx`.
- Pane reclamation and foreign-scope isolation remain owned by the unchanged
  `stores/__tests__/terminal-store.test.ts` and
  `components/__tests__/terminal-window-app.test.tsx`; their focused rerun checks composition
  against the newly exported UI and identity dependencies.

The initial composed snapshot passed all five suites through bounded root Turborepo:
5 files, 106 tests, 10.08 s. Receipt:
`.cache/performance/terminal-terminal/main-integration-focused-before.txt`.
No new benchmark, Go suite, gate or browser walk was run for these presentational deltas.

| Incoming file | Composition audit and disposition |
| --- | --- |
| `web/src/systems/terminal/components/terminal-approval-detail.tsx` | The exact command is passed to the existing `CodeBlock` as `code`; its copy action receives the same string, and the prompt glyph is presentation only. Irreversible/unclassifiable signals still come from the server detail. No permission, terminal output, replay or ring state is written. Existing approval tests pass. Retain incoming change. |
| `web/src/systems/terminal/components/terminal-grant-row.tsx` | Adds only optional display `agentLabel`; grant ID, digest and the original grant passed to revoke remain unchanged. The default preserves standalone callers. Exact-command revocation test passes. Retain incoming change. |
| `web/src/systems/terminal/components/terminal-journal-row.tsx` | Adds agent identity resolution while human/system retain their actor-kind presentation. Selection, command IDs, replay callbacks and row owner badges are unchanged. The resolver defaults to the selected profile, although each journal entry records `profile_name`; this static limitation is identical in incoming `8c025ff8f` and the merge, not introduced by the performance composition. Existing journal tests pass. Retain incoming change; no independent identity repair in this delivery. No ring/output path changed. |
| `web/src/systems/tool-approvals/components/tool-approval-grant-row.tsx` | Replaces only the agent display with `AgentIdentityChip`; permission scope, decision, input digest and revoke callback still use the source grant. The view uses the selected workspace's cached identity catalog and a network-free fallback outside the app provider. Existing grant interactions pass. Retain incoming change. |
| `web/src/systems/tool-approvals/components/tool-approval-grants-section.tsx` | Humanizes terminal labels and confirmation copy while preserving literal "any agent". Terminal grouping still carries the original source grant into revoke; workspace binding, query envelope/total and wider-rule admission stay in the unchanged view model. Existing scope, failure and revoke cases pass. Retain incoming change. |
| `web/src/systems/tool-approvals/components/__tests__/tool-approval-grants-section.test.tsx` | The only incoming assertion changes are `claude-code`/`cursor` to their humanized display labels. The suite still checks distinct server scopes, exact-input identity, mutation payloads and failures; it does not replace those checks with display-only assertions. All 13 existing cases pass. Retain incoming change. |

Read-only supporting inventory outside Go packages and Web systems:

- `sdk/react/`: no incoming files in the integration inventory. No serializer, reconciler, React
  view runtime or SDK React fixture was changed or rerun for this delta.
- `sdk/client/src/generated/sdk/types.gen.ts`, `sse/types.gen.ts` and
  `sdk/typescript/src/generated/contracts.ts`: textual declaration comparison against the
  pre-merge HEAD found respectively 170, 19 and 15 terminal/approval/actor declarations unchanged.
  `sdk.gen.ts` retains all 24 matching client methods and `openapi/compozy-openapi.d.ts` all 27
  matching operation blocks. Generated index additions and the new
  `AgentHeartbeatWakePatch` export in `sdk/typescript/src/index.ts` concern the incoming agent
  feature; terminal frame/offset/actor and tool-grant contracts are unchanged. Generator and
  whole-repository contract validation remain root-owned.
- All 17 incoming `packages/ui/` paths were inspected. `package.json`, `asset-modules.d.ts`,
  `agent-avatar.tsx`, `hooks/use-document-theme.ts`, `lib/agent-avatar.ts`,
  `lib/agent-avatar-tones.ts`, `exports/conversation.ts`, the avatar test and story introduce the
  shared identity rendering used by the new journal/grant leaves. Its theme observer disconnects
  on unmount; the terminal leaves use `state="unknown"`, so they do not claim live process state
  or activate the working animation. Catalog lookup is indexed by stable catalog array identity;
  these additions do not create a terminal output subscription or retain terminal pane IDs.
- The remaining UI deltas are `hooks/use-topbar-slot.ts`, `topbar.tsx` and its test (optional
  leading host context with equality/publication support), `listing-page.tsx` (nonshrinking
  scroll content), `markdown-editor.tsx`, its test and `exports/editors.ts` (optional tall editor,
  compact default retained), and additive `icons.ts` exports. No terminal approval/journal
  fixture requires changed props. These files were not edited, and this inspection makes no
  visual or browser-latency claim for the incoming UI feature.

The six incoming files are byte-identical to `8c025ff8f` after composition. The journal identity
dependency chain is also unchanged against that commit: `agent/hooks/use-agent-identity-view.ts`,
`agent/hooks/use-agent-identity.ts`, `agent/lib/agent-identity-context.ts`,
`terminal/components/terminal-journal-panel.tsx` and `os/apps/terminal/terminal-window.tsx`.
Consequently, the possible same-slug attribution to the selected profile is an upstream
limitation, not a regression caused by the performance changes. The journal payload carries
profile ownership but no workspace ID; workspace is held by its query/host. The root owner
explicitly kept that independent identity work outside this integration. No new test or UI
change was added, and no all-profile identity browser claim is made here.

Focused validation command:

```sh
python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/terminal/stores/__tests__/terminal-store.test.ts src/systems/terminal/components/__tests__/terminal-window-app.test.tsx src/systems/terminal/components/__tests__/terminal-approvals.test.tsx src/systems/terminal/components/__tests__/terminal-journal-panel.test.tsx src/systems/tool-approvals/components/__tests__/tool-approval-grants-section.test.tsx --maxWorkers=1
```

Final integration disposition: no repair required in this ownership. `internal/terminal` has
no incoming diff against the performance HEAD, including the measured ring implementation.
Frontend pane-reconciliation source and its original tests remain intact. The only worker edit
for this integration is this report; root owns the merged CI, code generation and consolidated
coverage. The 106 passing focused cases are composition evidence, not a replacement for those
delivery checks or a browser rewalk.
