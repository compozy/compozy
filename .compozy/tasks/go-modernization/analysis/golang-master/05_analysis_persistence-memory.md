# Analysis: persistence-memory

## Scope

This analysis restarts the persistence and memory review from the first step under the updated Go guidance. The operator direction was:

> "nós atualizamos aqui skills importantes que tem efeito no codigo go, voce tem que voltar ao primeiro passo e iniciar novamente toda a analise dos packages agora considerando a skill nova, o que foi feito pode permanecere, mas analise do seu goal tem que ser reiniciada"

The authorized source surface was every Go source, test, benchmark, and generated Go file below:

- `internal/store/**`
- `internal/resources/**`
- `internal/memory/**`
- `internal/modelcatalog/**`
- `internal/settings/**`
- `internal/deadentity/**`

The corpus contained 800 Go files and 222,468 lines: 601 handwritten non-test production files (104,100 lines), 144 test/benchmark files (95,001 lines), and 55 generated sqlc files (23,367 lines). Every file was enumerated and mechanically surveyed for package shape, file size, SQL cursor and transaction ownership, context creation, goroutine ownership, scope identifiers, error extraction, modern Go constructs, and the mandatory feature set. The high-risk working set was then read at function or file depth and cross-referenced with schemas, query sources, generated output, and canonical tests.

Generated sqlc output and embedded schemas were inspected as evidence only. Recommendations below target declarative schema/query sources, generator configuration, or handwritten adapters; none proposes hand-editing `sqlcgen/*.go`. No tests, generators, formatters, package managers, Git commands, or other state-mutating commands were run under this scoped research contract.

The updated guidance and supplied feature baseline used for the re-audit were:

- `.agents/skills/golang-master/SKILL.md` and its `references/*.md`
- `.agents/skills/eng/eng-code-guidelines/SKILL.md`
- `.agents/skills/eng/eng-cleanup-failure-paths/SKILL.md`
- `.agents/skills/eng/eng-test-conventions/SKILL.md`
- `.agents/skills/eng/eng-consolidate-test-suites/SKILL.md`
- `.agents/skills/eng/eng-schema-migration/SKILL.md`
- `.agents/skills/architectural-analysis/SKILL.md`
- `.agents/skills/refactoring-analysis/SKILL.md`
- `/home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt`

## Overview

The slice has a coherent persistence topology rather than one undifferentiated database layer:

- `internal/store` owns shared SQLite lifecycle, retry, migration, timestamp, ID, and persistence contracts. `globaldb` owns daemon/global and row-scoped workspace data; `sessiondb` owns one SQLite file per session; `workspacedb` owns a database located beneath a canonical workspace identity.
- `internal/resources` is a desired-state kernel over SQLite with actor, kind, source, and global/workspace scope enforcement. Its write path delegates transaction ownership to `store.ExecuteWrite`.
- `internal/memory` combines scope-rooted Markdown files with derived SQLite state, decision history, recall signals, consolidation, extraction, and replay. Global, workspace, and agent identities are explicitly modeled.
- `internal/modelcatalog` presents a source-merged global provider/model catalog through a storage interface implemented by `globaldb`.
- `internal/settings` owns global/workspace configuration projections, secret mutation rollback, apply records, and runtime status probes.
- `internal/deadentity` adds workspace-scoped, durable failure suppression around extensions, bridges, and MCP sidecars.

Several foundations are already strong:

- Four distinct migration streams are checked for sequential versions, independent version tables, domain-table ownership, and declarative-schema equivalence in `internal/store/migrate_streams_test.go:39-118` and `internal/store/migrate_streams_test.go:319-373`.
- Workspace databases resolve a stable workspace identity before opening and physically isolate different roots in `internal/store/workspacedb/workspace_db.go:25-52` and `internal/store/workspacedb/workspace_db_test.go:67-99`.
- Resource access applies both SQL-side narrowing and post-scan actor checks in `internal/resources/kernel.go:362-408` and `internal/resources/validate.go:119-183`.
- Dead-entity persistence keys every operation by `(workspace_id, kind, entity_id)` in `internal/store/globaldb/queries/dead_entities.sql:1-28`, with cross-workspace tests in `internal/deadentity/service_test.go:251-278` and `internal/store/globaldb/global_db_dead_entity_test.go:15-95`.
- Handwritten cursor owners in `resources`, most of `memory`, `sessiondb`, and `globaldb` already join close failures into named return errors. The strongest reusable examples are `internal/resources/kernel_records.go:89-126`, `internal/memory/catalog_rows.go:9-17`, and `internal/store/sessiondb/session_query_scan.go:10-45`.
- No handwritten production file exceeds the 500-line cap. The largest files are near the boundary (`internal/store/globaldb/global_db_task_profile.go` at 498 lines and several files in the 460–486 range), so they must not absorb further responsibilities.

The restart nevertheless found five material risk clusters:

1. The per-session database schema has no durable session or workspace identity. An existing file can be reopened under any nonblank `sessionID`, and read rows are then stamped with that supplied ID.
2. `sessiondb.ReadOnlyPool` and `deadentity.Service` hold mutexes across fallible I/O. The dead-entity path also invokes a store through an unbounded `context.WithoutCancel` context.
3. Recall signal recorders are one non-cancelable goroutine per workspace, retained until global shutdown; timed-out closes create additional waiting goroutines.
4. Some handwritten SQL paths still swallow cursor-close or rollback information, despite nearby canonical helpers that preserve it.
5. Several request and post-commit boundaries discard cancellation or detach work without a timeout.

The modernization opportunity is selective rather than wholesale. `omitzero`, range-over-integer, `slices`, `maps`, `min`/`max`, `strings.SplitSeq`/`FieldsSeq`, and `WaitGroup.Go` already appear where their semantics fit. The remaining high-value language changes are typed `errors.AsType`, the last legacy benchmark loop, a few `sync.Once` result caches, pure-clock `testing/synctest` tests, and additional sequence-based string parsing. `os.OpenRoot` is not merely syntactic: it closes a real symlink-containment gap in memory-file reads.

## Mechanisms / Patterns

### Persistence and ownership map

