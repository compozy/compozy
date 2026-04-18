# Event Journal & Streaming — Performance Analysis

Scope:
- `internal/core/run/journal/journal.go` (durable append-before-publish)
- `internal/core/plan/journal.go` (preparation close helper)
- `pkg/compozy/events/` (envelope + in-process Bus fan-out)
- `pkg/compozy/runs/` (replay / tail / watch over HTTP+SSE)
- `internal/daemon/run_manager.go`, `internal/daemon/watchers.go`, and
  `internal/daemon/*_transport_service.go` (daemon transport layer)

Source commit: `pn/daemon` @ `36e572c`.

> **Caveat:** no baseline numbers were captured in this pass. Every "expected
> win" below is a first-principles estimate from reading the code — validate
> with the benchmarks listed in "Verification plan" before implementing.

---

## Summary

Five hotspots dominate the event journal + streaming path:

1. **Per-event fsync during live operation.** `flushBatch` calls
   `file.Sync()` for *every* writable batch including single-event batches
   triggered by a 100ms ticker. On a fast disk one fsync costs ~5–15 ms; on
   HDD/NFS it is easily 30–100 ms. Every live event incurs that cost on the
   critical path before subscribers ever see the event. (journal.go:615)
2. **Replay re-reads the full history on every page.** `RunDB.ListEvents`
   has no SQL `LIMIT`; the daemon loads every row where `sequence >= fromSeq`,
   then trims in Go. A 5 000-event run transferred page-by-page from a CLI
   client re-fetches ~5 000, 4 744, 4 488 … events — quadratic cost in the
   number of stored events. (run_db.go:273, run_manager.go:498)
3. **Three re-decodes of `payload_json` for every replayed event.** SQLite
   stores the payload as TEXT; `ListEvents` copies it into a `json.RawMessage`
   (fine), then the HTTP handler re-marshals the whole `Event` for SSE, the
   SDK reads the SSE frame and `json.Unmarshal`s it back to `events.Event`,
   and finally the CLI decodes `payload` again to render run details. For
   large `kinds.SessionUpdatePayload` frames this easily triples CPU vs. a
   binary protocol.
4. **Bus fan-out copies the `Event` value (including `json.RawMessage` slice
   header) into a per-subscriber channel and takes an RLock + allocates a
   snapshot slice for every Publish.** With 1 subscriber per run this is
   cheap, but the daemon pattern (UI + workflow-streamer + future SSE
   clients) allocates `O(subs)` pointers per event.
5. **SSE per-event syscall pattern.** `WriteSSE` makes 5–8 separate
   `io.WriteString` / `writer.Write` calls followed by `writer.Flush()` with
   no intermediate buffering; each call is a syscall when gin's
   `ResponseWriter` is wrapped around a `net.Conn`. (sse.go:72–107)

Scores and fixes are in P0/P1/P2 below.

---

## Event Flow Walkthrough

One event, end-to-end. Annotations `[A##]` mark hotspots ranked later.

```
Producer goroutine (executor / session handler)
 └─ events.Event{…, Payload: json.RawMessage}      [A01] event envelope already holds a JSON blob
     └─ journal.Submit(ctx, ev) → j.inbox <- req   [A02] channel send, timer on timeout
         │
         │ writeLoop (single writer goroutine) picks req
         │
         ├─ encodeEvent                            [A03] json.Encoder.Encode(Event{…, Payload: RawMessage})
         │    - reallocates Event if schema/run/ts empty
         │    - appends LF, writes to bufio.Writer
         │
         ├─ append to state.pending
         │
         ├─ shouldFlushAfterAppend?                [A04] terminal OR batchSize=32; else wait for 100ms tick
         │
         └─ flushPending
             ├─ store.StoreEventBatch(ctx, items)  [A05] SQLite BEGIN + N INSERTs (events + projections) + COMMIT
             ├─ writer.Flush()                     [A06] bufio buffer → pwrite on the events.jsonl fd
             ├─ file.Sync()                        [A07] fsync on every flush — critical-path syscall
             └─ liveBus().Publish(ctx, ev) × N     [A08] RLock + snapshot slice + per-sub channel send


Subscriber side (RunManager.OpenStream → streamRun):

Client GET /api/runs/:id/stream with Last-Event-ID
 └─ RunManager.OpenStream
     ├─ globalDB.GetRun (global.db)               [A09] SQL query per-connection
     ├─ openLiveRunSubscription(active)            [A10] Bus.Subscribe → new buf=256 chan per subscriber
     └─ go streamRun
         ├─ replayRunStream                        [A11] calls Events(…) exactly once with limit=256
         │    └─ RunManager.Events(runID, after)
         │        ├─ GetRun                        [A12] SQL again for same run
         │        ├─ openRunDB                     [A13] NEW SQLite handle per request → migrations on first open
         │        ├─ runDB.ListEvents(fromSeq)     [A14] no SQL LIMIT → returns ALL rows ≥ fromSeq
         │        ├─ build filtered []Event        [A15] second allocation + timestamp re-comparison
         │        └─ slice trim to limit=256       [A16] up to (N-256) rows discarded after scan
         │
         ├─ writeStreamItem per replayed event     [A17] WriteSSE: json.Marshal + many small writes + Flush
         │
         └─ streamLiveRunEvents
             ├─ select on subscription.ch          [A18] per-event wake; already JSON-encoded in journal
             ├─ cursor filter via EventAfterCursor
             └─ WriteSSE again                     [A19] re-marshals full Event per live delivery
```

