# Store and session performance audit

Baseline: `de5ab2f4d`. Worktree: `responsiveness-audit-20261101`.

The original measurements below retain that baseline. The final section records the separate integration audit and local-board comparison against incoming main `8c025ff8f`; it does not reinterpret the historical numbers as measurements of the new Stays-on surfaces.

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

## Main integration delta — 2026-11-02

Reviewed the uncommitted merge of performance head `e62862f48362b9cc6da311df0078b0be914a5734` with main `8c025ff8fdd3a9a79e4f0e63f54b3bd144a4773b`, using `.cache/performance/main-integration-paths.json` as the delta inventory. No concrete integration defect was found in this lane. This is a targeted integration review, not a new audit or performance claim for the entire upstream Stays-on implementation.

The sole textual conflict in `global_db_session_index_test.go` was an append/append conflict. Resolution removed only the conflict markers and retained both complete authorities: `BenchmarkSessionCatalogLocalBoard` plus its plan logger, and incoming `TestPendingInteractionDescendants`. No assertion was weakened. The already-merged pointer arguments to `RegisterSession` and the incoming instance-main facet case were preserved. `global_db_worktree_test.go` was explicitly owned by the worktree/workspace lane and was not edited here.

Before resolving the test conflict, the invariants and owners were identified: repository-less board classification/counts/paging remain owned by `TestSessionCatalogBoard`; original descendant session/request identities, expiry, and exclusion of unrelated sessions remain owned by `TestPendingInteractionDescendants`, both in the existing real-SQLite globaldb suite. Existing suites own the additional integration checks; no new test file or duplicate assertion was added.

### Delta map and compatibility

| Delta area | Inspected integration boundary | Result and limit |
| --- | --- | --- |
| Store contracts and session registration | `SessionInfo`, metadata, catalog/creation/derivation interfaces, runtime option accessors, initiator fields and permission audit contracts | Incoming pointer signatures and identity fields remain authoritative. The package compiled with the merged tests. This lane did not change registration or migration policy. |
| Global session page, scan, facets and board source | `global_db_session_{page,scan,facets,board}.go`; `BoardExcludedIDs`; triage visibility; page/count filters and bound argument order | The three selected identity columns match the scanner order. The fast path still selects `sessions.*`, so it retains instance/initiator fields while substituting only `board_pr_state`. `BoardExcludedIDs` changes board counts independently of total badge counts, with its bind parameters preceding source/filter arguments. Main-session page exclusion stays a server-owned population decision. Real SQLite checks passed for both old board rules and the incoming facet rule. |
| Instance lifecycle and authority | `global_db_instances.go`, `global_db_instance_{session,authority,adopt,lifecycle,reconcile,rotation}.go`, owning SQL | Live identity and active-cap admission remain transactional; registration/adoption stamp matching profile, workspace and agent identity; current-main guards protect archive/delete and rotation publication. List/reconciliation reads use a 1–100 row bound and stable ID continuation. These new upstream paths add no dependency to the repository-less board projection or fold cache. Runtime/service review is complementary in the Loop lane. |
| Instance reports, pending interactions and prompt admission | Instance report repository; pending/prompt report mapping; input clear/cancel/replacement; descendant interaction scan | Source-key deduplication, current-main publication, 20-report reservation/dispatch bounds, and transactional release paths remain upstream-owned. Public report pages are bounded and ordered by `(created_at,id)`; report decoding parses timestamps and copies the bounded payload. Recursive descendant reads retain session and provider request identity and restrict profile/workspace reach. The new expiry column matches every owning scanner. |
| Wake, collectors, activity and seen | Instance wake/check/dedupe/finish repositories; heartbeat collectors; activity/check-in/seen reads | Due/reconciliation/public history pages are bounded. Inflight recovery and collector populations intentionally remain complete scoped reads; they are not relabeled as paginated UI history. Activity uses grouped aggregates, and seen writes use a monotonic `(instance,operator)` record. No additional cache, worker or concurrency policy was introduced. |
| Authoring, schema and generated SQL | Profile-aware soul/heartbeat mapping and rollback queries; workspace archive/deletion guards; migrations 164–172; generated globaldb mappings | Incoming profile authority, instance/archive protection, nullable fields and declarative schema were retained. Main adds six SQL files and 67 named queries; merged globaldb has 77 SQL files / 878 named queries, versus main's 76 / 875. The remaining difference is the existing extension visibility and task-event batching from this branch. No schema or generated artifact was edited by hand; root owns regeneration/equivalence checks. |
| Frontend catalog/list/board | Normalized `exclude_instance_main` filter, canonical list keys, shelf/group/board hooks, resident groups, attention counts and Stays-on rows | Population filters survive normalization and therefore participate in both request and cache identity. Counts remain served facets; row flattening stays at the view boundary. The new instance stream listener is detached with the existing subscription. Our exact-key comparator is unchanged and still compares logical flat keys; catalog/detail prefix families retain scope and per-query custom-hash behavior. |
| Frontend session identity and reports | Runtime provider's `AgentIdentityScopeContext`, report source reads, relayed asks, scoped agent identity, superseded-session lookup | Runtime identity uses the loaded session profile/workspace. Report source reads use the profile-aware by-ID factory; relayed asks use the existing detail-key subtree and preserve descendant `session_id`. New report payload rendering accepts unknown report kinds without discarding the retained transcript. This lane made no new navigation, loading or provider-lifetime change. |
| Transcript folds and new report entries | `session_event_write.go` dispatch, fold cache, incoming `EntryKindReport` projection, retained transcript consumers | Only assistant entries enter `projectAssistantEntry`; reports use `ProjectAssignedEntry` and do not close the active assistant boundary. Clearing evicted pointers does not change this routing. Sessiondb projection/archive race checks passed; the session/OS lane additionally ran the existing `TestReportProjection` race case. |
| Other previously audited store packages | `sessiondb` schema/generated queries, `workspacedb` and its nested packages | No incoming source delta. Existing ownership and the historical evidence remain unchanged; no repeat full-package audit or benchmark was performed. |