| Area | Data ownership | Primary mechanism | Existing invariant |
| --- | --- | --- | --- |
| Shared store core | Process/global infrastructure | `database/sql`, bounded `BEGIN IMMEDIATE` retry, embedded Goose/Atlas streams | Context-aware retry and joined connection/rollback cleanup in `internal/store/write.go:61-188` |
| `globaldb` | Mixed global and workspace-row-scoped data | sqlc query packages plus handwritten dynamic queries | Workspace predicates are explicit for scoped tables; dead entities are a representative closed tuple in `internal/store/globaldb/queries/dead_entities.sql:1-28` |
| `sessiondb` | Intended session-scoped data | One SQLite file and one dedicated writer loop per session | Incoming event IDs are checked against the handle's in-memory `sessionID` in `internal/store/sessiondb/session_event_write.go:108-121`; the file itself does not attest that identity |
| `workspacedb` | Workspace-scoped file | Canonical workspace identity + workspace-owned migration stream | Two roots produce different database files and isolated rows in `internal/store/workspacedb/workspace_db_test.go:67-99` |
| `resources` | Global or workspace desired state | Actor capability filters + optimistic versions + serialized source snapshots | A workspace-scoped actor can read only its exact scope, while a global daemon actor may see both, in `internal/resources/validate.go:119-183` |
| File memory | Global/workspace/agent scoped files | Lexically validated basename under derived directory + atomic mutation helper | Separators, dot segments, and invalid agent segments are rejected in `internal/memory/store_scope.go:102-127` and `internal/memory/store_index.go:171-186` |
| Memory catalog | Global/workspace/agent derived state | Scope/workspace/agent identity tuple stored in SQLite | Identity state key includes scope, workspace ID, agent name, and agent tier in `internal/memory/catalog_identity.go:13-44` |
| Model catalog | Global provider/model projection | Source interface + globaldb persistence adapter + refresh coalescing | Provider/source ownership is explicit in `internal/modelcatalog/types.go:152-178` and refresh flights serialize per provider in `internal/modelcatalog/service_refresh.go:281-318` |
| Settings | Global config and selected workspace overlays | Validated collection service, atomic config lifecycle, bounded secret rollback | Secret rollback deliberately uses a detached but time-bounded context in `internal/settings/mcp_secrets.go:233-238` |
| Dead entities | Workspace/external-runtime tuple | Durable globaldb row plus in-memory per-key state machine | SQL key and tests isolate equal entity IDs across workspaces; in-memory lifecycle currently has no eviction |

### Cursor and transaction ownership

The preferred handwritten cursor pattern is a named error return plus one deferred closer that joins a close failure. `resources` does this in `internal/resources/kernel.go:362-408`; memory catalog helpers do it in `internal/memory/catalog_rows.go:9-17`; read-only session queries do it in `internal/store/sessiondb/read_only.go:233-272`. This preserves both the primary scan/iteration error and the cleanup error.

Transaction ownership has also converged around two safe forms:

- `store.ExecuteWrite` owns connection acquisition, `BEGIN IMMEDIATE`, bounded busy retry, rollback with a detached bounded timeout, commit, and connection close in `internal/store/write.go:61-188`.
- Direct `sql.Tx` users normally defer rollback, ignore only `sql.ErrTxDone`, and join any other cleanup failure, as in `internal/store/globaldb/tx_helpers.go:68-97` and `internal/store/sessiondb/transcript_query.go:90-107`.

The remaining mismatches are localized: memory query helpers that only log `Rows.Close`, a settings query with bare `defer rows.Close()`, redundant double-close boilerplate in handwritten watch queries, and migration bootstrap rollback that does not filter `sql.ErrTxDone`.

### Concurrency and lifecycle

The slice generally uses explicit owners: session databases own writer goroutines and cancellation, consolidation owns a ticker loop, extractor runtime tracks per-session work, resource reconciliation owns one cancelable worker, and settings caps MCP probes with a fixed worker count. Modern `WaitGroup.Go` is already used in `internal/settings/mcp_collection_probes.go:25-53` and several concurrency tests.

Three lifecycle patterns need correction:

- `ReadOnlyPool.Open`, `CloseExpired`, and `Close` execute injected or database close/open operations while holding the pool mutex (`internal/store/sessiondb/read_only_pool.go:104-130`, `internal/store/sessiondb/read_only_pool.go:267-316`, `internal/store/sessiondb/read_only_pool.go:339-353`).
- Dead-entity methods hold a per-key state mutex while calling durable stores and an event sink (`internal/deadentity/service.go:73-211`, `internal/deadentity/events.go:19-55`).
- Recall creates and retains a worker keyed by each workspace using `context.WithoutCancel(ctx)` (`internal/memory/recall_source.go:58-85`); the worker processes I/O with that non-cancelable context and `Close` starts a fresh waiting goroutine on every call (`internal/memory/recall/signal_recorder.go:175-251`).

### Scoping and isolation

Row-scoped global data and resource data generally carry explicit workspace predicates. Memory derives workspace identity from the canonical root before catalog access (`internal/memory/store_catalog.go:23-78`) and includes the workspace/agent tuple in catalog state (`internal/memory/catalog_identity.go:20-44`). `workspacedb` also derives the database path from the workspace identity directory (`internal/store/workspacedb/workspace_db.go:25-52`).

The exception is the session database. `OpenSessionDB` and `OpenSessionDBReadOnly` accept a `sessionID` beside an arbitrary path, but the declarative schema contains only events, hook runs, token usage, and transcript projection tables—no immutable database owner row (`internal/store/sessiondb/schema/schema.sql:1-97`). Read scanning assigns the caller-supplied handle ID to every row (`internal/store/sessiondb/session_event_query.go:46-70` and `internal/store/sessiondb/read_only.go:334-400`). This makes path resolution, outside this package, the sole identity authority and leaves the storage boundary unable to detect a cross-session or cross-workspace mix-up.

### Generated boundaries

`globaldb/sqlc.yaml` and `sessiondb/sqlc.yaml` define the declarative query-to-Go boundary (`internal/store/globaldb/sqlc.yaml:1-12`, `internal/store/sessiondb/sqlc.yaml:1-12`). Generated files announce `Code generated by sqlc. DO NOT EDIT`, for example `internal/store/globaldb/sqlcgen/task_core.sql.go:1-12`. Generated cursor patterns—including their own defer/explicit-close shape—are evidence about generator behavior, not direct edit targets.

Schema changes must therefore flow through the owning declarative source, an append-only next migration, `atlas.sum`, query source, and codegen. The existing migration suite already verifies sequential identity and schema equivalence across global, session, memory, and workspace streams (`internal/store/migrate_streams_test.go:39-118`, `internal/store/migrate_streams_test.go:319-373`). The proposed session identity invariant must extend that pipeline rather than patch generated Go.

