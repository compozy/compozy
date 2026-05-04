# Storage / SQLite — Performance Analysis

_Scope_: `internal/store/{sqlite.go, schema.go, store.go, values.go, globaldb/*, rundb/*}` and cross-cutting callers in `internal/daemon` and `internal/core/run/journal`.
_Method_: static read of schema, SQL strings, call-sites; reasoned from SQLite/modernc semantics. No runtime profile was taken — findings are qualitative but each links to a concrete file/line and a predictable behavior.

---

## Summary

Daemon storage is two SQLite databases opened via `modernc.org/sqlite`: a single `global.db` (catalog/runs index) shared by the daemon and a per-run `run.db` opened on every run by the journal. Journal writes are the hot write path (every event batch → one SQLite tx + one jsonl `fsync`). Global-DB reads are dominated by run lists and per-run `GetWorkflow` amplification. The biggest wins are:

1. Tune SQLite pragmas (`cache_size`, `mmap_size`, `temp_store`, `wal_autocheckpoint`) and make the writer-pool single-connection — current defaults leave ~5-10× headroom on both cold reads and WAL throughput.
2. Drop dead indexes on `rundb.events`, `rundb.job_state`, `rundb.token_usage`, `rundb.hook_runs`, `rundb.transcript_messages`, `rundb.artifact_sync_log` — they are never read but add I/O to every write in the hot journal loop.
3. Stop using `LOWER(TRIM(status))` in every hot `runs` query (archive, active-run counts, purge, interrupted runs). This defeats `idx_runs_workspace_status` forcing a full scan on a table that grows monotonically. Normalize status on write.
4. Kill the `GetWorkflow`-per-row N+1 in `RunManager.List` (`internal/daemon/run_manager.go:377–390` + `:1282`), and remove per-row `DeleteRun`/`UpdateRun` loops in purge/reconcile by using `IN (…)` batch deletes or transactional batches.
5. Batch projection upserts inside `StoreEventBatch` — currently up to 5 statements per event, each re-parsed by Go `database/sql`. Use `sql.Stmt` prepared once per batch.

---

## SQLite Configuration Audit

File: `internal/store/sqlite.go:88-123`, `internal/store/store.go:5-12`.

| Pragma | Current | Recommended | Rationale |
|---|---|---|---|
| `journal_mode` | **WAL** | keep WAL | Correct; necessary for concurrent readers + single writer. |
| `synchronous` | **NORMAL** | keep NORMAL (or document) | Fine for WAL. FULL would double write latency. |
| `busy_timeout` | **5000 ms** | 5000–10000 | OK. Hides pool contention; see below. |
| `foreign_keys` | **ON** | keep ON | Correct. |
| `cache_size` | **unset (default ≈ 2000 pages ≈ 8 MiB)** | `-20000` (≈ 20 MiB) or `-65536` (≈ 64 MiB) | Cold reads of `events` / projection tables page-fault per row. |
| `mmap_size` | **unset (0)** | `268435456` (256 MiB) | Eliminates `read(2)` syscalls on the warm path and lets the OS page-cache back SQLite. |
| `temp_store` | **unset (= FILE)** | `MEMORY` | Sort/merge spills (e.g., hypothetical `ORDER BY timestamp` scans) stay in RAM. |
| `wal_autocheckpoint` | **default (1000 pages)** | `10000` + periodic `Checkpoint` | Daemon already has `Checkpoint(TRUNCATE)`; raising auto-threshold keeps writers from stalling. |
| `page_size` | **default 4096** | leave | Fine. Only settable before first write. |
| `auto_vacuum` | **NONE** | keep NONE | Incremental vacuum is unnecessary; `purge` removes rows but runs.db are small. |

Pool sizing: `defaultMaxOpenConns = 8`, `defaultMaxIdleConns = 8` in `internal/store/store.go:7-9`. Every `sql.DB` operation can grab any of 8 connections. Under WAL only one can hold the reserved/exclusive lock at a time, so writer concurrency ≥ 2 collides and falls back to `busy_timeout` (up to 5 s silent stall). For a store this small, we want:

- **Writer pool**: one `*sql.DB` restricted to `SetMaxOpenConns(1)` used for all writes.
- **Reader pool**: same file opened via a separate `*sql.DB` with `SetMaxOpenConns(runtime.NumCPU())` for parallel reads.