The frontend inventory contains two adapter files, 70 component/test/story files, 28 hook/test files, 31 library/test files, five mock files and four system entry/type files. Review concentrated on the data and lifecycle boundaries above; identity-only presentation changes and their incoming tests were inventoried rather than treated as new optimization opportunities.

### Merge validation

All commands used the shared bounded runner. No gate, full suite, daemon or browser was started by this lane.

| Check | Result | Receipt |
| --- | --- | --- |
| Globaldb real-SQLite race: board, visibility, stable keyset, paging index, descendant interactions; instance registration/adoption/isolation/activity/seen/check-ins/reports; initiator migration | Passed, 13 named top-level groups, 20.305 s | `.cache/performance/store-session/main-integration-globaldb.txt` |
| Root Turbo Web `--only`, one worker: query options, catalog streams, list groups, sessions hook, board, runtime provider, Stays-on signals, report part and relayed approvals | 256/256 tests passed across nine existing suites; Turbo 39.172 s | `.cache/performance/store-session/main-integration-frontend.txt` |
| Sessiondb real-SQLite race: `TestSessionDBTranscriptProjection` and `TestSessionDBArchivesEventRangesWithoutDeletingHistory` | Passed, 3.478 s | `.cache/performance/store-session/main-integration-sessiondb.txt` |
| Incoming `TestReportProjection` race, executed by session/OS owner | Passed within transcript package, 1.435 s; reused without rerunning | `.cache/performance/session-os/main8c-go-focused.txt` |
| Existing Go test-shape heuristic | Same five historical top-level-test findings listed above; exit 1, no new finding from the conflict resolution | `.cache/performance/store-session/main-integration-conventions.txt` |
| Read-only `gofmt -l` on the resolved Go test | Passed, no file listed | `.cache/performance/store-session/main-integration-format.txt` |