### Go feature review matrix

Status is scoped to these packages, not the whole repository.

| Feature | Status | Evidence and decision |
| --- | --- | --- |
| `errors.AsType` | `adopt` | It is already proven in `internal/modelcatalog/modelsdev_test.go:330` and `internal/store/globaldb/global_db_task_claim_test.go:1900`, while simple typed extraction still uses declaration + `errors.As` in `internal/store/write.go:218-228`, `internal/store/sqlite_corruption.go:47-58`, `internal/modelcatalog/errors.go:74-82`, and `internal/store/globaldb/sqlite_constraints.go:10-15`. Convert the simple cases; retain ordinary `errors.As` only where an addressable target is genuinely required. |
| `b.Loop` | `adopt` | Nearly every benchmark already uses it. `internal/resources/perf_bench_test.go:225-248` is the remaining `for idx := range b.N`; keep an explicit counter outside a `for b.Loop()` body. |
| JSON `omitzero` | `already` | Value `time.Time` watch payloads use `omitzero` in `internal/store/globaldb/global_db_task_watch_event_payloads.go:10-32`; optional time pointers correctly remain `omitempty`, e.g. `internal/store/session_liveness.go:23-49`. A mechanical tag rewrite would change API semantics. |
| `os.OpenRoot` / `os.Root` | `adopt` | Memory validates names lexically but reads through `os.ReadFile(path)` and stats joined paths (`internal/memory/store.go:123-152`, `internal/memory/store_scope.go:102-127`). A leaf symlink can still escape the intended root. Replace root path primitives with scoped directory capabilities and add symlink escape tests. |
| `strings.SplitSeq` / `FieldsSeq` / related string sequences | `adopt` | Good uses exist in `internal/modelcatalog/live_sources_command.go:98-104`, `internal/memory/batch_plan.go:136-153`, and `internal/store/migration_sql_parser.go:286-295`. `internal/modelcatalog/live_model_rows.go:165-185` still materializes all lines solely to iterate; use `SplitSeq` and an appropriate result capacity strategy. |
| `sync.WaitGroup.Go` | `adopt` | Settings already uses it in `internal/settings/mcp_collection_probes.go:25-53`. Production manual `Add`/`go`/`Done` remains in `internal/memory/consolidation/runtime.go:99-138`, `internal/memory/recall/signal_recorder.go:100-110`, and extractor launch sites such as `internal/memory/extractor/runtime_queue.go:161-184`. Convert while preserving the current rule that task registration happens before shutdown can call `Wait`. |
| Range over integer | `already` | Production and tests use it directly, e.g. `internal/store/globaldb/global_db_task_claim_helpers.go:232` and `internal/deadentity/service_test.go:227-259`. Remaining classic loops use the attempt/index value or non-unit conditions and should not be rewritten blindly. |
| `slices`, `maps`, built-in `min`/`max` | `already` | Representative uses include `internal/resources/validate.go:158-169`, `internal/modelcatalog/live_sources.go:146-155`, `internal/settings/mcp_secrets.go:203-225`, and `internal/memory/catalog_state.go:27-40`. No package-local compatibility helpers need to survive. |
| `testing/synctest` | `adopt` | Pure scheduler/timer tests still poll or sleep in `internal/memory/consolidation/runtime_test.go:165-189`, `internal/memory/consolidation/runtime_test.go:994-1005`, and `internal/store/sessiondb/session_db_extra_test.go:131-139`. Convert those canonical suites. Keep real HTTP timeout tests on real sockets, such as `internal/modelcatalog/modelsdev_test.go:200-228`, outside a synctest bubble. |
| `iter.Seq` / `iter.Seq2` | `defer` | Current persistence APIs intentionally return bounded pages/slices and own cursor close errors, e.g. `internal/store/sessiondb/transcript_query.go:90-176` and `internal/resources/kernel.go:362-408`. An iterator would move error and resource ownership into the consumer. Introduce only with a measured streaming use case and an explicit close/error contract. |
| `os.Process.WithHandle` | `not applicable` | The only production subprocess in the slice is `exec.CommandContext` with `Run` and captured output in `internal/modelcatalog/live_sources.go:101-137`; there is no direct cross-platform `Kill`/`Signal` handle lifecycle to wrap. |
| `sync.OnceFunc`, `OnceValue`, `OnceValues` | `adopt` | Result caches manually pair `sync.Once` with value/error fields in `internal/modelcatalog/service_refresh.go:155-182` and `internal/store/migration_plan.go:25-63`; both map directly to `OnceValues`. The release closure and lease error cache in `internal/store/sessiondb/read_only_pool.go:213-238` and `internal/store/sessiondb/read_only_pool.go:371-449` map to `OnceFunc`/`OnceValue`. |
| `math/rand/v2` | `reject` | Scoped randomness is security/correctness-sensitive: IDs and prompt tokens use `crypto/rand` (`internal/store/id.go:1-25`, `internal/store/globaldb/global_db_goal_prompt_common.go:1-25`), and SQLite retry jitter also deliberately draws from `crypto/rand` (`internal/store/write.go:194-216`). Do not downgrade these to non-cryptographic `rand/v2`. |
| `cmp.Or` | `reject` | Apparent defaults normally include trimming, validation, lazy time acquisition, enum normalization, or dependent fields, e.g. `internal/modelcatalog/service.go:322-348` and `internal/settings/config_apply_records.go:220-268`. `cmp.Or` would either evaluate fallbacks eagerly or hide domain normalization; no unambiguous scoped replacement justifies it. |
| `T.ArtifactDir`, `T.Attr`, `T.Output` | `not applicable` | The scoped tests use `t.TempDir` for disposable SQLite/filesystem isolation, such as `internal/store/workspacedb/workspace_db_test.go:18-99`; they do not own durable QA artifacts or test-run metadata. Adopt only when a test product contract requires retained evidence. |
| `http.CrossOriginProtection` | `not applicable` | The slice contains HTTP clients for model discovery (`internal/modelcatalog/live_sources_http.go:97-145`) but no HTTP server or handler boundary. This belongs to the HTTP/API owner. |
| `runtime/trace.FlightRecorder` | `not applicable` | The slice exposes benchmarks (for example `internal/resources/perf_bench_test.go:54-248`) but owns no daemon diagnostic capture surface. A flight recorder should be evaluated by the runtime/diagnostics package with retention and redaction policy. |
| Typed network `Dial` helpers | `not applicable` | Scoped network access is through `http.Client` in model discovery, not raw `net.Dial`/`DialContext` (`internal/modelcatalog/live_sources_http.go:97-145`). |
| `bufio.Buffer.Peek` | `not applicable` | Buffers here accumulate complete command or render output (`internal/modelcatalog/live_sources.go:117-133`, `internal/memory/checkpoint_summary.go:181-202`); no framed `bufio.Reader` parser needs lookahead. |
| `unique.Handle` / `unique.Make` | `reject` | Candidate values are persisted or user-controlled IDs with potentially unbounded cardinality, such as `store.DeadEntityKey` in `internal/store/types_dead_entity.go:23-53` and event identity fields in `internal/store/types_event.go:35-51`. Interning them would trade ordinary garbage collection for process-lifetime retention without profile evidence. |

