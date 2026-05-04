# Run Executor — Performance Analysis

> Scope: `internal/core/run/executor/`, `internal/core/run/exec/`,
> `internal/core/run/internal/{runshared,runtimeevents,acpshared}`,
> `internal/core/run/run.go`, `exec_facade.go`, `internal/core/run/preflight/`.
> Methodology: profile-driven thinking (no profiler run yet — this is a static
> trace; all claims below MUST be validated with `go test -bench`/`pprof`
> before implementing). One lever per change. Behavior preserved.

## Summary

The run pipeline is structurally sound for a small-N workflow (≤16 concurrent
jobs, seconds-to-minutes per job), but it is **event-dispatch-bound** whenever
an agent streams many `session.update` frames (a long Claude/Codex session
easily emits 1k–10k updates). Three classes of cost dominate:

1. **Duplicated JSON marshalling on every session update** — each update is
   marshalled ≥3 times (`PublicSessionUpdate` deep-copy, `json.Marshal` inside
   `runtimeevents.NewRuntimeEvent`, again inside the journal writer's
   `json.Encoder.Encode`, plus a fourth full `json.Marshal→Unmarshal`
   round-trip per observer subscriber inside `HookDispatcher.DispatchObserver`
   via `cloneJSONValue`).
2. **Per-event reflection and allocations on the hot submit path** —
   `acpshared.hasRuntimeEventSubmitter` runs `reflect.ValueOf` for every
   runtime event submitted, and `session_handler.HandleUpdate` builds a
   `formatBlockTypes` string (map alloc + sort + join) **unconditionally**
   even when slog is at WARN.
3. **Goroutine creation per submit / per waiter** — the bus is fan-out via
   goroutines-per-subscriber-per-publish for the journal's post-flush loop is
   fine, but `jobExecutionContext.waitChannel()` spawns a goroutine every
   time it is called, the `HookDispatcher.DispatchObserver` spawns one
   goroutine per extension per call (even for payloads that will be dropped),
   and the journal writer path allocates a fresh `submitRequest{}` and
   negotiates two `select` hops for each submit.

None of these is catastrophic on its own. Together, when an agent session
produces 50+ updates per second, they put 40–70% of the event-dispatch CPU
into marshalling/reflection rather than actual delivery, and they make the
blocking `Submit` path the bottleneck for sustained throughput.

## Execution Pipeline Walkthrough

Tracing one workflow run (mode=PRDTasks, UI on, OutputFormat=text,
daemon-owned=false, single job, long session):

1. `run.Execute` → `executor.Execute` →
   `prepareExecutionConfig` — deep-copies `RuntimeConfig` to snapshot, then
   dispatches `run.pre_start` (mutable hook) and re-snapshots to diff.
   **Hotspot P2:** diff runs `CloneTaskRuntimeRules` twice and `append([]string(nil), …)` twice.
   Only runs once per run — low total impact.
2. `ensureRuntimeEventBus` — allocates a `Bus[events.Event]` of buffer 64.
   **P2:** `runtimeEventBusBufferSize = 64` is small for
   session-update-heavy runs with two subscribers (UI + streamer). Bursts
   will trigger `sub.warnDrop` every second and silently lose frames.
   Bumping to 512 or making it configurable removes the drop-warn log
   overhead from the happy path.
3. `startWorkflowEventStreamer` — spawns 1 goroutine that ranges over the
   bus subscription and `json.Encoder.Encode`s every matching event to
   stdout. See P1-E: shared `json.Encoder` is fine, but there is no
   `bufio.Writer` wrapping stdout; every `Encode` triggers a write syscall
   per event. On a fast-streaming session this is the #1 syscall hotspot.
4. `executeJobsWithGracefulShutdown` → `newJobExecutionContext` →
   `launchWorkers` (goroutine per job, guarded by `sem`) or
   `launchOrderedWorkers` (**single driver goroutine** that iterates in
   order). For PRDTasks this serializes jobs; OK by design.
