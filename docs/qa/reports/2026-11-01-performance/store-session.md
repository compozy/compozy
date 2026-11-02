# Store and session performance audit

Baseline: `de5ab2f4d`. Worktree: `responsiveness-audit-20261101`.

## Test ownership

- Exact session query invalidation must retain the same workspace/session/profile scope, invalidate inactive entries, refetch active entries, and preserve custom Query hash configuration. Owning layer: frontend session Query cache. Canonical suite: `web/src/systems/session/lib/__tests__/query-options.test.ts`; existing catalog/live-tail hook suites own trailing reads and post-stop transcript reconciliation.
- Session board classification without an authoritative repository identity must ignore stored PRs while retaining badge precedence, counts, page bounds and child relations. Owning layer: global SQLite session catalog. Canonical suite and benchmark: `internal/store/globaldb/global_db_session_index_test.go`.
- Fold retention invariant: a session writer retains at most the two most recent assistant folds; eviction must release large message buffers for garbage collection. Owner: sessiondb transcript fold cache. Canonical behavioral suite: `session_db_test.go` incremental projection/rollback cases; retained-heap benchmark added to existing `perf_bench_test.go`, with four caches and three 2 MiB turns each, not a flaky heap threshold test.

## Coverage inventory

| Package/system | Inspected paths and responsibilities | Outcome |
| --- | --- | --- |
| `internal/store` | `sqlite.go`, `sqlite_config.go`, `write_admission.go`, catalog contracts, timestamp/cursor and query clauses | Existing shared admission waits before borrowing a pooled connection; eight-connection cap retained. No evidence for enlarging pools or parallel SQLite writes. |
| `internal/store/globaldb` | Session page/facet/board/relations/branch reads, process observation persistence, model-catalog hydration and batching, profile attention/notification cursors | Bounded session pages and aggregated relations already landed. Empty-repository board reads now skip impossible PR joins, with an interleaved page benchmark confirming the gain. Extension, worktree and task store seams assigned to their domain agents; no overlapping edits. |
| `internal/store/globaldb/schema` | Embedded schema stream and session index definitions | Existing catalog sort/profile/workspace and relation indexes inspected; no schema change needed for this lane. |
| `internal/store/globaldb/sqlcgen` | Generated catalog/notification/extension/worktree/task access follows owning SQL | Generated edits coordinated by root; source SQL is authoritative. |
| `internal/store/sessiondb` | Transcript page/changes/navigation/fold cache, event query scans, read-only lease pool and writer reset paths | Found a retained third fold behind a two-fold slice cap. Event/query pages retain cursor/generation ownership; full-history search is intentionally complete and is not silently truncated. |
| `internal/store/sessiondb/schema` | Projection indexes and embedded stream | `start_sequence`, `(kind,turn_id,start_sequence)` and changed-sequence reads already indexed. No index or migration added. |
| `internal/store/sessiondb/sqlcgen` | `transcript_projection.sql` page/search/outline and fold queries | No generated edit. Prefix-count validation remains: removing this fence without proof would trade corruption detection for a micro-optimization. |
| `internal/store/workspacedb` | `pool.go`, pool lifecycle/removal and terminal journal queries | Open coalesces concurrent requests per workspace, releases mutex before filesystem/database I/O and honors wait cancellation. No goroutines or new cache added. |
| `internal/store/workspacedb/schema` | Embedded schema and journal retention keys | No change warranted by the inspected paths. |
| `internal/store/workspacedb/sqlcgen` | Terminal artifact/recording CRUD and expiry queries | No change; domain terminal agent owns runtime throughput. |
| `internal/listcursor` | `cursor.go`, existing codec suite | Bounded 4096-byte versioned query-fingerprint codec; no high-frequency data walk or lifetime to parallelize. |
| `internal/appstatecontract` | `schema.go` | Constant-only schema version, no executable hot path. |
| `internal/diagnosticcontract` | `diagnostics.go` registry construction, lookup and validation | Runtime code/category validation already map-indexed; sorted administrative snapshots are bounded registry copies. No speculative caching. |
| `internal/notifications` | Cursor service/normalization and scope/error contracts, corresponding globaldb cursor persistence | One direct store call per operation; monotonic confirmation and full scope kept. No retained event stream or per-session fan-out inside this package. |
| `web/src/systems/session` | Catalog hooks/reconciliation, canonical key/options/invalidation, live-tail/runtime/transcript projection and retention policies | Exact-key wake matching checks the resource/session tail first, retaining logical-key semantics and Query lifecycle even under per-query hash overrides. Broad invalidation stays necessary only for prefix families. |
| `web/src/systems/notifications` | Query options/keys and attention API adapter | Profile identity and AbortSignal preserved. It owns no polling timer; session catalog reconciliation and shell own wakes. No independent stale-window change introduced. |