## Relevant Sources

The following sources are the smallest set that explains the slice's important boundaries and the findings below:

1. `internal/store/write.go` — canonical immediate-write transaction, retry, rollback, and close ownership.
2. `internal/store/migration_bootstrap.go` — bootstrap transaction cleanup mismatch.
3. `internal/store/migration_plan.go` — immutable migration compilation cache and `OnceValues` candidate.
4. `internal/store/migrate_streams_test.go` — stream separation, sequencing, checksums, and schema equivalence.
5. `internal/store/globaldb/tx_helpers.go` — direct `sql.Tx` cleanup reference pattern.
6. `internal/store/globaldb/global_db_watch_events.go` plus `global_db_watch_events_{network,observe,loop,automation}.go` — repeated handwritten cursor ownership.
7. `internal/store/globaldb/queries/dead_entities.sql` — representative workspace tuple enforced in SQL.
8. `internal/store/globaldb/sqlc.yaml` and `internal/store/globaldb/sqlcgen/task_core.sql.go` — query source/generator boundary.
9. `internal/store/sessiondb/schema/schema.sql` — per-session schema without persisted owner identity.
10. `internal/store/sessiondb/session_db.go`, `session_event_write.go`, and `session_event_query.go` — in-memory session labeling and event validation.
11. `internal/store/sessiondb/read_only.go` and `read_only_pool.go` — read-only open, cursor cleanup, leasing, quiescence, and lock/I/O ownership.
12. `internal/store/workspacedb/workspace_db.go` and `workspace_db_test.go` — positive physical workspace isolation pattern.
13. `internal/resources/kernel.go`, `kernel_records.go`, and `validate.go` — actor-scoped desired-state queries and correct cursor cleanup.
14. `internal/resources/kernel_transaction.go` — transaction delegation plus unused predecessor cleanup helpers.
15. `internal/memory/store.go`, `store_scope.go`, `store_index.go`, and `mutation_raw.go` — file-root derivation, lexical name validation, and raw file mutation.
16. `internal/memory/store_catalog.go` and `catalog_identity.go` — workspace/agent catalog identity.
17. `internal/memory/catalog_rows.go`, `query_records.go`, and `decision_query.go` — good and bad cursor cleanup side by side.
18. `internal/memory/contract/types.go` — public backend context boundary.
19. `internal/memory/recall_source.go` and `internal/memory/recall/signal_recorder.go` — per-workspace async recorder ownership.
20. `internal/memory/consolidation/runtime.go` and `runtime_test.go` — background loop ownership and synctest candidates.
21. `internal/modelcatalog/service.go`, `service_refresh.go`, and `errors.go` — persistence error propagation, refresh coalescing, and typed errors.
22. `internal/modelcatalog/live_sources.go`, `live_sources_http.go`, and `live_model_rows.go` — process/HTTP boundaries and sequence parsing.
23. `internal/settings/config_apply_records.go` and `collections.go` — settings SQL cleanup and lost request context.
24. `internal/settings/mcp_secrets.go` and `mcp_catalog_notifications.go` — bounded rollback reference versus unbounded detached notification.
25. `internal/deadentity/service.go` and `events.go` — durable workspace state machine, mutex scope, cache lifetime, and event persistence.

## Transferable Patterns

1. **Make the owner visible in the function signature.** Cursor-returning helpers should either consume and close the cursor internally or return an explicit close/error protocol. Existing bounded slice APIs correctly keep ownership local. Apply the `resources`/memory-catalog named-error closer pattern to remaining handwritten queries instead of inventing package-specific logging behavior.

2. **Use split-phase state transitions around I/O.** Under a lock: validate state, reserve a version/token, detach the resource or mark an operation in flight. Outside the lock: open/close/query/persist. Under the lock again: publish the result only if the version still matches. This applies to `ReadOnlyPool`, dead-entity loads/transitions, and any future per-key state machine. It prevents both head-of-line blocking and callback re-entrancy deadlocks.

3. **Give every goroutine one cancel path and one completion signal.** The recall recorder should own a cancelable context and a stable `done` channel. `Close` should signal the owner and select on that channel directly; it should not create a waiter goroutine per call. Workspace-keyed workers require an explicit TTL/eviction or a bounded shared worker pool.

4. **Bound detached work.** `context.WithoutCancel` is appropriate for rollback or post-commit duties only when paired with a documented timeout, as already demonstrated by `internal/settings/mcp_secrets.go:233-238` and `internal/memory/catalog_sync_failure.go:20-36`. Introduce a small owner-specific helper rather than scattering bare `WithoutCancel`/`Background` calls.

5. **Persist identity at the storage boundary.** `workspacedb` proves that resolving an owner before open prevents accidental cross-owner reuse. `sessiondb` should persist an immutable identity row and verify it on every read/write open. If workspace isolation is part of the session contract, store both `session_id` and `workspace_id`; do not rely solely on a caller-computed path.

6. **Represent a filesystem root as a capability, not a string.** A validated basename plus `filepath.Join` blocks lexical traversal but not leaf symlink escape or directory replacement races. A small `memoryRoot` abstraction wrapping `os.Root` can own relative open/stat/create/remove/rename operations and make the security boundary reviewable.

7. **Keep generated boundaries declarative.** A schema or query contract change starts in `schema.sql`/schema definitions and `queries/*.sql`, receives the next append-only migration, and regenerates sqlc output. Generated close patterns are changed through the generator/version/template, never by editing `.sql.go` files.

8. **Modernize semantic boilerplate, not domain logic.** `errors.AsType`, `WaitGroup.Go`, `OnceValues`, `b.Loop`, string sequences, and `testing/synctest` remove mechanical state without hiding invariants. `cmp.Or`, `iter.Seq`, `unique`, and broad JSON-tag rewrites should wait for a concrete contract or profile.