The frontend log includes React `act` notices, incoming board fixtures without a RouterProvider, an undefined mocked facet response, and the expected failed-approval diagnostic. The passing test result is not a zero-warning claim. Production data boundaries were not changed to suppress fixture diagnostics.

### Main-based local-board comparison

The session row hydration, triage predicate and facet work changed upstream, so the earlier `de5ab2f4d` timings cannot establish the incremental gain on main. A separate short comparison uses the frozen main checkout and the exact same 77-line local-board benchmark/plan harness through Go's overlay mechanism. Baseline production and test files remain unchanged; the baseline board/page/facet/scan/test bytes were checked against `8c025ff8f`. Both variants seed 4096 stopped global sessions and request the same 50-row Done page; only the Page sub-benchmark is timed.

The sequence was **main, merge, merge, main**, three samples per process with `-benchtime=200ms -count=3`, Go 1.27.3, `GOMAXPROCS=2`, `GOFLAGS='-p=2 -buildvcs=false'`, Darwin/arm64 on the same Apple M4 Max. The benchmark selector was `-run '^$' -bench '^BenchmarkSessionCatalogLocalBoard$/Page$' -v`. Every process ran serially through `run-bounded.py`.

| Page metric | Main `8c025ff8f` | Merged working tree |
| --- | ---: | ---: |
| Median, six samples | 7.173808 ms | 5.720850 ms |
| Range | 7.063201–7.284372 ms | 5.688211–5.953339 ms |
| Median allocated bytes/op | 304,561 | 298,193.5 |
| Median allocations/op | 2,309 | 2,288 |

The incremental median reduction is **20.25%**. All six merged page samples are faster than all six main samples. Main's count plan includes the three LEFT JOIN probes and correlated default-branch subquery; the merged plan retains the same session catalog index and removes those impossible joins. This establishes the local-board store-call improvement on the new main baseline. It does not quantify the upstream change from `de5ab2f4d`, facet speed, HTTP latency, navigation latency, or new Stays-on workloads.

Receipts: `.cache/performance/store-session/main-integration-local-board-{before-1-canonical,after-1,after-2,before-2}.txt` and `main-integration-local-board-summary.json`. The test overlay is `main8c-local-board-overlay.json` and its isolated replacement file is `main8c-local-board-test.go` in the same receipt directory. Harness SHA-256: `db5fc2eab2ec2a10e16e641e6c3592c18ee6da94055946e72fda58432cf38540`. The original frozen test file remains SHA-256 `2d8ace62f957dff24eb69eae0bc006c885ba99327a22d73c57e4a7b73e345089`.

The first overlay invocation used macOS's `/tmp` spelling while Go canonicalized the package directory to `/private/tmp`; the overlay was not applied and no benchmark ran. That successful empty test process supplies no measurement (`main-integration-local-board-before-1.txt`). The overlay was corrected to the canonical source path before collecting samples.

### Reviewed source freeze

This lane's merge edits are limited to the resolved test conflict and this report. The three optimized production modules remain byte-identical to the performance parent. Selected integration hashes are stored in `.cache/performance/store-session/main-integration-source-hashes.json`; the primary freeze is:

| File | SHA-256 |
| --- | --- |
| `internal/store/globaldb/global_db_session_index_test.go` | `0052a1b59d30a35cf91f58ec8e9ff8fcc610a8cf6a54a449f110360d552534aa` |
| `internal/store/globaldb/global_db_session_board.go` | `e4f8ba150b02745da85c05163989b97345c26db9501c2078d43a74dc40f22e95` |
| `internal/store/sessiondb/transcript_fold_cache.go` | `01ddc83c90c8b53ed701e1ce85291adeb52c79c5d5e2531bac63c500a127c3df` |
| `web/src/systems/session/lib/session-query-invalidation.ts` | `82e58ae01a32939cc3867d0a77c12bd8237227936c65d4141865221adf0592f0` |
| `web/src/systems/session/lib/__tests__/query-options.test.ts` | `ed2ac05ba3871adc7d4f20e1c2034bf1e44e78d6423bcd9d6b5bcf56368611b4` |