5. Per job: `jobRunner.run` → `dispatchPreExecuteHook` → loop over
   `maxAttempts`, calling `executeAttempt` → `acpshared.ExecuteJobWithTimeout`.
   **Hotspot P1-A:** each `dispatchPreExecuteHook` call allocates a full
   `model.Job` clone TWICE (once for `before`, once inside the hook payload),
   and `applyHookModelJob` allocates a third clone of `CodeFiles`, `Prompt`,
   and `Groups`. This is bounded per retry but expensive when prompt bodies
   are large (multi-MB).
6. `ExecuteJobWithTimeout` → `SetupSessionExecution` → spawns a goroutine
   to run `StreamSessionUpdates(session, handler)` → per update:
   - `handler.HandleUpdate(update)` — see P0-A/P0-B below.
   - Any of `submitRuntimeEvent` → `runtimeevents.NewRuntimeEvent` →
     `json.Marshal(payload)` → `journal.Submit(ctx, event)` → blocks on
     the journal inbox (`submitTimeout` default = 5 s).
7. Per session update inside `SessionUpdateHandler.HandleUpdate`:
   - `PublicSessionUpdate(update)` — full deep copy (blocks, thought blocks,
     plan entries, commands, JSON copies for tool inputs). **P0-A.**
   - `model.DispatchObserverHook("agent.on_session_update", payload)` —
     if any extension subscribes, does `cloneJSONValue` (marshal+unmarshal)
     per subscriber and spawns a goroutine. **P0-B.**
   - `renderUpdateBlocks` — calls `renderContentBlocks` then writes stdout
     + log files. Two `Fprintln` per line, no buffering on the log file or
     stdout. **P1-F.**
   - `emitReusableAgentLifecycleFromUpdate` — iterates blocks, decodes JSON
     per tool-use block. When there are no reusable agents this still runs.
     **P2.**
   - `emitSessionUpdateEvent` → `submitRuntimeEvent` — another marshal.
     **P0-A tail.**
   - `recordUsageUpdate` if usage present — another marshal + submit.
   - `h.logger.Info("acp session update", …, "block_types", formatBlockTypes(update.Blocks))`
     — builds a temporary map + keys slice + sort + join **every update**
     regardless of log level. **P1-C.**
8. Journal writer loop (`runActiveLoop`): each request is JSON-encoded into
   the bufio.Writer, appended to `state.pending`, and on batch-fill or
   terminal event it flushes via `writer.Flush() → file.Sync() →
   bus.Publish` per event. **P1-B:** `file.Sync()` per batch is correct for
   durability, but when the batch size is only `defaultBatchSize = 32` and
   terminal events flush immediately, bursty session-update streams cause
   many small fsyncs. Not a dominant cost compared to marshalling but real.
9. On completion: `finalizeExecution` waits for observer hooks
   (5 s timeout), dispatches `run.pre_shutdown`, writes the result JSON to
   disk twice (once indented, once raw for `OutputFormatRawJSON`).

## P0 Findings

### P0-A. Triple-marshal of every `session.update` on the producer goroutine

- **File / line:** `internal/core/run/internal/runtimeevents/events.go:20-30`
  and `internal/core/run/internal/acpshared/session_handler.go:111-175`
  and `internal/core/run/internal/acpshared/session_handler.go:199-208`.
- **Pattern:**
  1. `PublicSessionUpdate(update)` deep-clones every block (see
     `internal/core/contentconv/contentconv.go:31-71, 115-205`) — including
     `copyJSON` which copies `ToolUseBlock.Input` and `RawInput` byte-by-byte.
  2. `runtimeevents.NewRuntimeEvent(runID, kind, payload)` does
     `json.Marshal(payload)` (entire SessionUpdate tree) to produce
     `events.Event.Payload`.
  3. The journal writer, inside `encodeEvent`
     (`internal/core/run/journal/journal.go:586-604`), calls
     `encoder.Encode(enriched)` which re-marshals the whole envelope
     *including* the already-marshalled `json.RawMessage` Payload. That
     isn't a full re-marshal of the payload bytes (RawMessage is written
     verbatim), but the enclosing Event is re-marshaled and written.
  4. Each bus subscriber's Publish does not re-marshal (it hands the same
     struct), but the workflow JSON streamer calls `encoder.Encode(ev)` a
     second time on stdout for JSON modes.