9. **Delete superseded helpers and their coverage-only tests together.** Once `store.ExecuteWrite` owns resource transactions, old local rollback combinators are not useful safety nets. Keeping tests that are their only callers makes dead production code look live and obscures the canonical mechanism.

10. **Place regressions at the owning layer.** Before adding a test, name the invariant and reuse its canonical suite: session identity and pool ownership belong to existing `sessiondb` suites; recall worker lifecycle belongs to `internal/memory/recall/signal_recorder_test.go`; model status preservation belongs to `internal/modelcatalog/service_test.go`; resource cleanup belongs to `internal/resources/kernel_test.go`; settings cancellation/close behavior belongs to `internal/settings/service_test.go` or `config_apply_service_test.go`. Do not duplicate SQL cleanup invariants across every adapter when a shared helper/driver can prove them once.

## Risks / Mismatches

### Finding summary

| ID | Severity | Confidence | Finding |
| --- | --- | --- | --- |
| PM-01 | HIGH | HIGH | A session database file is not durably bound to its supplied session/workspace identity. |
| PM-02 | HIGH | HIGH | `ReadOnlyPool` performs open/close I/O while holding its global mutex. |
| PM-03 | HIGH | HIGH | Recall retains one non-cancelable worker per workspace and leaks waiter goroutines on timed-out close attempts. |
| PM-04 | HIGH | HIGH | Dead-entity methods hold per-key mutexes across store/event I/O; event persistence is detached without a timeout. |
| PM-05 | HIGH | HIGH | Memory and settings queries can return success after losing `Rows.Close` failures. |
| PM-06 | MEDIUM | HIGH | Several request/post-commit boundaries discard cancellation without a bounded replacement context. |
| PM-07 | MEDIUM | MEDIUM | Memory file reads are lexically confined but not symlink-confined; `os.Root` is warranted. |
| PM-08 | MEDIUM | HIGH | Migration bootstrap joins `sql.ErrTxDone` into a primary commit failure. |
| PM-09 | MEDIUM | HIGH | Model catalog silently discards failure while loading the prior successful status. |
| PM-10 | MEDIUM | HIGH | Long-lived keyed state has no eviction policy in recall and dead-entity registries. |
| PM-11 | LOW | HIGH | Superseded helpers, an unused SQL projection column, a dead conditional, and repeated watch close boilerplate remain. |
| PM-12 | LOW | HIGH | Two tests discard close errors and one subtest violates the canonical `Should …` shape. |

### PM-01 — session file identity is caller-asserted, not storage-attested

**Evidence.** `OpenSessionDB` stores the supplied ID only in the Go handle (`internal/store/sessiondb/session_db.go:71-105`). `OpenSessionDBReadOnly` validates that the ID and path are nonblank, opens the path, validates schema currency, and returns a handle with that ID; it never compares the ID with file contents (`internal/store/sessiondb/read_only.go:139-180`). The schema has no owner table or `session_id`/`workspace_id` column (`internal/store/sessiondb/schema/schema.sql:1-97`). Query scanning assigns `s.sessionID` to rows (`internal/store/sessiondb/session_event_query.go:46-70`), while write validation only compares incoming events with the same in-memory field (`internal/store/sessiondb/session_event_write.go:108-121`).

**Impact.** Reopening session A's path as session B can mix subsequent writes and causes reads of A's content to be labeled B. If path resolution ever crosses workspace boundaries, this becomes a workspace data disclosure. The package cannot independently prove the isolation invariant required of its data.

**Recommendation.** Add an immutable singleton session identity record containing at least `session_id`; include `workspace_id` if workspace ownership is part of the open contract. Seed it when creating a database and require an exact match on writable and read-only open. This is a greenfield hard cut: update the declarative session schema, append the next migration, update query sources, regenerate sqlc, and delete any path-only assumption—never patch generated `.sql.go`. Extend the existing session DB canonical suite with the invariant: “a file created for one session/workspace cannot be reopened under another identity.”

**Fowler technique.** Replace Primitive with Object (path + supplied ID becomes an attested database identity) and Introduce Assertion at the persistence boundary.

### PM-02 — read-only pool holds its mutex across I/O

**Evidence.** `Open` takes `p.mu`, invokes `closeExpiredLocked` (which closes recorders), and calls the injected opener before unlocking (`internal/store/sessiondb/read_only_pool.go:104-130`, `internal/store/sessiondb/read_only_pool.go:339-353`). `CloseExpired` directly calls the same close loop under the mutex (`internal/store/sessiondb/read_only_pool.go:267-278`). `Close` holds the mutex while closing every recorder (`internal/store/sessiondb/read_only_pool.go:280-316`). By contrast, `Quiesce` correctly detaches the recorder under the lock and closes it after unlock (`internal/store/sessiondb/read_only_pool.go:191-209`).

**Impact.** One slow SQLite open/close blocks every lease acquisition/release and pool shutdown. Because opener/recorder implementations are interfaces, a re-entrant callback can deadlock. Close latency scales under a global critical section.

**Recommendation.** Generalize the correct `Quiesce` pattern: detach expired/all entries and publish an “opening” reservation under lock; perform I/O outside; then reconcile the result under lock. Use a per-key in-flight state/channel so concurrent opens share one result without serializing unrelated sessions. Preserve `Add`-before-`Wait` ordering if `WaitGroup.Go` is introduced. The invariant belongs to `internal/store/sessiondb/session_db_integration_test.go`, which already exercises `ReadOnlyPool`: unrelated keys must progress while one injected opener/closer is blocked.

**Fowler technique.** Split Phase and Extract Function.

### PM-03 — recall worker lifetime is unbounded per workspace

**Evidence.** `recallSignalRecorder` retains one recorder in a shared map keyed by workspace and constructs it with `context.WithoutCancel(ctx)` (`internal/memory/recall_source.go:58-85`). The registry removes recorders only when all are closed (`internal/memory/store_options.go:91-121`). The worker calls persistence methods with the non-cancelable context (`internal/memory/recall/signal_recorder.go:230-263`). `Close` signals a stop channel but cannot interrupt in-flight I/O; every call creates another goroutine waiting on the same `WaitGroup`, which survives if the close context times out (`internal/memory/recall/signal_recorder.go:175-200`).