Client side (`pkg/compozy/runs`):
```
OpenRunStream → bufio.NewReader over HTTP body
 └─ per line: strings.HasPrefix + TrimSpace        [A20] per-line string alloc
     └─ dispatchEvent → json.Unmarshal(raw, *Event) [A21] second full decode per event
         └─ sendItem → 32-deep chan                 [A22] tiny buffer; back-pressure silently fills
```

---

## Opportunity Matrix (Score = Impact × Confidence ÷ Effort)

| Hotspot (file:line)                                             | Impact | Conf | Effort | Score | Tier |
|-----------------------------------------------------------------|:------:|:----:|:------:|:-----:|:----:|
| [A14] `RunDB.ListEvents` no SQL LIMIT (run_db.go:273)           |   5    |  5   |   1    | 25.0  |  P0  |
| [A07] Per-flush fsync (journal.go:615)                          |   5    |  4   |   2    | 10.0  |  P0  |
| [A13] Open+migrate RunDB per HTTP request (run_manager.go:482)  |   4    |  5   |   2    | 10.0  |  P0  |
| [A17]/[A19] SSE `WriteSSE` many-syscall pattern (sse.go:72-107) |   3    |  5   |   2    |  7.5  |  P1  |
| [A15]/[A16] second filter+discard loop (run_manager.go:503)     |   3    |  4   |   1    | 12.0  |  P0  |
| [A08] Bus snapshot allocation per Publish (bus.go:145)          |   3    |  4   |   2    |  6.0  |  P1  |
| [A21] Full `events.Event` re-decode in SDK (run.go:857-863)     |   3    |  4   |   3    |  4.0  |  P1  |
| [A12] Redundant `GetRun` on stream open (run_manager.go:478+)   |   2    |  5   |   1    | 10.0  |  P1  |
| [A22] 32-item client buffer (run.go:378)                        |   3    |  3   |   1    |  9.0  |  P1  |
| [A03] `json.Encoder.Encode(Event)` on critical path             |   2    |  3   |   3    |  2.0  |  P2  |
| [A20] SSE frame parser alloc per line (run.go:746-811)          |   2    |  4   |   2    |  4.0  |  P2  |
| [A10] new 256-slot channel per subscriber (bus.go:67)           |   2    |  3   |   2    |  3.0  |  P2  |
| [A02] double select on submit (journal.go:266-294)              |   1    |  4   |   1    |  4.0  |  P2  |

Only items with Score ≥ 2.0 are retained.

---

## P0 Findings

### P0-1. Replay scans entire history per page — O(N × pages)

- **File:** `internal/store/rundb/run_db.go:266-315` (`ListEvents`) and
  `internal/daemon/run_manager.go:472-527` (`Events`).
- **Pattern:**
  ```go
  // run_db.go
  `SELECT sequence, event_kind, payload_json, timestamp
   FROM events WHERE sequence >= ? ORDER BY sequence ASC`
  ```
  No SQL `LIMIT`. `RunManager.Events` then loads ALL rows, does a second
  `EventAfterCursor` filter in Go, allocates `filtered := make([]Event, 0,
  len(events))`, and *only then* trims to `limit`:
  ```go
  filtered := make([]eventspkg.Event, 0, len(events))
  for _, item := range events {
      if apicore.EventAfterCursor(item, query.After) {
          filtered = append(filtered, item)
      }
  }
  if len(filtered) <= limit { … } else { page.Events = append(page.Events, filtered[:limit]...) }
  ```