- **Impact:** On streaming sessions with 1k updates, estimate 2–4 MB/s of
  Go-heap marshalling churn per session just for `SessionUpdate` envelopes,
  all on the ACP-read goroutine that also must not stall (stalling here
  backs up the ACP subprocess pipe and eventually deadlocks).
- **Fix sketch:**
  - Marshal the public session update once; pass `json.RawMessage` through
    to the hook payload and the runtime event:
    ```go
    raw, err := json.Marshal(publicUpdate)
    // hook payload
    payloadRaw, _ := json.Marshal(sessionUpdateHookPayload{
        RunID: h.runID, JobID: h.jobID, SessionID: h.sessionID,
        Update: raw, // now json.RawMessage
    })
    // reuse payloadRaw for the runtime event
    event := events.Event{ RunID: h.runID, Kind: events.EventKindSessionUpdate, Payload: payloadRaw }
    return h.journal.Submit(h.ctx, event)
    ```
    Requires widening `sessionUpdateHookPayload.Update` to `json.RawMessage`
    (extensions already see JSON over the wire).
  - Pool `bytes.Buffer` + `json.Encoder` with `sync.Pool` for the
    `runtimeevents.NewRuntimeEvent` hot path. Encoder reuse with
    `SetEscapeHTML(false)` shaves additional allocations.
- **Expected win:** ~40–60% CPU reduction on session-update dispatch path;
  ~50% heap alloc reduction per update. Isomorphism: identical bytes on
  the wire (same encoder, same schema).

### P0-B. `DispatchObserver` clones every payload via `json.Marshal`+`json.Unmarshal` and spawns a goroutine per subscriber, per call

- **File / line:** `internal/core/extension/dispatcher.go:107-144` and
  `internal/core/extension/dispatcher.go:306-317` (`cloneJSONValue` = full
  round-trip into `map[string]any`). Invoked from
  `internal/core/run/internal/acpshared/session_handler.go:118-128` on every
  session update.
- **Pattern:** The dispatcher calls `cloneJSONValue(payload)` for every
  `entry` (subscriber), producing a map-based deep copy, then `go func(...)`
  to invoke it. On a session that emits 2k updates with two extensions
  subscribing to `agent.on_session_update`, that is 4k goroutines spawned
  and 4k marshal+unmarshal round-trips, each allocating a fresh
  `map[string]any` tree. The scheduler handles it but the GC load is
  substantial.
- **Impact:** When *any* extension subscribes to streaming hooks, this
  dwarfs everything else in the producer path. Under zero subscribers the
  cost collapses to a cheap `d.chainEntries(hook)` map lookup.
- **Fix sketch:**
  1. Marshal the payload **once** outside the subscriber loop, pass
     `json.RawMessage` to each subscriber goroutine. Each subscriber's
     RPC/IPC encoder can forward the raw bytes without re-unmarshalling
     into a `map[string]any`.
     ```go
     raw, err := json.Marshal(payload)
     // loop subscribers
     for _, entry := range entries {
         d.pending.Add(1)
         go func(entry hookChainEntry, raw json.RawMessage) { ... }(entry, raw)
     }
     ```
  2. Bound observer fan-out with a small worker pool
     (e.g. `runtime.GOMAXPROCS` workers per dispatcher) feeding a
     buffered channel. Drops stale updates when the queue saturates
     (observer = fire-and-forget, already best-effort per comment).
  3. For streaming-safe hooks (`agent.on_session_update`) add an explicit
     opt-in flag so that extensions that don't need per-frame updates are
     skipped entirely — most audit extensions only need
     session.completed / session.failed.
- **Expected win:** 3–10× throughput on `HandleUpdate` when an extension
  is subscribed; eliminates a goroutine-spawn storm that degrades
  scheduler tail latency.