**Impact.** Workspace churn creates one retained map entry, queue, and goroutine per workspace until store shutdown. A stuck catalog write can make the worker immortal. Repeated shutdown attempts can accumulate waiter goroutines even though each caller times out.

**Recommendation.** Give the recorder an owned cancelable work context, a stable `done` channel closed by the worker, and a documented graceful-drain deadline followed by cancellation. Select on `done` directly in every `Close`; no waiter goroutine is needed. Add an idle eviction policy or replace per-workspace goroutines with a bounded shared worker pool carrying the workspace key in each job. `WaitGroup.Go` may simplify the single owned worker after lifecycle semantics are fixed. The invariant belongs to `internal/memory/recall/signal_recorder_test.go`: timeout, repeat close, blocked source, and workspace churn must leave a bounded goroutine/registry count.

**Fowler technique.** Replace Data Structure (per-key worker map to bounded lifecycle manager), Substitute Algorithm, and Remove Dead Code (`stopOnce` is redundant with the existing successful `CompareAndSwap`).

### PM-04 — dead-entity state locks cover persistence and unbounded event I/O

**Evidence.** `BeforeProbe` and `Status` hold `state.mu` while `ensureLoaded` may call `FindDeadEntity` (`internal/deadentity/service.go:73-115`, `internal/deadentity/service.go:241-260`). `RecordFailure` and `RecordSuccess` hold it while marking/clearing durable state and emitting transition events (`internal/deadentity/service.go:118-211`). `emitTransition` calls `WriteEventSummary(context.WithoutCancel(ctx), …)` without a timeout (`internal/deadentity/events.go:19-55`).

**Impact.** A slow store blocks every operation for that entity. An event sink that never returns can hold the key mutex indefinitely even after the caller cancels. Re-entrancy through an injected store/event implementation can deadlock. Because methods intentionally fail open, an indefinitely blocked call is especially inconsistent with the contract.

**Recommendation.** Use a versioned split phase: reserve/load/transition intent under the key lock, perform durable I/O with the caller context (or a bounded post-commit context), then publish the result if the version still matches. Emit events after releasing the key lock. Define one timeout for fail-open transition summaries. Add blocking fake-store/fake-event cases to the existing `internal/deadentity/service_test.go`; the invariant is “same-key state remains coherent, cancellation returns, and other calls cannot be held forever by an observer.”

**Fowler technique.** Split Phase and Encapsulate Variable.

### PM-05 — handwritten cursor cleanup loses close failures

**Evidence.** Dream-run, daily-log, and decision queries defer `closeRows` (`internal/memory/query_records.go:185-223`, `internal/memory/query_records.go:330-379`, `internal/memory/decision_query.go:55-85`). That helper only logs a close failure (`internal/memory/query_records.go:424-431`), so the caller can return success. The same package already has a correct join helper (`internal/memory/catalog_rows.go:9-17`). Settings `ListApplyRecords` uses bare `defer rows.Close()` (`internal/settings/config_apply_records.go:150-190`). A reusable close-error driver already proves this failure mode in `internal/store/globaldb/rows_close_error_test.go:15-111`.

**Impact.** The API reports a successful read even when the driver reports cursor finalization failure. Logging also gives the error a second, non-composable handling path and makes behavior logger-dependent.

**Recommendation.** Convert affected functions to named error returns and reuse/standardize the package's join-on-close helper. Remove the logging-only helper and now-unused `slog` import. Put each regression in its canonical query suite; if multiple packages need the same driver fault, extract the fixture to a shared test utility rather than copying the invariant.

**Fowler technique.** Replace Inline Code with Function Call and Consolidate Duplicate Conditional Fragments.

### PM-06 — cancellation is discarded at synchronous boundaries

**Evidence.** Settings carries a request context through provider settings and credential checks but drops it before `ClassifyDeclared`, substituting `context.Background()` (`internal/settings/collections.go:183-217`, `internal/settings/collections.go:251-270`). Post-commit task observers receive `context.WithoutCancel(ctx)` and run synchronously (`internal/store/globaldb/global_db_task_transaction.go:32-50`, `internal/store/globaldb/global_db_task_observer.go:20-48`). MCP install notification does the same without a deadline (`internal/settings/mcp_catalog_notifications.go:12-35`). Async resource event/health sinks receive `context.Background()` (`internal/resources/reconcile_pass.go:174-240`). The memory backend contract also lacks context on `List`, `Read`, `Write`, `Delete`, and `LoadPromptIndex`, forcing production methods to create `context.Background()` for database/event side effects (`internal/memory/contract/types.go:180-190`, `internal/memory/store.go:155-166`, `internal/memory/store_catalog.go:182-217`).

**Impact.** Cancellation and deadlines do not bound synchronous probes/callbacks; a mutation may be durable but its caller can hang forever in post-commit fanout. Memory operations cannot carry trace/deadline/workspace values through their public interface.

**Recommendation.** Pass the caller context to genuinely synchronous work. For deliberately durable post-commit work, use an owner-specific bounded context derived from `WithoutCancel` and define whether timeout is returned, diagnosed, or queued. Hard-cut the memory `Backend` interface to context-aware methods and update all adapters/callers together; do not add parallel legacy methods. Canonical suites should inject a blocking implementation and prove the documented deadline behavior.

**Fowler technique.** Change Function Declaration and Extract Function (bounded post-commit context construction).

### PM-07 — lexical memory paths do not contain symlinks

**Evidence.** `cleanPathSegment` and `cleanFilename` reject absolute paths, dot segments, NULs, and separators (`internal/memory/store_scope.go:102-127`, `internal/memory/store_index.go:171-186`). `Read` and `Exists` then use ordinary path-based `os.ReadFile`/`os.Stat` (`internal/memory/store.go:123-152`); writes and deletes also resolve string paths before mutation (`internal/memory/mutation_raw.go:15-85`, `internal/memory/mutation_raw.go:122-174`). Tests cover lexical traversal but no symlink leaf escape (`internal/memory/store_test.go:160-196`, `internal/memory/store_test.go:2111-2131`).

**Impact.** A symlink with an allowed filename inside a memory directory can make reads/stat follow a target outside the intended scope. Whether an untrusted extension or local actor can create that symlink is outside this slice, hence MEDIUM confidence on exploitability, but the containment property is not enforced by the storage layer.

**Recommendation.** Open each global/workspace/agent directory as an `os.Root` capability and perform relative operations through it. Preserve atomic write semantics with a temporary file and rename within the same root. Add leaf-symlink, intermediate-symlink, root-replacement, and normal atomic-write cases to the existing `internal/memory/store_test.go` suite.