### Scope of the integration conclusion

This lane added no public surface, schema, native tool, hook/config or official skill change beyond the incoming upstream contract; the root change-impact audit owns their combined accounting. Existing QA authorities are the catalog/board/streaming scenarios listed above plus incoming `MS-web-stays-on-placement`, `MS-web-stays-on-reports` and `RT-session-attention-catalog`. No scenario expectation needed a merge-specific edit here. Final integrated browser behavior, generated equivalence, gates and current-head CI remain root-owned. Historical navigation traces and per-package microbenchmarks are not presented as measurements of the merged Stays-on UI.

## Session-list keyboard diagnostic on `895d8a5d7` — 2026-11-02

The root's bounded changed-file diagnostic found a concrete failure in `session-list.test.tsx`: “Should nest through the keyboard menu using only eligible parent choices” sent Enter after finding Destination in the DOM, but `onAdopt` received no call. This failure has **no demonstrated causal connection to the separate frontend CI timeout**. The original test, session row actions, choice construction hook and shared dropdown primitive were byte-identical between main `8c025ff8f` and `895d8a5d72840c92e01015bf313a2e2340cb084d`.

The invariant was named before editing: keyboard nesting must invoke adoption with the original source and eligible destination, while excluding a different profile. Owner: session-list interaction. Canonical suite: `web/src/systems/session/components/__tests__/session-list.test.tsx`; its existing case owns this behavior.

An isolated unchanged run passed. Passive capture in the full canonical suite then distinguished the two orderings without changing keyboard actions or assertions. In the passing control, Destination was present while the active element was still “Nest under…”, but the scheduled focus transfer completed before the Enter keydown; Destination received the key and adoption ran once. In the failing buffered capture, both the active element and the Enter event target remained “Nest under…”, and adoption remained at zero. Installed Base UI 1.9.0's `useListNavigation` delegates this handoff to `enqueueFocus`, which schedules `requestAnimationFrame`; DOM presence is not keyboard readiness. The neighboring folder-menu case already waits for focus inside its submenu.

The minimal correction retains the Destination element and waits for `expect(destination).toHaveFocus()` before the same Enter. Destination presence, Foreign exclusion and the final `onAdopt(source, target)` assertion remain intact. There is no programmatic focus, sleep, action retry, increased timeout, production edit or retained instrumentation. The new focus assertion also verifies the automatic handoff instead of bypassing it.

| Evidence | Result | Receipt under `.cache/performance/store-session/` |
| --- | --- | --- |
| Unchanged isolated case through root Turbo `--only`, one worker | Passed, 1 selected / 136 skipped; Turbo 3.403 s | `session-list-keyboard-isolated-before.txt` |
| Passive full-suite control | 137/137 passed; Destination received Enter after the focus handoff | `session-list-keyboard-focus-diagnostic.txt` |
| Buffered capture before correction | Existing adoption assertion failed; Enter targeted “Nest under…” and adoption count was zero | `session-list-keyboard-focus-buffered.txt` |
| Corrected full canonical suite through root Turbo `--only --force`, one worker | 137/137 passed; Turbo 9.643 s | `session-list-keyboard-after.txt` |
| Targeted Oxfmt check and Oxlint with `--deny-warnings` | Both passed | `session-list-keyboard-format.txt`, `session-list-keyboard-lint.txt` |

All executions used `run-bounded.py`. An attempted runtime comparison in the frozen main export could not start because its `node_modules` were no longer present (`vitest: command not found`, exit 127, `session-list-keyboard-main8c-before.txt`); it supplies no test result. The main comparison here is source equality, while the causal failing/passing traces and corrected canonical suite ran on the current checkout. Existing React `act` notices remain visible in the suite log. No further repetitions or broader local checks were run after the canonical suite passed. Root owns full current-head CI with continuous command logs.