### P0-C. `hasRuntimeEventSubmitter` uses `reflect.ValueOf` on every runtime event submit

- **File / line:** `internal/core/run/internal/acpshared/command_io.go:26-37`.
- **Pattern:**
  ```go
  func hasRuntimeEventSubmitter(submitter runtimeEventSubmitter) bool {
      if submitter == nil { return false }
      value := reflect.ValueOf(submitter)
      switch value.Kind() { case reflect.Chan, …: return !value.IsNil() }
      return true
  }
  ```
  Called from `submitRuntimeEvent` (per event), `emitSessionStartedEvent`,
  `emitReusableAgentSetupLifecycle`, and the reusable-agent lifecycle path.
  Uses reflection solely to detect a typed-nil `*journal.Journal`.
- **Impact:** `reflect.ValueOf` + `Kind()` is measurable (100–200 ns + 1
  alloc per call, per `go:noinline` concerns). Multiplied by 1k updates it
  is a ~100 µs + 24 KB/s footprint for something that should be free.
- **Fix sketch:** Drop reflection entirely. The only caller that can
  pass a typed-nil is a converted `*journal.Journal`; make the check
  explicit at the call-site or use a tiny helper that does a non-reflective
  typed assertion:
  ```go
  func hasRuntimeEventSubmitter(s runtimeEventSubmitter) bool {
      if s == nil { return false }
      if j, ok := s.(*journal.Journal); ok { return j != nil }
      return true
  }
  ```
  Alternatively, the callers that already hold `*journal.Journal` (all of
  them in this package) should check `j != nil` directly and bypass the
  interface indirection.
- **Expected win:** measurable but small; most valuable as *simplification*
  removing reflection from a hot path, enabling further compiler
  optimizations (inlining the wrapper).

## P1 Findings

### P1-A. `hookModelJob` / `applyHookModelJob` perform three deep clones of the job's `Prompt` + `CodeFiles` + `Groups` per retry

- **File / line:** `internal/core/run/executor/hooks.go:77-128` and the
  caller `runner.go:186-209`.
- **Pattern:** For every retry / post-execute, we clone the whole job
  (including prompt bytes and grouped issues) to build a hook payload and
  diff two copies. `applyHookModelJob` then clones once more when
  applying. When prompt bodies are large (review mode can push hundreds of
  KB), this is real cost per retry.
- **Impact:** Bounded by `MaxRetries+1` dispatches per job; dominates when
  MaxRetries is high and hooks mutate. Otherwise O(1) per job.
- **Fix sketch:** For the no-hook common case (`RuntimeManager == nil`),
  short-circuit both `hookModelJob` calls in `dispatchPreExecuteHook` and
  `dispatchPostExecuteHook` — `DispatchMutableHook` already returns
  `payload` unchanged when manager is nil, so the clones are wasted. Guard
  with:
  ```go
  if r == nil || r.execCtx == nil || r.execCtx.cfg == nil ||
     r.execCtx.cfg.RuntimeManager == nil {
      return nil
  }
  ```
  For the hooked path, replace the `append([]byte(nil), src.Prompt...)`
  double-copy with a single alloc by passing a slice copy into the
  payload and mutating in-place when applying, OR switch `model.Job`'s
  Prompt to an immutable reference (prompts aren't mutated by hooks in
  practice — validate via `jobRuntimeChanged`).
- **Expected win:** Eliminates 3× prompt-body allocs per retry in the
  no-extension case (the default). On 100 jobs × 1 retry × 100 KB prompts
  that is 30 MB of avoided copies.

### P1-B. Journal batch flushes the whole file at every terminal event + every 32-event batch, sync-blocking the producer

- **File / line:** `internal/core/run/journal/journal.go:473-500` and
  `606-642`.