**Fowler technique.** Replace Primitive with Object (root path string to a `memoryRoot` capability) and Extract Class.

### PM-08 — bootstrap rollback pollutes primary errors with `sql.ErrTxDone`

**Evidence.** On any non-nil return after `BeginTx`, migration bootstrap unconditionally joins `tx.Rollback()` (`internal/store/migration_bootstrap.go:139-184`). A failed `Commit` may leave the transaction already done, making rollback return `sql.ErrTxDone`. Other direct transaction helpers intentionally ignore only that sentinel (`internal/store/globaldb/tx_helpers.go:26-34`, `internal/store/sessiondb/transcript_query.go:96-104`).

**Impact.** Callers receive an unrelated cleanup sentinel joined to the actual bootstrap/commit failure, complicating classification and diagnostics.

**Recommendation.** Filter `sql.ErrTxDone`, wrap any real rollback error with operation context, and join it with the primary error. Extend the existing migration engine suite—not a new static/prose test—with a driver/fake path that proves primary and real rollback errors are both retained while `ErrTxDone` is suppressed.

**Fowler technique.** Replace Inline Code with Function Call using one canonical transaction-cleanup operation.

### PM-09 — model catalog silently loses a persistence read error

**Evidence.** Stale failure persistence calls `preserveLastSuccess` before writing status (`internal/modelcatalog/service.go:184-224`, `internal/modelcatalog/service_refresh.go:258-277`). `preserveLastSuccess` discards `ListSourceStatus` errors by returning silently (`internal/modelcatalog/service.go:374-386`).

**Impact.** A store read failure can cause the next failed-source status to overwrite or omit the real `LastSuccess`, degrading freshness/health truth while the higher-level operation appears successful. There is no diagnostic indicating the state loss.

**Recommendation.** Return an error from `preserveLastSuccess` and propagate it through the refresh persistence path. If product policy explicitly prefers fail-open status persistence, make that policy visible by returning a diagnostic alongside the result; do not silently erase the failure. Add the invariant to `internal/modelcatalog/service_test.go`: a prior-status read failure must be observable and must not publish a falsely complete status.

**Fowler technique.** Change Function Declaration.

### PM-10 — keyed state registries have no bounded lifetime

**Evidence.** The recall registry retains an entry per workspace until global close (`internal/memory/store.go:64-67`, `internal/memory/store_options.go:103-121`). `deadentity.Service.stateFor` inserts a state for every valid tuple and never deletes one, including after successful clear (`internal/deadentity/service.go:230-238`). Entity IDs are externally keyed strings across extension, bridge, and MCP sidecar families (`internal/store/types_dead_entity.go:13-53`).

**Impact.** A long-lived daemon accumulates heap state under workspace/entity churn; recall additionally accumulates active goroutines and queues as covered by PM-03.

**Recommendation.** Define cardinality and lifetime as part of each owner contract. Evict dead-entity entries after a successful clear once no operation owns the state; use a bounded/TTL cache if repeated lookups justify caching. Make recall worker lifecycle explicitly workspace-owned or globally bounded. Tests should assert eviction through public lifecycle behavior rather than map-length implementation details unless bounded registry size is itself the contract.

**Fowler technique.** Encapsulate Collection and Extract Class (lifecycle-aware registry).

### PM-11 — dead and duplicated implementation remains

**Evidence.** `rollbackTx`, `rollbackImmediate`, and `joinCleanupError` in `internal/resources/kernel_transaction.go:13-42` have no production callers; their only callers are coverage-oriented tests in `internal/resources/kernel_test.go:991-995` and `internal/resources/kernel_test.go:1145-1162`. The live transaction path delegates to `store.ExecuteWrite` (`internal/resources/kernel_transaction.go:44-55`). Session hook queries select/scan `rowid` and immediately discard it (`internal/store/sessiondb/session_hook_runs.go:140-200`). `globalHomeFromMemoryDir` has both branches return the same expression (`internal/memory/store_scope.go:87-100`). Handwritten watch readers both defer `rows.Close()` and explicitly close/join on every return path, for example `internal/store/globaldb/global_db_watch_events.go:277-320` and `internal/store/globaldb/global_db_watch_events_network.go:43-125`.

**Impact.** Dead helpers and tests obscure the actual transaction owner; unused projection work and dead conditions add noise; duplicate close ownership makes future branches easy to get wrong. The watch behavior is currently functionally safe because `sql.Rows.Close` is idempotent, so this is LOW severity rather than a correctness defect.

**Recommendation.** Delete the superseded resource helpers and their helper-only tests. Remove `rowid` from hook-run SELECT projections/scans. Simplify the identical branch. Give watch readers one named-error deferred closer and remove branch-by-branch close calls. Generated sqlc files with similar output remain generator-owned.

**Fowler technique.** Remove Dead Code, Simplify Conditional Logic, and Extract Function.

### PM-12 — test cleanup and naming drift

**Evidence.** `internal/store/sessiondb/session_db_extra_test.go:189-207` registers `t.Cleanup(func() { _ = db.Close() })`; `internal/store/globaldb/global_db_task_test.go:4031-4040` discards `rows.Close`. `internal/store/globaldb/global_db_task_test.go:560-564` names a subtest `ShouldMapChildConstraintFailuresToTaskValidationErrors` rather than the required `Should …` sentence form.

**Impact.** A cleanup regression can be hidden by the test that should reveal it, and naming drift weakens suite consistency. These are isolated test-quality failures.

**Recommendation.** Report cleanup errors with `t.Errorf` in cleanup callbacks, preserving an earlier `Fatal` result, and rename the existing subtest in place. Do not add standalone tests for these implementation details.

**Fowler technique.** Replace Inline Code with Function Call (canonical test cleanup helper where repeated) and Rename Variable/Test.

## Open Questions

1. **Session identity authority:** Is it an intentional architecture rule that only the upstream path resolver binds a session database to its session/workspace, or should `sessiondb` attest `session_id` and `workspace_id` itself? The current package contract cannot detect a mismatched path/ID pair.
2. **Memory backend hard cut:** May the `memcontract.Backend` interface be changed in one breaking update so every filesystem/catalog operation accepts `context.Context`? A compatibility layer would conflict with the greenfield policy and preserve the current cancellation hole.
3. **Worker/cache lifetime:** What is the intended lifetime and maximum cardinality of per-workspace recall recorders and per-entity dead-state entries? The code currently implies process lifetime, but no owner contract or bound states that choice.
4. **Post-commit delivery semantics:** Must task observers, marketplace install notifications, dead-entity summaries, and resource health/event sinks complete synchronously before the initiating API returns, or may they enter a bounded durable queue? The correct timeout/error policy depends on this decision.
5. **Filesystem trust boundary:** Are memory directories writable by extensions, agents, shared workspaces, or any actor less trusted than the daemon? If yes, `os.Root` conversion should be treated as a security fix; if no, it remains defense-in-depth but still gives a stronger containment invariant.