As written, `sqlite.OpenSQLiteDatabase` is also used for the per-run `run.db` where the journal is the _only_ writer; pool size 8 buys nothing but allocates up to 8 file handles.

---

## Schema / Index Audit

### global.db (`internal/store/globaldb/migrations.go:20-127`)

| Table | Indexes declared | Query patterns observed | Gaps |
|---|---|---|---|
| `workspaces` | `uq_workspaces_root_dir`, `idx_workspaces_name` | PK lookup by id, root_dir lookup, `ORDER BY root_dir ASC, id ASC` (`registry.go:192-196`) | — |
| `workflows` | `idx_workflows_workspace`, `idx_workflows_workspace_slug`, partial `uq_workflows_active_slug WHERE archived_at IS NULL` | PK, `(workspace_id, slug)` active lookup, `ORDER BY archived_at DESC, created_at DESC, id DESC` with `WHERE archived_at IS NOT NULL` (`archive.go:218-225`) | Latest-archived query is a partial scan. A covering partial index `… WHERE archived_at IS NOT NULL` would speed it but cardinality is tiny — P2. |
| `artifact_snapshots` | `idx_artifact_snapshots_checksum`, PK `(workflow_id, artifact_kind, relative_path)` | `WHERE workflow_id = ?` scan (`sync.go:358-364`) | PK is workflow-prefixed ⇒ already covers the scan. `idx_artifact_snapshots_checksum` has **no reader**; write-side cost. |
| `task_items` | `uq_task_items_workflow_number`, `uq_task_items_workflow_task_id` | `WHERE workflow_id = ?` and count by status (`archive.go:168-176`), upserts via unique index | — |
| `review_rounds` | `uq_review_rounds_workflow_round` | latest-round (`ORDER BY round_number DESC LIMIT 1`), `SUM(unresolved_count) WHERE workflow_id = ?` (`archive.go:177`) | Unique key already covers workflow_id prefix ⇒ OK. |
| `review_issues` | `uq_review_issues_round_issue` | list by round_id (`reviews.go:95-101`) | — |
| `runs` | `idx_runs_workspace_started`, `idx_runs_workspace_status` | `WHERE LOWER(TRIM(status)) IN (…)` (archive counts, list-interrupted, list-terminal-purge, count-active-runs, unregister-check); `WHERE workflow_id = ?` in archive eligibility (`archive.go:180-183`) | **P0**: `LOWER(TRIM(status))` bypasses `idx_runs_workspace_status`. **P1**: no index on `runs.workflow_id`. |
| `sync_checkpoints` | PK only | upsert only | — |
| `schema_migrations` | PK | startup read | — |

### run.db (`internal/store/rundb/migrations.go:20-83`)

| Table | Indexes declared | Query patterns observed | Gaps |
|---|---|---|---|
| `events` | PK `sequence`; `idx_events_kind`, `idx_events_timestamp`, `idx_events_job_id` | `MAX(sequence)` (PK optimization), `WHERE sequence >= ? ORDER BY sequence ASC`, last event `ORDER BY sequence DESC LIMIT 1` | **P0**: All 3 secondary indexes are **dead** — no query filters by kind, timestamp, or job_id. They cost ~3 extra B-tree writes per event on the hottest write path. |
| `job_state` | PK `job_id`; `idx_job_state_status` | `ORDER BY job_id ASC` only | **P1**: `idx_job_state_status` is dead. |
| `transcript_messages` | PK `sequence`; `idx_transcript_messages_timestamp` | `ORDER BY sequence ASC` only | **P1**: timestamp index is dead. |
| `hook_runs` | PK `id`; `idx_hook_runs_recorded_at` | `ORDER BY recorded_at ASC, id ASC` | This index **is** used for the ORDER BY. Keep. |
| `token_usage` | PK `turn_id`; `idx_token_usage_timestamp` | `ORDER BY timestamp ASC, turn_id ASC` | This index **is** used for the ORDER BY. Keep. |
| `artifact_sync_log` | PK `sequence`; `idx_artifact_sync_log_path` | `ORDER BY sequence ASC` only | **P1**: path index is dead. |

Net: of 10 declared secondary indexes in `rundb`, **5 are used by zero queries**, and the journal pays the cost on every single event.

### JSON vs typed columns