## Concurrency assessment

These fixes remove work and release references. Adding goroutines inside SQLite row decoding or the single session writer would duplicate work, contend on the writer, or violate snapshot/fold ownership. Existing pool open coalescing and write admission already permit independent databases to progress without holding the shared map mutex during I/O. No new goroutine or synchronization primitive is required.

## Measured memory correction

`BenchmarkTranscriptFoldRetainedHeap` retains four session-writer fold caches, each receiving three distinct 2 MiB assistant messages, then forces GC while keeping the caches alive. Before/after use Go 1.27.3, `GOMAXPROCS=2`, `-benchtime=1x -count=3` on the same host. The median retained heap per cache fell from **6,303,472 B to 4,199,638 B (33.37%)**. Ranges: before 6,301,714–6,309,406; after 4,198,032–4,205,904 B. These are retained fixture heap measurements, not a production process RSS claim.

Cause: trimming a slice to two folds left the third pointer in its backing array, keeping the oldest large assistant message reachable. `slices.Delete` clears evicted/taken tail references, so the advertised two-fold cap also holds for the garbage collector. The writer still owns the cache and all transaction/reset semantics remain unchanged.

## Measured local-board correction

`BenchmarkSessionCatalogLocalBoard` seeds 4096 durable stopped User-scope sessions in a real temporary SQLite database, requests the Done column, and reads a 50-row page with no repository identities. The before fixture was copied alone into the frozen baseline tree. Production baseline files remained unchanged.

The final sequence alternated **before, after, after, before**, with three 200 ms samples per process (six measurements per variant), Go 1.27.3 and `GOMAXPROCS=2`. Median page time fell from **8.969 ms to 7.353 ms (18.02%)**. Before range: 8.372–10.249 ms; after: 6.777–7.709 ms. Median allocations fell from 2305 to 2283.5 per operation (six-sample median), and allocated bytes from 289,802.5 to 283,457 B/op. An independent initial five-sample comparison was 8.406→6.758 ms (19.60%). These are store-call measurements including count and bounded row hydration, not HTTP or browser timings.

The actual count `EXPLAIN QUERY PLAN` retained `idx_sessions_global_catalog_navigator (archived_at=?)` and removed three indexed LEFT JOIN probes (`worktrees`, `worktree_status`, `forge_pull_requests`) plus the correlated `json_each` default-branch subquery. Without repository identity none of those PR rows can be authoritative, so the shared source supplies `NULL AS board_pr_state`. Non-empty repository maps still use the existing branch/default-branch/repository path. The same source serves facets and child relations; canonical board tests preserve badge precedence, lineage, scope, counts, and keyset paging. New assertions cover both nil and empty repository maps while conflicting PR records remain stored.

Facet timings were less stable: the initial medians were 13.013→12.443 ms, while the interleaved medians were 16.529→12.825 ms. The change is retained for the consistent paged-read gain and simpler plan; this report does not claim a stable facet latency improvement. Raw receipts: `.cache/performance/store-session/local-board-baseline-fixed.txt`, `local-board-after.txt`, and `local-board-paired.txt`.

## Attempt ledger

| Attempt | Evidence | Decision |
| --- | --- | --- |
| Query hash-index exact lookup | Synthetic usage median 5.73→2.74 ms, catalog 20.88→13.31 ms under Bun 1.4.4; misses per-query custom hashes | Rejected before delivery; logical-key compatibility takes precedence over the timing. |
| Logical exact comparison from the key tail | Pinned Bun 1.4.0, median of three interleaved runs: surfaces 9.392→7.652 ms; usage 6.707→4.153 ms | Keep the invalidation comparator; remove the catalog hook integration because catalog remained within noise (21.011→20.698 ms). |
| Materialize narrow session classification CTE | First paired median 7.369→7.137 ms for hosted facets; page remained neutral. A later process failed during concurrent SQL generation | Removed; the incomplete comparison does not establish a material latency win. |
| Skip PR joins with no repository identity | Interleaved SQLite page median 8.969→7.353 ms; count plan drops three joins and JSON subquery | Keep the three-line empty-map source path; preserve the repository-aware path. |
| Clear evicted fold references | Retained heap median 6.30→4.20 MB/cache | Keep; no added cache or concurrency. |
| Remove per-append event-prefix COUNT fence | Static candidate only; fence protects assigned-prefix consistency | Not changed; no measured win or equivalent corruption/rewind guarantee established. |