## Evidence

### Audit basis

- Updated Go doctrine: `/home/pedronauck/Projects/compozy/.agents/skills/golang-master/SKILL.md` and `/home/pedronauck/Projects/compozy/.agents/skills/golang-master/references/modernize.md`, `concurrency.md`, `context.md`, `errors.md`, `safety.md`, `testing.md`, and `performance.md`.
- Cleanup doctrine: `/home/pedronauck/Projects/compozy/.agents/skills/eng/eng-cleanup-failure-paths/SKILL.md` and `/home/pedronauck/Projects/compozy/.agents/skills/eng/eng-cleanup-failure-paths/references/cleanup-table.md`.
- Schema doctrine: `/home/pedronauck/Projects/compozy/.agents/skills/eng/eng-schema-migration/SKILL.md`.
- Fowler smell/refactoring catalogs: `/home/pedronauck/Projects/compozy/.agents/skills/architectural-analysis/references/detection-catalog.md` and `/home/pedronauck/Projects/compozy/.agents/skills/refactoring-analysis/references/refactoring-techniques.md`.
- Supplied Go feature baseline: `/home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt`.

### Concrete code evidence

- Session identity mismatch: `internal/store/sessiondb/schema/schema.sql:1-97`; `internal/store/sessiondb/session_db.go:71-105`; `internal/store/sessiondb/read_only.go:139-180`; `internal/store/sessiondb/session_event_query.go:46-70`; `internal/store/sessiondb/session_event_write.go:108-121`.
- Pool lock/I/O overlap: `internal/store/sessiondb/read_only_pool.go:104-130`; `internal/store/sessiondb/read_only_pool.go:191-209`; `internal/store/sessiondb/read_only_pool.go:267-316`; `internal/store/sessiondb/read_only_pool.go:339-353`.
- Recall lifecycle: `internal/memory/recall_source.go:58-85`; `internal/memory/store_options.go:91-121`; `internal/memory/recall/signal_recorder.go:100-110`; `internal/memory/recall/signal_recorder.go:175-251`.
- Dead-entity lock and detached event: `internal/deadentity/service.go:73-211`; `internal/deadentity/service.go:230-279`; `internal/deadentity/events.go:19-55`.
- Cursor cleanup mismatch/reference: `internal/memory/query_records.go:185-223`; `internal/memory/query_records.go:330-431`; `internal/memory/decision_query.go:55-85`; `internal/memory/catalog_rows.go:9-17`; `internal/settings/config_apply_records.go:150-190`; `internal/store/globaldb/rows_close_error_test.go:15-111`.
- Bounded versus unbounded detached context: `internal/settings/mcp_secrets.go:233-238`; `internal/memory/catalog_sync_failure.go:20-36`; `internal/settings/mcp_catalog_notifications.go:12-35`; `internal/store/globaldb/global_db_task_transaction.go:32-50`; `internal/resources/reconcile_pass.go:174-240`.
- Filesystem containment: `internal/memory/store_scope.go:102-127`; `internal/memory/store_index.go:171-186`; `internal/memory/store.go:123-166`; `internal/memory/mutation_raw.go:15-174`; `internal/memory/store_test.go:160-196`; `internal/memory/store_test.go:2111-2131`.
- Transaction cleanup: `internal/store/write.go:134-188`; `internal/store/migration_bootstrap.go:139-184`; `internal/store/globaldb/tx_helpers.go:26-97`; `internal/store/sessiondb/transcript_query.go:90-107`.
- Scope/isolation strengths: `internal/store/workspacedb/workspace_db.go:25-52`; `internal/store/workspacedb/workspace_db_test.go:67-99`; `internal/resources/kernel.go:330-408`; `internal/resources/validate.go:119-183`; `internal/store/globaldb/queries/dead_entities.sql:1-28`; `internal/deadentity/service_test.go:251-278`.
- Memory identity: `internal/memory/store_catalog.go:23-104`; `internal/memory/catalog_identity.go:13-114`; `internal/memory/contract/types.go:180-190`.
- Model catalog error/cache modernization: `internal/modelcatalog/service.go:184-224`; `internal/modelcatalog/service.go:374-386`; `internal/modelcatalog/service_refresh.go:155-182`; `internal/modelcatalog/service_refresh.go:258-318`; `internal/modelcatalog/errors.go:74-82`.
- Generated/schema boundary: `internal/store/globaldb/sqlc.yaml:1-12`; `internal/store/sessiondb/sqlc.yaml:1-12`; `internal/store/globaldb/sqlcgen/task_core.sql.go:1-12`; `internal/store/migrate_streams_test.go:39-118`; `internal/store/migrate_streams_test.go:319-373`.
- Dead/duplicated code and test drift: `internal/resources/kernel_transaction.go:13-55`; `internal/resources/kernel_test.go:991-995`; `internal/resources/kernel_test.go:1145-1162`; `internal/store/sessiondb/session_hook_runs.go:140-200`; `internal/memory/store_scope.go:87-100`; `internal/store/globaldb/global_db_watch_events.go:277-320`; `internal/store/sessiondb/session_db_extra_test.go:189-207`; `internal/store/globaldb/global_db_task_test.go:560-564`; `internal/store/globaldb/global_db_task_test.go:4031-4040`.

### Limitations

- This was a static, read-only source audit. Findings about definite control flow and ownership are HIGH confidence; exploitability of the memory symlink gap is MEDIUM confidence because actor access to the directories is owned outside the authorized slice.
- No runtime trace, race run, fault-injected database execution, benchmark, or generated-drift check was permitted in this explorer dispatch. The recommended canonical tests are the next evidence step; they should validate production fixes rather than weaken expectations.
- Upstream HTTP/CLI/UDS path resolution and workspace propagation were outside this slice. PM-01 therefore proves that `sessiondb` itself does not attest identity; a parent analysis must determine whether any upstream layer currently prevents every mismatched path/ID call.