- `events.payload_json TEXT`, `job_state.summary_json TEXT`, `transcript_messages.metadata_json TEXT`, `task_items.depends_on_json TEXT` — stored as JSON text. None of them are filtered or indexed on JSON paths. This is correct (no `json_extract` pressure). The cost is pure bytes.
- `artifact_snapshots.body_text` can be up to 256 KiB and is checked via `CHECK (length(body_text) <= 262144)` (`migrations.go:59`). CHECK runs on every insert/upsert; trivial on upsert cadence.

### Text timestamps

- `store.FormatTimestamp` / `store.ParseTimestamp` (`internal/store/values.go:12-27`) serialize every `time.Time` to RFC3339-ish text (`"2006-01-02T15:04:05.000000000Z"`). Every read path calls `time.Parse(layout, …)` per row. For the 1000-row event page and transcript replay scenarios this is non-trivial Go CPU. Integer `INTEGER strftime('%s','now')` or Unix-nanos would be ~10× cheaper to marshal — **P2** (defer; requires migration).

---

## Query Hot Paths

Ranked by (expected frequency × per-call cost):

| # | Hot path | Site | Cost driver |
|---|---|---|---|
| 1 | Journal batch write | `rundb.StoreEventBatch` (`run_db.go:143-177`) + `storeProjectedEvent` (`:641-658`) | 1 tx + up to 5 statements per event + 5 dead indexes = ~5–10× write amplification on `events`. |
| 2 | Run list rendering | `RunManager.List` → `toCoreRun` → `GetWorkflow` per row (`run_manager.go:377-390`, `:1282`) | N+1 round-trip; 100 runs = 101 queries. |
| 3 | Snapshot / replay | `runDB.ListEvents(0)` + `ListTranscriptMessages` + `ListTokenUsage` (`run_manager.go:422-436`) | Full scan of events table; per-row JSON and timestamp parse. |
| 4 | Active-run / purge / interrupted scans | `globaldb/runs.go:37-46, 56-83, 164-192`, `archive.go:168-205` | `LOWER(TRIM(status))` prevents index use → full table scan of `runs`. |
| 5 | Sync reconciliation | `globaldb/sync.go:273-898` (artifact_snapshots, task_items, review_rounds, review_issues) | N statements per table per reconcile, all prepared fresh. |
| 6 | Workspace/workflow resolve | `Resolve`/`getWorkspaceByRootDir` (`registry.go:111-183`) | Fast (unique index) but called on every CLI action. |
| 7 | Purge loop | `shutdown.go:109-127` | per-row `DeleteRun` + `os.RemoveAll` — each row is its own transaction. |
| 8 | Reconcile loop | `reconcile.go:137-160` | per-row `MarkRunCrashed` and synthetic append. |

### Plan reasoning highlights

- **`runs` with `LOWER(TRIM(status)) IN (…)`**: SQLite can only use an index on `status` if the WHERE clause applies a pure equality on the indexed column. Wrapping in functions forces a full table scan. With `defaultKeepMax = 200` and a `runs` table that accumulates across days, this may still be hundreds to thousands of rows per query.
- **`events WHERE sequence >= ?`**: PK range scan → optimal. But there is no `LIMIT` (`run_db.go:273`). `Events()` paginates in Go, so we transfer the whole tail of the run's events then discard. For long-running runs this moves ~O(events) bytes on each page.
- **`MAX(sequence) FROM events`**: PK → O(1) in SQLite via rightmost leaf.
- **Archive eligibility** (`archive.go:166-205`): one `SELECT` with 5 correlated `COUNT(1)`/`SUM()` subqueries, each scanning a workflow-filtered table. Of those, the last two (`FROM runs WHERE workflow_id = ?` and the status filter) scan full `runs` because there is no `idx_runs_workflow_id`.

---

## P0 Findings

### P0-1. Dead secondary indexes on rundb write-hot tables