- **Impact:** For a run with N events, a paginated replay with page size P
  costs `N + (N-P) + (N-2P) + …` ≈ `N² / (2P)` rows read, O(N²) time and
  O(N) memory per page. Worst offenders are session-update-heavy runs
  (thousands of chunk events). In addition, every row materializes a full
  `events.Event{Payload: json.RawMessage(copy)}` in Go heap.
- **Fix (isomorphism preserved — ordering is unchanged, fromSeq
  semantics identical):**
  1. Push `LIMIT ?` into the SQL: `WHERE sequence >= ? ORDER BY sequence
     ASC LIMIT ?`. Pass `limit+1` so the handler can detect `HasMore`
     without re-querying.
  2. Delete the Go-side `filtered` slice and the trim branch — the SQL
     already returns the correct window.
  3. Since `sequence` is the primary key, the planner uses the `events`
     rowid range scan directly; no new index needed.
- **Expected win:** For the common case of a freshly-attached viewer on a
  2 000-event run, replay drops from ~40 paged full-table scans to 8 (one
  per 256-page), each reading exactly 256 rows. That is ~10× fewer rows
  read and ~N× less Go allocation. For a long run (20 k events) it turns
  hundreds of milliseconds of replay latency into low tens.
- **Also fix at [A16]:** once SQL returns ≤ `limit+1` rows the Go-side
  "discard > limit" branch disappears, removing an extra allocation.

### P0-2. `fsync` on every flush including single-event batches

- **File:** `internal/core/run/journal/journal.go:606-642`.
- **Pattern:**
  ```go
  func (j *Journal) flushBatch(writer *bufio.Writer, file *os.File, pending []events.Event) error {
      if err := writer.Flush(); err != nil { … }
      if err := file.Sync(); err != nil { … }   // fsync every flush
      …
      for _, ev := range pending { bus.Publish(ctx, ev) }  // ← publish AFTER fsync
  }
  ```
  Combined with `defaultFlushInterval = 100 * time.Millisecond` and
  `defaultBatchSize = 32`, on a bursty producer (e.g. agent streaming
  chunks) the ticker often fires with 1-5 events pending, so the
  steady-state cost per event is roughly `fsync_latency / avg_batch_size`
  — on an SSD that is ≥1 ms/event; on external/laggy disks or NFS much
  more, and it blocks the writer goroutine, which in turn blocks
  `j.inbox` producers once the inbox fills.
- **Impact:** Every subscriber delivery is strictly after a blocking
  fsync because `flushBatch` publishes only after sync. For session chunk
  events ("streaming tokens") this adds visible lag to the TUI and to
  SSE clients.
- **Fix (single lever, behavior-preserving):**
  - Keep the ordering guarantee (append before publish for *durability*)
    but decouple fsync from publish. Two options — pick one and
    benchmark:
    1. **Group-commit / coalesce fsync**: flush `writer` frequently but
       only `file.Sync()` when (a) the batch contains a terminal event
       or a checkpoint-worthy kind, (b) last-sync age exceeds e.g. 250 ms,
       or (c) `eventsSinceSync ≥ batchSize × 2`. This keeps durability
       for crashes to a bounded window (250 ms) and collapses dozens of
       fsyncs into one.
    2. **Publish-on-write, sync async**: publish to the live bus after
       `writer.Flush()` (bytes reach kernel), and have fsync run on a
       separate ticker. Durability semantic changes from "every
       subscriber delivery implies fsynced" to "fsync catches up within
       N ms"; document this and update tests accordingly.
  - If "append-before-publish" means *kernel write* rather than stable
    storage, option 2 is the right change. If it means fsync, option 1
    is. The current code conflates the two.
- **Expected win:** Steady-state journal throughput 5–50× (dominated by
  fsync on the platform). End-to-end event-to-subscriber latency drops
  by the fsync cost, typically 3–30 ms on SSD, 30–100 ms on slow disks.

### P0-3. Fresh `RunDB` handle (including migrations) per HTTP request

- **File:** `internal/daemon/run_manager.go:482-488, 1549-1555`.
- **Pattern:**
  ```go
  runDB, err := openRunDB(listCtx, runID)     // opens new *sql.DB,
  if err != nil { return … }                  // runs applyMigrations,
  defer func() { _ = runDB.Close() }()        // WAL pragmas, then closes
  ```
  `ListRunEvents`, `GetRunSnapshot`, `streamRun` (via `replayRunStream →
  Events`), `Cancel`, and `resolveTerminalState` each call this. In
  SQLite, `sql.Open` is cheap but `applyMigrations` (in
  `rundb.Open` via `store.OpenSQLiteDatabase`) runs schema-version checks
  and pragma SETs on every open.