- **Pattern:**
  ```go
  func (j *Journal) shouldFlushAfterAppend(pending []events.Event, kind events.EventKind) bool {
      return isTerminalEvent(kind) || len(pending) >= j.batchSize
  }
  ```
  Then `flushBatch` does `writer.Flush → file.Sync → bus.Publish per event`.
  On a stream that emits 10 session-updates per logical "flush window"
  we fsync every ~32 updates (100 ms window). Each `file.Sync` is
  hundreds of microseconds on Linux, low-ms on macOS. Meanwhile
  `bus.Publish` inside the writer goroutine blocks the writer if any
  subscriber channel is full (but the bus implementation drops when full,
  so publish is O(subscribers) — still fine).
- **Impact:** For long-running steady-stream jobs, fsync overhead is a
  few percent of total writer CPU. For short bursts (10 events then idle
  for a minute) it is irrelevant because the ticker flushes anyway.
- **Fix sketch:**
  1. Separate the durable flush path from the bus publish: publish to the
     bus immediately after `writer.Flush` (before `file.Sync`) so
     subscribers see updates without waiting on fsync. Verify ordering vs
     readers who re-read the file — they already tail-follow, so this is
     safe.
  2. Raise `defaultBatchSize` to 128 or make it adaptive (double up to
     512 when the inbox has >N queued items).
  3. Defer the fsync to a periodic worker (100 ms ticker is already
     present), only fsync immediately on terminal events.
- **Expected win:** ~2–5% CPU under high streaming load + significant
  latency reduction for subscribers.

### P1-C. Unconditional `formatBlockTypes` on every session update

- **File / line:** `internal/core/run/internal/acpshared/session_handler.go:154-173`
  and `404-419`.
- **Pattern:** `h.logger.Info("acp session update", …, "block_types",
  formatBlockTypes(update.Blocks), …)` — `formatBlockTypes` allocates a
  `map[model.ContentBlockType]int`, an `[]string` of keys, sorts them,
  and `strings.Join`s them. Runs every call, regardless of whether the
  logger is enabled.
- **Impact:** ~4 allocations + sort per update. On a 1k-update session
  that is 4k extra allocations + a few dozen ms of wasted CPU.
- **Fix sketch:** Guard with `slog.Logger.Enabled(ctx, slog.LevelInfo)`
  OR convert to a `slog.LogAttrs` with a lazy `slog.Attr` value
  (`slog.Any("block_types", lazyFormatter{blocks})` where the formatter
  implements `LogValuer`). Same fix applies to `snapshotBlockCounts`.
- **Expected win:** eliminates the allocation and map/sort entirely when
  not logging at Info level (the daemon path runs at WARN).

### P1-D. `jobExecutionContext.waitChannel()` spawns a goroutine per call

- **File / line:** `internal/core/run/executor/execution.go:642-649`.
- **Pattern:** `waitChannel` allocates a channel and starts a goroutine
  to run `wg.Wait()` and close it. Called from `executeJobsWithGracefulShutdown`
  and `executorController`. Only invoked once per run today — low impact
  — but note: if `controller.done` is ever re-derived (e.g. after force
  shutdown fallback) the goroutine leak path exists.
- **Fix sketch:** Expose a stored `done` channel on the execCtx that is
  closed exactly once by the last worker; remove the secondary goroutine.
  Callable lazily via `sync.Once`.
- **Expected win:** minor; primary benefit is lifecycle clarity (the
  goroutine currently has no ownership story).

### P1-E. `json.Encoder` writes straight to `os.Stdout` without buffering in `startWorkflowEventStreamer`

- **File / line:** `internal/core/run/executor/event_stream.go:60`.
- **Pattern:** `encoder := json.NewEncoder(dst)` where `dst == os.Stdout`.
  `json.Encoder.Encode` does its own internal buffer, but it flushes via
  `Writer.Write` once per call — and os.Stdout is unbuffered on POSIX, so
  every event becomes a write syscall. Additionally, `exec.go:1063-1076`
  mirrors the same pattern (`json.Marshal + append '\n' + Write`) with
  two writers (rawWriter + stdoutWriter) each taking a mutex.
- **Impact:** 1 syscall per event per subscriber. On a 1k-update run with
  lean JSON streaming that is 1k syscalls vs ~10 (100-event batched)
  writes. Not huge in absolute terms (<5 ms) but noticeable on slow
  terminals.