Two initial current-worktree local-board measurements failed before executing the benchmark because another agent's worktree benchmark was temporarily uncompilable. Two frozen harness setup failures (an unused import, then a missing required workspace field) were corrected before any accepted local-board sample; neither yielded timings. The final baseline uses the root's frozen `de5ab2f4d` baseline tree plus only the local-board benchmark harness copied from this lane; no production baseline file was modified. The initial frontend run was stopped by concurrent sqlc drift before tests, so focused iteration reruns through root Turbo `--only`; final integrated generation checks remain root-owned.

The attempted catalog-hook reuse of the same comparator produced no material end-to-end synthetic catalog gain and was removed. Only the owning invalidation module keeps the reverse exact comparison; no shared helper API or hook behavior change remains. The paired benchmark has three warmups and ten samples per process, then alternates six separate before/after processes (Bun 1.4.0) against the frozen baseline. Surface invalidation improved **18.52%**; usage invalidation improved **38.09%**. Both refer to 64 updates across 1024 retained queries, not browser paint or HTTP latency.

Static query inventory was scanned independently by store stream: `globaldb` 71 SQL files / 811 named queries; `sessiondb` 3 SQL files / 50 named queries; `workspacedb` 1 SQL files / 16 named queries. Unbounded session-related `:many` declarations were inspected, with representative callers traced through administrative identity/configuration populations, input queues, retention, and one transcript entry's assigned events. This is static inventory and targeted call-path review, not runtime measurement of every query. Complete reads were not silently paginated because that would change their correctness contract. Public rich catalog pages and transcript pages remain explicitly bounded.

Closed writer retention was also traced through `Manager.closeSessionRecorder`: the manager detaches its recorder after close even on error, and normal stopped sessions therefore do not retain a closed database's fold cache. No extra shutdown reset was added without an owned retention path.

## Additional call-path checks

- `ListSessionMonitors` is the complete boot-recovery population (`monitor/recover.go` → `LoadMonitors`), with finished monitor pruning owned by the monitor service. It is not the per-session HTTP monitor listing. No page cut was inserted into recovery.
- Folder counts use the existing `session_placements(folder_id)` index and session primary key under profile/workspace scope. The inspected query is one SQL read, not per-folder application round trips.
- `ListSessionLoopWork` uses scoped active loop rows and correlated ownership/lease reads. Runtime loop batching and scheduler work belong to the loop and daemon lanes; no independent cache of mutable work state was introduced here.
- Session live-tail applies frames in order in a single cache write per batch, caps pending frames at 64, fences generation/sequence, and aborts/closes timers and transport on disposal. Existing transcript page release and warm-cache policies were retained. This lane does not claim a new browser streaming or heap measurement.

## Focused validation

Every command below ran through `python3 .cache/performance/run-bounded.py` from the worktree root, with the shared validation slot, `GOMAXPROCS=2`, and `GOFLAGS=-p=2`.