- **Impact:** Each streaming client costs at least one full open/close
  during stream setup; polling clients repeat it every call. Under load
  this multiplies SQLite contention (extra transactions per minute,
  extra file handles). For CLI `compozy runs tail` which polls
  `/snapshot` + `/events` repeatedly, this is pure overhead.
- **Fix:** Cache an `*rundb.RunDB` per `runID` in `RunManager` with a
  reference-count / idle-TTL eviction. The daemon already holds active
  runs in `m.active`; extend this to a separate `map[string]*rundb.RunDB`
  keyed by run ID, guarded by `sync.RWMutex`. Close on idle (e.g. 30 s
  after last read) or when `removeActive` runs. For writers the journal
  already owns its own handle (`Journal.store`) — read handles are the
  cheap, safe place to cache.
- **Expected win:** Removes the migration check and WAL pragma churn
  from every client request. For a CLI running `watch` at 100 ms, that
  is 10 fewer `sql.Open` per second per client. Estimated 30–60% CPU
  reduction on the read-heavy daemon profile.

### P0-4. `RunManager.Events` double-scans and re-allocates

- **File:** `internal/daemon/run_manager.go:503-525`.
- **Pattern:** Even after P0-1, the current code does a second pass to
  build `filtered` just to reapply `EventAfterCursor(item, query.After)`
  — but `ListEvents(fromSeq = query.After.Sequence)` already filters on
  sequence. The additional predicate only fires when `timestamp` is
  before the cursor timestamp at equal sequence, which is impossible
  (sequence is monotonic within a run).
- **Fix:** Once P0-1 is in, drop the `filtered` slice entirely and
  build the page directly from `events`. Keep the sequence-based
  filter in SQL; handle `HasMore` via the `limit+1` convention.
- **Expected win:** One fewer copy per replayed event (~48 bytes +
  `json.RawMessage` slice-header copy per event, no deep copy of
  payload), and one less heap allocation per page. Trivial per event,
  measurable on a 20 k-event warm replay.

---

## P1 Findings

### P1-1. `WriteSSE` does 5–8 writes + Flush per event

- **File:** `internal/api/core/sse.go:59-107`.
- **Pattern:** for each event the function calls `writeSSEString` up to
  six times (each `io.WriteString(writer, value)`), plus
  `writer.Write(payload)`, plus `writer.Flush()`. With Gin's default
  `ResponseWriter` wrapping a `net.Conn` these are real syscalls, and
  the `Flush()` sends a TCP packet with TCP_NODELAY behavior.
- **Fix (behavior-preserving):** Build the frame in a pooled
  `bytes.Buffer`, then do one `writer.Write(buf.Bytes())` and one
  `writer.Flush()`. Reuse the buffer via `sync.Pool`. This is the
  canonical SSE pattern.
  ```go
  var sseBufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}
  // frame := sseBufPool.Get().(*bytes.Buffer); frame.Reset(); defer sseBufPool.Put(frame)
  // frame.WriteString("id: "); frame.WriteString(id); frame.WriteByte('\n') …
  // frame.WriteString("data: "); frame.Write(payload); frame.WriteString("\n\n")
  // writer.Write(frame.Bytes()); writer.Flush()
  ```
- **Expected win:** 5–8× fewer syscalls per event on SSE path. Critical
  for runs producing high-volume `session.update` chunks.

### P1-2. Bus `snapshot()` allocates per Publish

- **File:** `pkg/compozy/events/bus.go:76-95, 145-154`.
- **Pattern:**
  ```go
  func (b *Bus[T]) snapshot() []*subscription[T] {
      b.mu.RLock(); defer b.mu.RUnlock()
      snapshot := make([]*subscription[T], 0, len(b.subs))
      for _, sub := range b.subs { snapshot = append(snapshot, sub) }
      return snapshot
  }
  ```
  Called on every `Publish`. For runs with 2–3 subscribers this is
  small, but the allocation itself escapes to the heap. Over a 20 k
  event run that is 20 k allocations plus map iteration.
- **Fix options:**
  1. Swap the snapshot slice for an atomic pointer (`atomic.Pointer[[]sub]`)
     rebuilt only on Subscribe/Unsubscribe. Publish path becomes
     lock-free with one atomic load.
  2. Or reuse a `sync.Pool` of `[]*subscription[T]` slices.
  Option 1 is strictly better — subscription topology changes rarely
  vs. publish rate.