- **Fix sketch:** Wrap stdout in a `bufio.Writer` (flush on terminal
  events and a 50 ms ticker), or batch encode and flush every N events.
  The stream already closes when the terminal event arrives, so flushing
  on terminal boundaries is sufficient.
- **Expected win:** 80–95% reduction in write syscalls for JSON stream
  mode.

### P1-F. `renderUpdateBlocks` writes one line at a time to multi-writers without buffering

- **File / line:** `internal/core/run/internal/acpshared/session_handler.go:177-191`
  and `runshared/buffers.go:80-85` (`CreateLogWriters` → `io.MultiWriter`).
- **Pattern:** `writeRenderedLines` writes each rendered line via
  `Fprintln(writer, line)`. The writer is a `MultiWriter(file, os.Stdout)`
  when human output is enabled, so every line hits two writers and is
  not bufio-buffered on either side.
- **Impact:** High-frequency agent message chunks (streaming tokens) can
  emit hundreds of tiny writes per second. Disk and stdout both pay.
- **Fix sketch:** Wrap the per-target writers in `bufio.Writer` inside
  `CreateLogWriters`; flush on session completion and on a short ticker.
  Guarantee the flush on shutdown (the file close path already implies
  it if owned by the session execution). Keep a direct path for the raw
  `io.Writer` callers that care about per-line ordering (e.g. TUI
  renderer, which already has its own buffering).
- **Expected win:** ~5× fewer syscalls on streaming sessions; removes
  lock contention on stdout.

### P1-G. `composeSessionPrompt` allocates 3× for every session create

- **File / line:** `internal/core/run/internal/acpshared/command_io.go:387-395`.
- **Pattern:**
  ```go
  basePrompt := append([]byte(nil), prompt...)
  combined := strings.TrimSpace(systemPrompt) + "\n\n" + string(basePrompt)
  return []byte(combined)
  ```
  Allocates (1) a copy of the prompt, (2) a concatenation string, (3)
  conversion back to `[]byte`. On resume path we also call
  `model.CloneMCPServers` + `buildSessionEnvironment` (fresh map every
  call).
- **Impact:** O(prompt_size) per session setup. For reviews with large
  prompts + retries this stacks up.
- **Fix sketch:**
  ```go
  func composeSessionPrompt(prompt []byte, systemPrompt string) []byte {
      sys := strings.TrimSpace(systemPrompt)
      if sys == "" { return prompt } // already a fresh copy at the caller
      out := make([]byte, 0, len(sys)+2+len(prompt))
      out = append(out, sys...)
      out = append(out, '\n', '\n')
      out = append(out, prompt...)
      return out
  }
  ```
  Remove the defensive base-copy — `job.Prompt` is already cloned at job
  creation and before hook dispatch.
- **Expected win:** ~3× fewer allocations for session setup (small
  absolute cost but lives on a retry-sensitive path).

## P2 Findings

### P2-A. `aggregateUsage` serializes all session handlers on `aggregateMu`

- **File / line:** `internal/core/run/internal/acpshared/session_handler.go:218-228`
  and `executor/runner.go:141-152`.
- **Pattern:** Every `session.update` carrying usage takes
  `aggregateMu.Lock()` to add 5 ints to `aggregateUsage`. When concurrent
  >1 this produces lock contention on the usage-update hot path.
- **Fix sketch:** Switch `model.Usage` aggregation to atomics
  (`atomic.Int64` per field) and call `atomic.AddInt64` — reads happen
  only at end-of-run.
- **Expected win:** eliminates contention at `Concurrent > 1`; near-zero
  otherwise.

### P2-B. `executor.waitChannel` + `controller.done` race: the writer goroutine drains `inbox` on close, but `Submit` path takes `j.submitMu.RLock` on *every* submit

- **File / line:** `internal/core/run/journal/journal.go:248-295`.
- **Pattern:** `submit` takes `submitMu.RLock` on every call to avoid
  submitting into a closed inbox. `closing` is a simple bool guarded by
  the same mutex. An `atomic.Bool` plus a single `select` on `j.done`
  would be cheaper on the hot path (no RWMutex operations).
