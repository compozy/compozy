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