- **Expected win:** Removes N allocations per run and eliminates the
  RLock from the publish hot path, at a small cost when subs change.

### P1-3. SDK re-decodes full `events.Event` for every SSE frame

- **File:** `pkg/compozy/runs/run.go:857-863, 742-811`.
- **Pattern:** `dispatchEvent` does `json.Unmarshal(raw, &events.Event{})`
  after the daemon already encoded a structurally identical value. For
  consumers that just want a projection (status / progress bar) this
  pays full decode cost per event.
- **Fix:** Expose a typed "lazy" Event where `Kind`, `Seq`, and
  `Timestamp` decode via a small head-parse (e.g. `json.Decoder.Token`
  through the envelope fields), and `Payload` stays as
  `json.RawMessage` until accessed. Alternatively, provide a
  `RawEvent` type and let the few callers that need it upgrade to a
  decoded view.
- **Expected win:** 2–3× faster decode for the common "filter by kind
  then render cursor" consumer loops in `pkg/compozy/runs` examples.

### P1-4. Redundant `GetRun` calls on stream open

- **File:** `internal/daemon/run_manager.go:475-499, 530-554`.
- **Pattern:** `OpenStream` calls `globalDB.GetRun`. Then it calls
  `replayRunStream` → `Events(runID, …)` which calls `globalDB.GetRun`
  again. Two SQL queries on the global DB for the exact same runID on
  every stream-open.
- **Fix:** Change `Events` to accept an already-resolved `runID string`
  without the GetRun guard when called internally, or refactor into an
  unexported `eventsInternal(ctx, runID, query)` helper. Keep the
  external `Events` (called from HTTP handler) doing the check exactly
  once.
- **Expected win:** Halves global.db queries on `/stream` setup; with
  many subscribers this is the biggest global-lock pressure source.

### P1-5. SDK stream has 32-deep buffer, drops on overrun

- **File:** `pkg/compozy/runs/run.go:377-383, 865-872`.
- **Pattern:**
  ```go
  items: make(chan RemoteRunStreamItem, 32),
  …
  case s.items <- item: return nil
  default: return errors.New("client run stream buffer is full")
  ```
  A terminal error on the first slow consumer iteration; no
  documentation of this limit; default client sits next to a daemon
  that buffers 64 events plus a 256-depth Bus channel.
- **Fix:** Either make the buffer configurable or block-with-ctx
  (`select ctx.Done() / s.items <-`) rather than non-blocking drop;
  the SSE framing already preserves order, so dropping mid-stream
  breaks cursor monotonicity.
- **Expected win:** Eliminates a whole failure mode for slow UI
  consumers. Performance: a blocking send is the right backpressure
  signal and costs nothing when consumers keep up.

---

## P2 Findings

### P2-1. `json.Encoder` + re-allocation in `encodeEvent`

- **File:** `internal/core/run/journal/journal.go:586-604`.
- **Pattern:** Every event hits `strings.TrimSpace(enriched.SchemaVersion)`,
  `strings.TrimSpace(enriched.RunID)`, `time.Now().UTC()`, and then
  `encoder.Encode(enriched)`. `json.Encoder.Encode` internally
  allocates a `*bytes.Buffer` for every call, even when the encoder is
  long-lived.
- **Fix:** Cache a pooled `*bytes.Buffer` and `json.Encoder(buf)` per
  writer; or manually write the envelope fields (known, small, stable
  JSON) and inject the already-encoded `Payload` verbatim. The event
  schema is fixed — a hand-written encoder could be 3× faster and
  allocation-free.
- **Expected win:** Smaller than P0 but non-zero; reclaims CPU from
  the single writer goroutine, which today is the bottleneck when
  fsync is fast (NVMe, tmpfs tests).

### P2-2. SSE client parser does per-line `strings.TrimSpace` + prefix

- **File:** `pkg/compozy/runs/run.go:778-799`.
- **Pattern:** Each incoming line does three separate
  `strings.HasPrefix` + `strings.TrimPrefix` + `strings.TrimSpace`
  passes, and each creates a short-lived string. For high-volume runs
  this dominates SDK CPU.
- **Fix:** Switch to a byte-oriented parser using `bytes.HasPrefix` on
  the line buffer from `bufio.Reader.ReadSlice('\n')`. Keep the
  parsed frame fields as pre-allocated `[]byte` that the payload
  decoder consumes directly.