**File/line**: `internal/store/rundb/migrations.go:33-35, 44, 53-54, 73, 81`.
**Pattern**: 5 indexes (`idx_events_kind`, `idx_events_timestamp`, `idx_events_job_id`, `idx_job_state_status`, `idx_transcript_messages_timestamp`, `idx_artifact_sync_log_path`) have **no reader** anywhere in the repo (confirmed by grep). The writer (journal batch) pays the B-tree insert cost on every event.
**Why it hurts**: `events` is inserted once per journaled event. Each secondary index is an extra B-tree insert (log_N page writes). 3 dead indexes on `events` ≈ 3× extra I/O per event. Under WAL this lands in the `-wal` file before checkpoint — dominating write throughput for this process.
**Proposed fix**: Add migration `v2`: `DROP INDEX IF EXISTS idx_events_kind;` + same for the other 5 indexes. If future queries need them, add them back with a clear reader.
**Expected win**: ~30–50 % fewer B-tree ops per event insert on `events`; journal flush `StoreEventBatch` latency should drop noticeably (back-of-envelope: 4 → 1 index write per row on a 4-column PK-only table).

### P0-2. `LOWER(TRIM(status))` defeats the status index on `runs`

**Files/lines**:
- `internal/store/globaldb/runs.go:42` (count active runs)
- `internal/store/globaldb/runs.go:61` (list interrupted runs)
- `internal/store/globaldb/runs.go:170` (list terminal for purge)
- `internal/store/globaldb/archive.go:174, 182` (archive eligibility)
- `internal/store/globaldb/registry.go:935` (count active per workspace)
**Pattern**: `WHERE LOWER(TRIM(status)) NOT IN ('completed', 'failed', 'cancelled', 'canceled', 'crashed')`.
**Why it hurts**: SQLite needs a pure column reference to use `idx_runs_workspace_status`. Wrapping in `LOWER`/`TRIM` forces `SCAN TABLE runs`. As `runs` grows (100s–1000s with default `KeepMax=200`) every status check scales linearly.
**Proposed fix**:
1. Normalize on write: trim + lowercase in `PutRun` / `UpdateRun` (`registry.go:358-481`), so the column is always canonical.
2. Replace query predicates with `WHERE status IN ('completed','failed','cancelled','canceled','crashed')` / `status NOT IN (...)` — a direct equality set that SQLite can plan via the index.
3. Add `CHECK (status = LOWER(TRIM(status)))` to keep the invariant enforced.
**Expected win**: index-seek vs full-scan; O(log N) vs O(N). For active-run counts this is the difference between <100 µs and N×microseconds per row.

### P0-3. N+1: `GetWorkflow` per run in `RunManager.List`

**File/line**: `internal/daemon/run_manager.go:377-390` calls `m.toCoreRun` for each row; `toCoreRun` (`:1262-1289`) calls `m.globalDB.GetWorkflow(ctx, workflowID)`.
**Pattern**: Listing 100 runs issues 101 round-trips (1 list + 100 single-row workflow fetches). Each round-trip is a prepared-stmt compile + exec + scan.
**Why it hurts**: Even at sub-ms per query this is 100 serial waits, with pool contention (see pool note below). CLI `runs list` becomes latency-bound.
**Proposed fix**: Join once:

```sql
SELECT r.run_id, r.workspace_id, r.workflow_id, r.mode, r.status, r.presentation_mode,
       r.started_at, r.ended_at, r.error_text, r.request_id,
       w.slug AS workflow_slug
FROM runs r
LEFT JOIN workflows w ON w.id = r.workflow_id
WHERE r.workspace_id = ? AND r.status = ? AND r.mode = ?
ORDER BY r.started_at DESC, r.run_id ASC
LIMIT ?;
```

Then `toCoreRun` becomes a pure mapping. Or keep current API and add `ListRunsWithWorkflow` for callers that need the slug.
**Expected win**: 100 serial queries → 1. p95 of `runs list` collapses from O(100 × RTT) to O(1 × RTT).

---

## P1 Findings

### P1-1. Per-row projection upserts inside `StoreEventBatch`

**File/line**: `internal/store/rundb/run_db.go:166-169`, `:641-658`, `:692-775`.
**Pattern**: For a batch of 32 events, the function loops and calls up to 5 `tx.ExecContext` per event (insert event + up to 4 projection upserts). `database/sql` re-prepares each statement by default unless `tx.Prepare` is used.
**Why it hurts**: 32 events × 5 statements = 160 prepare+bind+exec cycles per batch. Under `modernc.org/sqlite`, each `ExecContext` parses the SQL string and binds parameters.
**Proposed fix**:
1. Create a `prepared` struct holding `*sql.Stmt` for each SQL (events insert, 4 upserts) at `Open` time, or lazily on first use.
2. Within `StoreEventBatch`, `tx.Stmt(prepared.xxx).ExecContext(...)` reuses the parsed plan.
3. Alternative: collect all pending events then issue one multi-row `INSERT INTO events VALUES (?,?,?,?,?,?),(?,?,?,?,?,?),…` — at most ~800 bytes per row; 32 rows is well below SQLite's 1 MB parameter cap (999 bind vars means up to ~166 events per statement at 6 cols).
**Expected win**: 30–60 % reduction in CPU per batch flush; fewer parser allocations.