- **Fix sketch:** Replace `closing bool` with `atomic.Bool` and drop the
  RLock around `case j.inbox <- req`; rely on the `case <-j.done`
  fallthrough to catch close races.
- **Expected win:** removes one atomic + mutex op per submit; meaningful
  when submitting 10k+ events/run.

### P2-C. `snapshotWorkflowPreparedStateConfig` copies `TaskRuntimeRules` twice per run

- **File / line:** `internal/core/run/executor/execution.go:298-358`.
- **Pattern:** Each run calls `snapshotWorkflowPreparedStateConfig` twice
  (once before hook, once after) to diff, which each time calls
  `model.CloneTaskRuntimeRules` and `append([]string(nil), cfg.AddDirs...)`.
- **Fix sketch:** When `RuntimeManager == nil` (no hooks), skip both
  snapshots entirely. When hooks are active, diff field-by-field without
  cloning the slices — compare by length + element equality first, clone
  only on mismatch for the error message.
- **Expected win:** negligible per run, but removes 2 extra allocations
  per cold start, helpful for short `exec` runs.

### P2-D. `buildExecutionResult` rebuilds per-job info via append-copy for `CodeFiles`

- **File / line:** `internal/core/run/executor/result.go:62-81`.
- **Pattern:** `append([]string(nil), item.CodeFiles...)` per job, which
  is fine once, but the whole slice is then re-marshalled twice for JSON
  and RawJSON (indented and compact) at `emitExecutionResult`.
- **Fix sketch:** Marshal once to a shared buffer; keep the indented
  buffer only when `OutputFormat == JSON`. For `RawJSON` stream a
  compact encoder straight to stdout without the `MarshalIndent` pass.
- **Expected win:** 1× MarshalIndent + 1× Marshal instead of two full
  marshals. Run-ending cost; only matters for many short runs.

### P2-E. `exec.streamExecSession` ignores backpressure; its inner goroutine stalls the session when `emit` blocks on stdout

- **File / line:** `internal/core/run/exec/exec.go:448-466`.
- **Pattern:** For each update, calls `execution.Handler.HandleUpdate` +
  `state.emitSessionUpdate` sequentially. `emitSessionUpdate` marshals
  the entire update again and writes to stdout under
  `execEventWriter.mu`. Any slow reader on stdout stalls the goroutine
  and thereby the ACP update channel.
- **Fix sketch:** Use a bounded ring channel for stdout emits with a
  small worker. On overflow, coalesce like session updates (drop
  `AgentMessageChunk` frames but keep terminal frames) to protect the
  agent from back-pressure-deadlocks.
- **Expected win:** primarily correctness under slow-stdout conditions;
  minor CPU save.

### P2-F. `workflowEventStreamer.FinalizeAndStop` uses a fixed 5 s deadline via `time.After`

- **File / line:** `internal/core/run/executor/event_stream.go:95-112`.
- **Pattern:** `time.After(workflowEventStreamWaitTimeout)` leaks the
  timer if the streamer completes early. Negligible per run but a
  textbook pattern-smell.
- **Fix sketch:** Use `time.NewTimer` + `defer timer.Stop()`.
- **Expected win:** hygiene / GC pressure removal.

### P2-G. `reusableAgentMCPServerNames` allocates a slice even when no Stdio servers

- **File / line:** `internal/core/run/internal/acpshared/reusable_agent_lifecycle.go:110-123`.
- **Fix sketch:** Preallocate `names` with `len(servers)` when non-zero,
  `nil` otherwise; avoid the first `make([]string, 0, ...)` for the
  empty case. Negligible alone; group with P2-E as "kill allocations in
  lifecycle emitters" pass.

## Verification plan

Before implementing any change:

1. **Capture baseline** using `go test -bench=. -benchmem` in the
   following places (create benchmarks if missing):
   - `internal/core/run/internal/acpshared/BenchmarkHandleUpdate` —
     single update with 3 text blocks, no hooks. Measure
     allocs/op + ns/op.
   - `internal/core/run/executor/BenchmarkSubmitRuntimeEvent` — 1k
     events through `submitEventOrWarn`.
   - `internal/core/run/journal/BenchmarkJournalSubmit` — existing or
     new, 10k events + terminal, fsync on tmpfs.
   - `internal/core/extension/BenchmarkDispatchObserver` — 1 subscriber,
     noop invoker, large payload (`SessionUpdate` with 5 blocks).
2. **Profile** an end-to-end run that produces heavy session updates:
   `pprof` via `go tool pprof http://localhost:<debug>/debug/pprof/profile?seconds=30`
   during an `exec` run against a canned agent that emits 1k chunks.
   Alternatively, write a stub ACP client that streams N updates in
   `internal/core/run/exec/BenchmarkExecStreamThroughput` and capture
   CPU + alloc profiles. Verify hotspots match the ranking above before
   committing.
3. **Golden outputs:** capture
   - `events.jsonl` byte-identical for a deterministic run (use a fixed
     ACP stub, fixed time, fixed seqs).
   - Stdout of JSON/RawJSON streamer for the same run.
   - `run.json` result + per-turn `result.json`.
     Before + after SHA256 into `.compozy/tasks/perf/golden_checksums.txt`.
4. **Isomorphism proof per change:**
   - Ordering preserved: sequential in writeLoop, no reordering.
   - JSON bytes identical: use shared encoder config
     (`SetEscapeHTML(false)` must remain default-off today — verify).
   - Floating point: N/A (int aggregation).
   - RNG seeds: N/A.
5. **Regression suite:** `make verify` after each change; in addition to
   the benchmark suite, run
   `go test -race ./internal/core/run/... ./pkg/compozy/events/... ./internal/core/run/journal/...`.
6. **Rollback:** single-commit per change; `git revert <sha>` is the
   rollback contract. Stop after each landing and re-profile — the
   ranking will shift (e.g. once P0-A lands, the streamer syscall
   cost in P1-E will dominate).

## Opportunity Matrix (Impact × Confidence ÷ Effort)

| Finding | Impact | Confidence | Effort | Score |
|---|---|---|---|---|
| P0-A: triple-marshal of session updates | 5 | 4 | 3 | 6.67 |
| P0-B: observer dispatch clones + per-call goroutines | 5 | 4 | 3 | 6.67 |
| P0-C: reflection in `hasRuntimeEventSubmitter` | 2 | 5 | 1 | 10.0 |
| P1-A: hook payload triple-clones prompt/groups | 3 | 4 | 2 | 6.0 |
| P1-B: fsync-per-batch on journal | 3 | 3 | 3 | 3.0 |
| P1-C: unconditional `formatBlockTypes` | 3 | 5 | 1 | 15.0 |
| P1-D: waitChannel goroutine | 1 | 4 | 1 | 4.0 |
| P1-E: unbuffered stdout encoder | 3 | 4 | 2 | 6.0 |
| P1-F: MultiWriter without bufio | 3 | 4 | 2 | 6.0 |
| P1-G: composeSessionPrompt extra alloc | 2 | 5 | 1 | 10.0 |
| P2-A: usage mutex contention | 2 | 4 | 2 | 4.0 |
| P2-B: journal submit RWMutex | 2 | 3 | 2 | 3.0 |
| P2-C: prepared-state snapshot clones | 1 | 4 | 1 | 4.0 |
| P2-D: execution result double-marshal | 1 | 5 | 1 | 5.0 |
| P2-E: exec stream backpressure | 2 | 3 | 3 | 2.0 |
| P2-F: time.After leak | 1 | 5 | 1 | 5.0 |
| P2-G: reusableAgent names alloc | 1 | 5 | 1 | 5.0 |

> Rule: Only implement Score ≥ 2.0. Priority order for a first pass
> (highest score first, but only after profiling confirms the hotspot):
> **P1-C → P0-C → P1-G → P0-A → P0-B → P1-E → P1-F → P1-A → P2-F → P2-D.**