- **Expected win:** 2× SDK-side decode throughput in tight replay
  loops.

### P2-3. Per-subscriber 256-slot channel

- **File:** `pkg/compozy/events/bus.go:13-68`.
- **Pattern:** Each `Subscribe()` allocates `make(chan T, bufSize=256)`
  with a copy of the event value for every delivery. `events.Event`
  plus the embedded `json.RawMessage` slice-header is ~80 B so the
  channel backing array is ~20 KB per subscriber — tolerable, but
  every Publish copies the value into the channel (two copies total:
  producer → chan, chan → consumer).
- **Fix:** Pass `*events.Event` through the channel; the payload is
  already a `json.RawMessage` (immutable from consumer perspective
  post-publish). This avoids a large value copy on both ends.
- **Expected win:** Modest; measurable under chunk storms.

### P2-4. Double select in `Journal.submit`

- **File:** `internal/core/run/journal/journal.go:266-294`.
- **Pattern:** First non-blocking select, then a blocking select with a
  fresh `time.NewTimer(j.submitTimeout)` (default 5 s). The double
  pattern is fine; the `time.NewTimer`+`Stop` pair allocates. For the
  99% case where the inbox has room the first select wins, so this is
  really a cost on the slow path. Still, if the submitter is hot
  (session chunks under backpressure), those timer allocations add up.
- **Fix:** Use a per-journal pooled `*time.Timer` or `time.AfterFunc`
  with cancellation, or switch the blocking send to a select on
  `ctx.Done()` only (drop the submit timeout now that the inbox is
  configurable and backpressure is observable via
  `DropsOnSubmit`).
- **Expected win:** Small, and only on the slow/backpressure path.

---

## Verification plan

Measurement first — do not implement any P0 change before the
corresponding benchmark baseline is captured.

1. **Journal micro-benchmark (new):**
   `internal/core/run/journal/journal_bench_test.go` — measure
   Submit→durable write→Publish latency for 1, 10, 100 concurrent
   producers × {1 ev, 10 ev, 100 ev} batches. Report ns/op and
   allocs/op with `-benchmem`. Run on SSD and on a `tmpfs`-mounted
   path (simulates NFS vs local disk gap). Capture baseline and
   re-run after each P0 change individually.

2. **Replay throughput benchmark (new):**
   `pkg/compozy/runs/replay_bench_test.go` — seed a run with 1 k, 10 k,
   100 k events, then measure the time for `Run.Replay(0)` to drain
   using the in-process daemon. Expect P0-1 to be dramatically faster
   at 10 k+.

3. **SSE CPU profile:**
   `go test -run XXX -bench BenchmarkStreamRun -cpuprofile cpu.out`
   on `internal/api/core`. Confirm `WriteSSE` and `json.Marshal` are
   the top symbols before P1-1.

4. **Bus fan-out benchmark:**
   add `BenchmarkBusPublishNSubs` with N ∈ {1, 4, 16, 64}. Compare
   `allocs/op` before and after P1-2.

5. **Golden output invariants:**
   - `events.jsonl` byte-for-byte identical after P0-1 (SQL change)
     and P0-2 (fsync change) — dump with `sha256sum -c`.
   - Replay order: assert `[]Event` from `Run.Replay(0)` is
     sequence-monotonic before/after P0-1.
   - SSE framing: capture a reference `curl -N` dump and diff
     before/after P1-1 (IDs, event names, data payloads must match;
     whitespace inside `data:` is irrelevant to SSE but keep it
     bit-identical for safety).

6. **Durability test (P0-2):**
   introduce a test that kills the daemon between Submit and
   subscriber observation under the *new* publish-on-flush semantic;
   assert that the un-fsynced events are recoverable from the jsonl
   tail on restart (the existing `recoverJournalFile` handles partial
   tail truncation). Document the new crash-loss window in journal.go
   and add it to the contract test suite.

7. **Concurrency proof:**
   run `go test -race -count=20 ./internal/core/run/journal/...
   ./internal/api/core/... ./pkg/compozy/events/...` after every
   structural change (Bus atomic snapshot, RunDB cache).

8. **Before-claiming-done gate:**
   `make verify` (fmt + lint + test + build) per the repo contract,
   plus the two benchmarks above with a ledger entry showing
   before/after numbers.

Implement one P0 at a time; keep each commit behind a flag or a
constant if behavior changes. After each commit, re-profile — the
ranking after P0-1 will change (fsync cost becomes dominant by
percentage once replay scans shrink).