| Check | Result | Raw receipt |
| --- | --- | --- |
| Root Turbo `test --filter=./web --only` for `query-options.test.ts` and `use-session-catalog-streams.test.tsx`, `--maxWorkers=1` | 57 tests passed across two existing suites, including active/inactive reads, scope, per-client hash, per-query hash function and explicit hash override | `.cache/performance/store-session/frontend-final.txt` |
| Go race, real SQLite: `./internal/store/sessiondb -run '^(TestSessionDBTranscriptProjection\|TestSessionDBArchivesEventRangesWithoutDeletingHistory)$' -count=1 -parallel=2`, `CGO_ENABLED=1`, modernc checkptr exclusion | Passed (4.784 s); incremental projection, rollback and archive behavior retained | `.cache/performance/store-session/sessiondb-race.txt` |
| Go race, real SQLite: `./internal/store/globaldb -run '^(TestSessionCatalogBoard\|TestSessionCatalogPagingIndexesFreshDB)$' -count=1 -parallel=2`, `CGO_ENABLED=1`, modernc checkptr exclusion | Passed (6.795 s); includes badge/PR equivalence, repository isolation, scoped lineage and paging | `.cache/performance/store-session/globaldb-race-final.txt` |
| Go local-board benchmark `-run '^$' -bench '^BenchmarkSessionCatalogLocalBoard$' -benchtime=200ms -count=3 -v`, B-A-A-B against frozen baseline | Six samples per variant, all after page samples faster than all before samples; count plans logged | `.cache/performance/store-session/local-board-paired.txt` |
| Root-pinned Bun 1.4.0, frozen baseline/current interleaved existing cache benchmark | Three warmups and ten samples per process, three processes per variant | `.cache/performance/store-session/cache-paired.jsonl` |
| Go retained-heap benchmark `-run '^$' -bench '^BenchmarkTranscriptFoldRetainedHeap$' -benchtime=1x -count=3` | Three before and three after measurements | `.cache/performance/store-session/fold-heap-before.txt`, `fold-heap-after.txt` |
| Scoped `gofmt` and `oxfmt` | Passed formatting; existing test convention checker reports five pre-existing top-level attention/interaction tests without subtests in `global_db_session_index_test.go` | `.cache/performance/store-session/format-conventions.txt` |

Reproduction commands for the focused behavioral checks:

```sh
python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/session/lib/__tests__/query-options.test.ts src/systems/session/hooks/__tests__/use-session-catalog-streams.test.tsx --maxWorkers=1
python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race '-gcflags=modernc.org/...=-d=checkptr=0' ./internal/store/sessiondb -run '^(TestSessionDBTranscriptProjection|TestSessionDBArchivesEventRangesWithoutDeletingHistory)$' -count=1 -parallel=2
python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race '-gcflags=modernc.org/...=-d=checkptr=0' ./internal/store/globaldb -run '^(TestSessionCatalogBoard|TestSessionCatalogPagingIndexesFreshDB)$' -count=1 -parallel=2
```

The convention findings refer to `TestSessionAttentionSeenFenceIsMonotonicAndPassiveReadsArePure`, `TestSessionAttentionEndedEarlyMarkDrivesDurableCatalog`, `TestSessionAttentionLifecycleUpdateBumpsOrderingFenceAtomically`, `TestPendingInteractionCanonicalTransactionRedactsAndRollsBackTogether`, and `TestOrphanInteractionResolutionCouplesQueueAndIsIdempotent`. This lane's new assertions extend an existing `t.Run` board case. Unrelated test cleanup was not mixed into this change. The source formatters passed; the separate report-only `oxfmt` invocation returned exit 2 because repository ignore rules exclude QA Markdown reports (`report-format.txt`), so it provides no report-formatting evidence.

## Cross-surface impact and delivery boundary

No public wire shape, native-tool ID, CLI flag, hook/config contract, SQLite schema, or official `skills/compozy/` guidance changes. Workspace/profile identity and existing read authorization are preserved. Relevant QA journeys for the root's integrated real run: `ET-web-catalog-navigation`, `RT-session-attention-catalog`, `ET-web-sessions-board-paging-archive`, `ET-web-sessions-board-children`, `ET-web-session-context-meter`, and `RT-visible-session-streaming`.

Extension registry batching is owned and measured in [extension/marketplace](extension-marketplace.md); worktree list batching belongs to [worktree/workspace](worktree-workspace.md); scoped task-event loading belongs to [daemon/observe](daemon-runtime.md). Their store seams and generated SQL are not included in this lane's numerical claims. Root owns integrated generation, browser/daemon QA, final gates and current-head CI. Focused results here do not substitute for those delivery checks.

## Owned final files

- `internal/store/sessiondb/transcript_fold_cache.go` and existing `perf_bench_test.go`: clear retired fold pointers and measure retained heap.
- `internal/store/globaldb/global_db_session_board.go` and existing `global_db_session_index_test.go`: skip impossible PR joins, preserve repository classification with behavioral coverage, and measure local-board reads.
- `web/src/systems/session/lib/session-query-invalidation.ts` and existing `lib/__tests__/query-options.test.ts`: compare exact keys from the tail while preserving logical equality, scoped invalidation, active refetch and custom hashes.
- This report. No catalog reconciliation hook diff, facet CTE, schema, generated-file, dependency, pool-size, or goroutine change remains in this lane.