### P1-2. Per-row `DeleteRun` in purge

**File/line**: `internal/daemon/shutdown.go:109-127`.
**Pattern**: `for i := range candidates { ... m.globalDB.DeleteRun(listCtx, run.RunID) ... }`. Each call is a separate transaction (auto-commit).
**Why it hurts**: N fsyncs in series. Under WAL each commit = fsync of `-wal`.
**Proposed fix**:
1. After filesystem cleanup succeeds for a batch, issue `DELETE FROM runs WHERE run_id IN (?, ?, …)` with up to ~500 ids per statement.
2. Or wrap the whole loop in a `BeginTx`/`Commit` so all deletes share one fsync. Directory removal failures still stop the loop — collect the successful ids and delete them at the end.
**Expected win**: purge of 200 rows goes from ~200 fsyncs to 1 fsync (~100 ms → ~2 ms on SSD).

### P1-3. Reconcile `MarkRunCrashed` per row

**File/line**: `internal/daemon/reconcile.go:137-160`.
**Pattern**: `for i := range interrupted { ... db.MarkRunCrashed(ctx, row.RunID, …) ... }`. `MarkRunCrashed` internally does `GetRun` + `UpdateRun` (two queries).
**Why it hurts**: 3 round-trips per interrupted run (GetRun, UpdateRun, and the synthetic append into rundb). On crash recovery this runs before the daemon reports ready — adds to cold-start time.
**Proposed fix**:
1. `UPDATE runs SET status = 'crashed', ended_at = ?, error_text = ? WHERE run_id IN (?, ?, …) AND LOWER(status) IN ('starting','running')` (once P0-2 is fixed the `LOWER` is unnecessary).
2. Keep the rundb synthetic append per run (it writes to a different file), but move the catalog update out of the loop.
**Expected win**: daemon readiness time under recovery scales to O(#interrupted) fsyncs → O(1). For 50 interrupted runs that's ~50× faster recovery.

### P1-4. `ListEvents` returns the whole stream

**File/line**: `internal/store/rundb/run_db.go:271-275`, caller `run_manager.go:498`.
**Pattern**: `SELECT … FROM events WHERE sequence >= ? ORDER BY sequence ASC` without `LIMIT`. Caller filters + trims to `limit` in Go (`run_manager.go:503-525`).
**Why it hurts**: For a 10k-event run paginated at 1k per page, every page scans + transfers all remaining rows (first page: 10k; second: 9k; …). Quadratic work for full pagination.
**Proposed fix**: Push the `LIMIT` into the SQL (`LIMIT ? + 1` to detect `HasMore`). The `EventAfterCursor` filter is a strict-inequality on `(sequence, timestamp)` — this can also be encoded as `WHERE sequence > ?`.
**Expected win**: O(page×pages) → O(page) per pagination step; 10× less data transferred for long replays.

### P1-5. Missing index on `runs.workflow_id`

**File/line**: `internal/store/globaldb/archive.go:180-183`, which runs `SELECT COUNT(1) FROM runs WHERE workflow_id = ?` as a correlated subquery.
**Why it hurts**: full `runs` scan per archive-eligibility check.
**Proposed fix**: `CREATE INDEX idx_runs_workflow_id ON runs(workflow_id) WHERE workflow_id IS NOT NULL;` (partial to skip NULL workflow_id rows).
**Expected win**: archive eligibility goes from O(runs) to O(#runs-for-workflow).

### P1-6. Writer pool oversized; readers share connections with writers

**File/line**: `internal/store/sqlite.go:67-68`, `internal/store/store.go:7-9`.
**Pattern**: `SetMaxOpenConns(8)` for every SQLite `*sql.DB`.
**Why it hurts**: WAL permits exactly one writer at a time. With 8 connections a burst of parallel writes (journal + daemon transport) contends on the reserved lock; `busy_timeout=5000` masks this by silently stalling up to 5 s. Also each connection holds memory (page cache duplication across connections).
**Proposed fix**:
1. For `global.db`: keep a single shared `*sql.DB` with `SetMaxOpenConns(1)` for writes; open a second `*sql.DB` on the same path for reads. Or simpler: open with `?_txlock=immediate` and `SetMaxOpenConns(1)` and rely on SQLite's concurrent reader model via the shared-cache wire. Given modernc's default, easiest is `SetMaxOpenConns(1)` and measure.
2. For `run.db` (journal-only writer): `SetMaxOpenConns(1)` unambiguously.
**Expected win**: eliminates `SQLITE_BUSY`/timeout tail latencies. p99 write latency bounded.

### P1-7. Sync reconciliation: per-row upserts without prepared statements

**File/line**: `internal/store/globaldb/sync.go:301-330, 504-529, 638-658, 761-780`.
**Pattern**: Four loops each issuing a bespoke `INSERT … ON CONFLICT` per row inside a single transaction.
**Why it hurts**: Each row re-parses ~300–500 bytes of SQL. Sync batches can touch 100s of artifact rows.
**Proposed fix**: Use `tx.Prepare` once per loop; reuse `*sql.Stmt`. No schema change.
**Expected win**: 2–3× speedup on large workflow syncs (CPU-bound Go parsing, not disk).

---

## P2 Findings

### P2-1. Text timestamps everywhere

**File/line**: `internal/store/values.go:12-27`, 53 usages across the store. Every read path round-trips `time.Parse` on a 30-char string.
**Why it hurts**: In Go, `time.Parse` allocates (format reflection, location lookup). For replays of thousands of rows this shows up as `time.Parse` in profile.
**Proposed fix**: Add `started_at_unix INTEGER` / `timestamp_unix INTEGER` companion columns (migration v2), keep the text for human debugging, read from the integer column. Or migrate fully to integer nanoseconds.
**Expected win**: ~5–10 % CPU on large list/replay endpoints; defer — requires migration + test rewrite.

### P2-2. Duplicate `configureSQLite` after DSN pragmas

**File/line**: `internal/store/sqlite.go:88-123`. Pragmas are already in the DSN and applied per-connection. `configureSQLite` re-applies some on one pool connection (not deterministic which).
**Why it hurts**: Cosmetic / redundant. If the two paths ever disagree (e.g., DSN gets `synchronous(NORMAL)` but `configureSQLite` runs `synchronous = FULL`), only one connection in the pool gets the override.
**Proposed fix**: Remove `configureSQLite` entirely or move all pragmas out of the DSN into this single-point configuration and set them with `conn.Raw(...)` for every connection using `sql.Conn` callbacks.

### P2-3. Excessive `strings.TrimSpace` on every input

**File/line**: `internal/store/globaldb/registry.go` (36 occurrences), similar in `sync.go`, `rundb/run_db.go`.
**Why it hurts**: Each trim allocates a new string on mismatch. Negligible individually; accumulates on hot paths (PutRun/UpdateRun run on every state transition).
**Proposed fix**: Trim once at the API boundary (transport or CLI parse), assume trimmed values thereafter.

### P2-4. `loadExistingArtifactSnapshots` loads full body text just to compare checksums

**File/line**: `internal/store/globaldb/sync.go:358-394` selects `body_text` for every existing snapshot. Used only to preserve old body when checksum matches (`:296-299`).
**Why it hurts**: For 256 KiB bodies × N workflows this transfers MB of data per sync even when nothing changed.
**Proposed fix**: Split the query: first `SELECT artifact_kind, relative_path, checksum, body_storage_kind`. Only when checksum matches AND new payload overflows, issue a targeted `SELECT body_text` on that row (or just preserve by not touching `body_text` on checksum-match via `UPDATE … SET checksum=?, ... (skip body_text)`).

### P2-5. `prepareArtifactSnapshot` computes `len([]byte(bodyText))` redundantly

**File/line**: `internal/store/globaldb/sync.go:426`. In Go, `len(s)` on a string is already byte length; the `[]byte` conversion copies the string.
**Why it hurts**: 256 KiB copy per artifact. For large bodies this is real.
**Proposed fix**: `if len(bodyText) > artifactBodyLimitBytes`.

### P2-6. JSON storage for `depends_on_json` / `summary_json` is fine but unindexable

**File/line**: `migrations.go:70`, `migrations.go:41`. No JSON1 usage today. Future task-graph queries would need JSON extraction.
**Why it hurts**: Only if a future feature wants to query by dependency. Noted for planning.

---

## Verification plan

1. **Bench harness**. Add a bench in `internal/store/rundb/run_db_test.go`:
   - `BenchmarkStoreEventBatch_32` — fill a tmp `run.db`, time `StoreEventBatch(batch of 32 mixed events)` for 1000 iterations.
   - Capture baseline before/after dropping dead indexes and after stmt-prepare refactor.
2. **`EXPLAIN QUERY PLAN`** for each flagged query. Run against a populated DB and confirm the plan. Expected outputs:
   - Before: `SCAN TABLE runs` for `LOWER(TRIM(status))` queries. After (P0-2): `SEARCH runs USING INDEX idx_runs_workspace_status`.
   - Before: `SCAN TABLE runs` for workflow_id subqueries. After (P1-5): `SEARCH runs USING INDEX idx_runs_workflow_id`.
3. **N+1 proof**. Wrap `m.globalDB.GetWorkflow` with a counter in a test; run `RunManager.List` with 50 runs mapped to 5 distinct workflows. Expect 50 calls before; 0 after the join.
4. **Pool contention**. Enable `go-sqlite3`/modernc trace or log `SQLITE_BUSY` returns; reproduce by running two concurrent writers (journal + `PutRun`). Tail latency > 100 ms before; uniform after `SetMaxOpenConns(1)`.
5. **Pragma diff**. After switch, run `PRAGMA cache_size; PRAGMA mmap_size; PRAGMA temp_store; PRAGMA wal_autocheckpoint;` on a daemon-opened DB; compare with expected values.
6. **Purge wall-clock**. `time ./compozy runs purge` on a seeded DB with 500 terminal runs. Expect ≥10× speedup after P1-2 batch-delete.
7. **Golden-output isomorphism**. For every change, capture `sha256sum` of the JSONL journal and the SQL dump (`sqlite3 run.db .dump`) before/after across a canonical run. None of the proposed changes alter stored rows or event bytes — hashes must match. Index drops do not affect `.dump` (indexes are emitted separately); they must vanish but data rows stay identical.

---

## Change Opportunity Matrix

| ID | Hotspot | Impact | Confidence | Effort | Score | Tier |
|---|---|---|---|---|---|---|
| P0-1 | Drop dead rundb indexes | 5 | 5 | 1 | **25.0** | 1 |
| P0-2 | Normalize `status` + drop `LOWER(TRIM)` | 5 | 5 | 2 | **12.5** | 1 |
| P0-3 | Join runs + workflows (remove N+1) | 4 | 5 | 2 | **10.0** | 1 |
| P1-1 | Prepared stmts in StoreEventBatch | 4 | 4 | 2 | **8.0** | 1 |
| P1-2 | Batch DELETE in purge | 3 | 5 | 1 | **15.0** | 1 |
| P1-3 | Batch UPDATE in reconcile | 3 | 5 | 2 | **7.5** | 1 |
| P1-4 | Push LIMIT into ListEvents | 3 | 5 | 1 | **15.0** | 1 |
| P1-5 | `idx_runs_workflow_id` | 3 | 5 | 1 | **15.0** | 1 |
| P1-6 | Writer pool = 1 | 3 | 4 | 1 | **12.0** | 1 |
| P1-7 | Prepared stmts in ReconcileWorkflowSync | 3 | 4 | 2 | **6.0** | 1 |
| P2-1 | Integer timestamps | 3 | 3 | 4 | 2.25 | 2 |
| P2-2 | Remove duplicate pragma config | 1 | 5 | 1 | 5.0 | 2 |
| P2-3 | Remove redundant TrimSpace | 1 | 4 | 2 | 2.0 | 2 |
| P2-4 | Skip body_text load on checksum match | 2 | 4 | 2 | 4.0 | 2 |
| P2-5 | Drop `[]byte(bodyText)` copy | 1 | 5 | 1 | 5.0 | 2 |

Everything ≥ 5.0 is a candidate for this round; P0-1, P1-2, P1-4, P1-5 are fast wins (single-commit each). P0-2 needs a data-migration note (must rewrite existing `status` values).
