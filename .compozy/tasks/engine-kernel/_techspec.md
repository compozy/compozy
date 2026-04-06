# TechSpec: Engine Kernel Refactor

## Executive Summary

This specification refactors Compozy's execution engine to introduce a Service Kernel, a typed Event Bus, a durable Journal, and a public Reader Library. The refactor is purely structural — no new user-facing features ship — but it unblocks four future phases: a daemon with JSON-RPC API (Phase B), a hook runtime for community extensibility (Phase C), client SDKs with OpenRPC (Phase D), and plugins (Phase E). Today the CLI calls `core.Run()` directly, a single `uiCh chan uiMsg` couples the executor to Bubble Tea, and events are silently dropped at three lossy sites. Every future ingress (RPC, hooks, SDK) would either replicate this coupling or require this refactor. The Codex architecture review and an earlier council session both converged on the same conclusion: ship the refactor first.

The primary technical trade-off is ~53 files touched and 3-5 person-weeks of refactor work now, in exchange for (a) eliminating the two-path execution problem before it exists, (b) producing a complete and durable event log as the canonical source of truth for all consumers, and (c) locking in a typed command contract and event taxonomy that external SDKs and hook authors will depend on for years. The alternative — layering a daemon and RPC server on top of the current code — would freeze TUI-era seams into the public SDK surface and make every future change require equivalence work across parallel call sites.

## System Architecture

### Component Overview

Five components are introduced, each in a dedicated package.

**Service Kernel** (`internal/core/kernel/`) — typed command dispatcher with per-operation handlers. Replaces direct calls from CLI to `core.Run()` and siblings. Accepts typed command structs, returns typed results, propagates `context.Context`. Future RPC server (Phase B) dispatches over the same kernel.

**Event Bus** (`pkg/compozy/events/` — public package) — generic in-process pub/sub with bounded per-subscriber channels and non-blocking fanout. Replaces the single `uiCh chan uiMsg`. Multiple subscribers (TUI, future RPC broadcast, future hook dispatcher) attach independently; slow subscribers degrade in isolation via per-subscriber drop accounting. Lives under `pkg/` because the Reader Library (also public) must be able to name `events.Event` types in its API.

**Journal** (`internal/core/run/journal/`) — single-writer-per-run that assigns monotonic sequence numbers, appends events to `.compozy/runs/<run-id>/events.jsonl`, batch-flushes to disk, and THEN publishes to the event bus. The journal is UPSTREAM of fanout, guaranteeing that live subscribers observe only events already persisted.

**Reader Library** (`pkg/compozy/runs/`) — public typed API for reading runs and tailing events. Uses `nxadm/tail` and `fsnotify`. First public package surface of Compozy; consumed by Phase B daemon, Phase D SDKs, debugging tools, CI integrations.

**Cobra Refactor** (`internal/cli/`) — existing CLI commands rewired to construct command structs and call `kernel.Dispatch`. The `commandState.runWorkflow` injection seam already exists; this refactor changes the injected function to call the kernel. Zero regression to user-visible CLI UX.

One side-fix is included: `internal/core/agent/session.go` buffer capacity grows from 128 to 1024 with timed backpressure and explicit drop logging (ADR-006), closing the most upstream lossy path.

### Data Flow

```
CLI command (Cobra)                          Service Kernel
     │                                             │
     │ construct RunStartCommand{...}              │
     │─────────────────────────────────────────────►
     │                                             │
     │                                    handler.Handle(ctx, cmd)
     │                                             │
     │                                    plan.Prepare() → RunArtifacts
     │                                    construct Journal
     │                                             │
     │                                    run.Execute(ctx, jobs, artifacts, cfg, journal)
     │                                             │
     │                                             ▼
     │                                    ACP session + worker pool
     │                                             │
     │                                    emit events ──► Journal.Submit(ev)
     │                                             │              │
     │                                             │     assign seq + append
     │                                             │     events.jsonl + batch flush
     │                                             │              │
     │                                             │              ▼
     │                                             │     Bus.Publish(enriched)
     │                                             │              │
     │                                             │    ┌─────────┼────────────┐
     │                                             │    ▼         ▼            ▼
     │                                             │   TUI    (Phase B:    (Phase C:
     │                                             │          RPC fanout)  hook dispatcher)
     │                                             │
     │                              return RunStartResult
     │◄────────────────────────────────────────────
```

## Implementation Design

### Core Interfaces

The kernel dispatcher uses Go 1.23 generics:

```go
package kernel // internal/core/kernel/

import (
    "context"
    "fmt"
    "reflect"
    "sync"
)

type Handler[C any, R any] interface {
    Handle(ctx context.Context, cmd C) (R, error)
}

type Dispatcher struct {
    mu       sync.RWMutex
    handlers map[reflect.Type]any
}

func NewDispatcher() *Dispatcher {
    return &Dispatcher{handlers: make(map[reflect.Type]any)}
}

func Register[C any, R any](d *Dispatcher, h Handler[C, R]) {
    var zero C
    d.mu.Lock()
    d.handlers[reflect.TypeOf(zero)] = h
    d.mu.Unlock()
}

func Dispatch[C any, R any](ctx context.Context, d *Dispatcher, cmd C) (R, error) {
    d.mu.RLock()
    h, ok := d.handlers[reflect.TypeOf(cmd)]
    d.mu.RUnlock()
    var zero R
    if !ok {
        return zero, fmt.Errorf("no handler for %T", cmd)
    }
    typed, ok := h.(Handler[C, R])
    if !ok {
        return zero, fmt.Errorf("handler type mismatch for %T", cmd)
    }
    return typed.Handle(ctx, cmd)
}
```

The event bus allocates a subscription struct per subscriber, each carrying its own drop counter. Publish snapshots subscribers under RLock, releases, then sends without holding the lock:

```go
package events // pkg/compozy/events/

import (
    "context"
    "sync"
    "sync/atomic"
)

type SubID uint64

type subscription[T any] struct {
    id           SubID
    ch           chan T
    dropped      atomic.Uint64
    lastWarnedAt atomic.Int64
}

type Bus[T any] struct {
    mu      sync.RWMutex
    subs    map[SubID]*subscription[T]
    nextID  SubID
    bufSize int
    closed  atomic.Bool
}

func New[T any](bufSize int) *Bus[T] {
    return &Bus[T]{subs: make(map[SubID]*subscription[T]), bufSize: bufSize}
}

func (b *Bus[T]) Subscribe() (SubID, <-chan T, func()) {
    b.mu.Lock()
    b.nextID++
    id := b.nextID
    sub := &subscription[T]{id: id, ch: make(chan T, b.bufSize)}
    b.subs[id] = sub
    b.mu.Unlock()
    return id, sub.ch, func() { b.unsubscribe(id) }
}

func (b *Bus[T]) Publish(ctx context.Context, evt T) {
    b.mu.RLock()
    snap := make([]*subscription[T], 0, len(b.subs))
    for _, s := range b.subs {
        snap = append(snap, s)
    }
    b.mu.RUnlock()
    for _, s := range snap {
        select {
        case s.ch <- evt:
        case <-ctx.Done():
            return
        default:
            s.dropped.Add(1) // rate-limited warn elsewhere
        }
    }
}

func (b *Bus[T]) DroppedFor(id SubID) uint64 { /* lookup + read atomic */ }
func (b *Bus[T]) Close(ctx context.Context) error { /* unsubscribe all, close all */ }
```

The journal is a single-writer goroutine per run. The writer owns `seq` as a plain local variable (no atomics — only the writer touches it):

```go
package journal // internal/core/run/journal/

import (
    "bufio"
    "context"
    "encoding/json"
    "os"
    "sync"

    "github.com/compozy/compozy/pkg/compozy/events"
)

type Journal struct {
    path  string
    inbox chan events.Event
    bus   *events.Bus[events.Event]
    done  chan struct{}
    once  sync.Once
}

func Open(path string, bus *events.Bus[events.Event], bufCap int) (*Journal, error) {
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
    if err != nil {
        return nil, err
    }
    j := &Journal{
        path:  path,
        inbox: make(chan events.Event, bufCap),
        bus:   bus,
        done:  make(chan struct{}),
    }
    go j.writeLoop(f) // writer owns seq as loop-local variable
    return j, nil
}

func (j *Journal) writeLoop(f *os.File) {
    defer close(j.done)
    defer f.Close()
    buf := bufio.NewWriterSize(f, 16<<10)
    enc := json.NewEncoder(buf)
    var seq uint64 // writer-owned, no atomics needed
    // ...loop: read from inbox, assign seq, enc.Encode, batch flush, bus.Publish
}

func (j *Journal) Submit(ctx context.Context, ev events.Event) error { /* send to inbox with ctx timeout */ }
func (j *Journal) Close(ctx context.Context) error { /* signal writer, wait for done or ctx */ }
```

### Data Models

Event envelope (serialized as one JSON line in `events.jsonl`):

```go
package events // pkg/compozy/events/

import (
    "encoding/json"
    "time"
)

type Event struct {
    SchemaVersion string          `json:"schema_version"`
    RunID         string          `json:"run_id"`
    Seq           uint64          `json:"seq"`
    Timestamp     time.Time       `json:"ts"`
    Kind          EventKind       `json:"kind"`
    Payload       json.RawMessage `json:"payload"`
}

type EventKind string
```

Command structs live in `internal/core/kernel/commands/`, one file per domain:

```go
type RunStartCommand struct {
    WorkspaceRoot string
    Mode          model.ExecutionMode
    Name          string
    IDE           string
    Model         string
    PromptText    string
    PromptFile    string
    // ...scope-limited to fields relevant to starting a run
}

type RunStartResult struct {
    RunID        string
    ArtifactsDir string
    Status       string
}
```

Run summary exposed by the reader library:

```go
package runs

type RunSummary struct {
    RunID        string
    Status       string
    Mode         string
    IDE          string
    Model        string
    WorkspaceRoot string
    StartedAt    time.Time
    EndedAt      *time.Time
    ArtifactsDir string
}
```

On-disk artifact layout (unchanged from exec-command task_02):

```
.compozy/runs/<run-id>/
├── run.json              # RunSummary + config snapshot
├── events.jsonl          # append-only event log (NEW for non-exec modes)
├── result.json           # final result (existing)
├── jobs/
│   ├── <safe-name>.prompt.md
│   ├── <safe-name>.out.log
│   └── <safe-name>.err.log
└── turns/                # exec mode only
```

### API Endpoints

Not applicable in Phase A. The kernel exposes typed Go APIs; Cobra wraps them in CLI flags; future phases wrap them in JSON-RPC.

## Integration Points

**ACP runtime registry** (`internal/core/agent/registry.go`) — unchanged. The kernel's `RunStart` handler continues to use `agent.EnsureAvailable()` for runtime validation. Agent specs for claude, codex, droid, cursor, opencode, pi, gemini, copilot stay exactly as today.

**Workspace config** (`internal/core/workspace/config.go`) — unchanged. Config merging (CLI flags > `[<section>]` > `[defaults]` > runtime defaults) is consumed by CLI commands before constructing kernel command structs. Kernel commands themselves receive resolved values.

**Filesystem artifacts** (`.compozy/runs/<run-id>/`) — extended. Every run mode now writes `events.jsonl` (previously only exec mode did). `run.json` and `result.json` writers are unchanged.

**Provider APIs** (GitHub, CodeRabbit) — wrapped. Calls from `resolveProviderBackedIssues()` now emit `provider.call_started` / `provider.call_completed` / `provider.call_failed` events in addition to their existing behavior.

**Bubble Tea TUI** (`internal/core/run/ui_model.go`) — modified. The TUI becomes an event bus subscriber instead of reading from `uiCh` directly. Its message types and render logic are unchanged; only the event source changes.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| `pkg/compozy/events/` (new, public) | new | `Bus[T]`, `Event` envelope, `EventKind`, `SubID` types. Low risk. | Create package, ~200 LOC + tests |
| `pkg/compozy/events/kinds/` (new, public) | new | Payload structs per domain (run, job, session, tool_call, usage, task, review, provider, shutdown). Low risk. | Create 9 Go files, one per domain |
| `pkg/compozy/runs/` (new, public) | new | `List`, `Open`, `Replay`, `Tail`, `WatchWorkspace`, `RunSummary`, `RunEvent`, `RunEventKind`. Medium risk (public API stability). | Create public package with nxadm/tail + fsnotify |
| `internal/core/kernel/` (new) | new | `Dispatcher`, `Handler[C,R]`, `Register`, `Dispatch`, `KernelDeps`, `BuildDefault`. Low risk. | Create package, register 6 Phase A handlers |
| `internal/core/kernel/commands/` (new) | new | 6 typed command/result structs (RunStart, WorkflowPrepare, WorkflowSync, WorkflowArchive, WorkspaceMigrate, ReviewsFetch) + `*FromConfig` translators. Low risk. | 6 command files |
| `internal/core/run/journal/` (new) | new | Per-run writer goroutine, writer-owned seq, batch flush, terminal fsync. Medium risk (durability semantics). | Writer loop + tests covering ordering, crash, ctx cancel |
| `internal/core/api.go:202-251` | modified | Six exported functions (`Prepare`, `Run`, `FetchReviews`, `Migrate`, `Sync`, `Archive`) become internal adapters wrapping kernel.Dispatch. `core.Config` stays transitional. Medium risk. | Rewrite exports as thin dispatcher adapters |
| `internal/core/run/execution.go:27-112` | modified | `Execute` accepts journal + bus; replaces `uiCh` sends at lines 300, 312, 333, 335, 359, 388, 610 with `journal.Submit`. High risk (core executor). | Rewire 7 send sites; thread journal through |
| `internal/core/run/execution.go:733-824` | modified | `afterTaskJobSuccess` + `afterReviewJobSuccess` + `resolveProviderBackedIssues` emit `task.*`, `review.*`, `provider.*` events via journal (emit-after-success policy per ADR-003). Medium risk. | Add 8-10 journal.Submit call sites |
| `internal/core/run/logging.go:83-130` | modified | `HandleUpdate` emits `session.update` events via journal; removes 2 drop sites at lines 105 and 117. High risk. | Rewrite to use journal; remove uiCh direct writes |
| `internal/core/run/exec_flow.go` | modified | Exec mode's existing events.jsonl writer is replaced by the shared journal (same file path, same format). Medium risk. | Remove custom writer; use journal.Submit |
| `internal/core/run/command_io.go:95` | modified | `uiMsg`-related wiring re-targeted at TUI bus subscriber. Low risk. | Update imports/types |
| `internal/core/run/types.go:226` | modified | `uiMsg` types become TUI-private; event domain types live in `pkg/compozy/events/kinds/`. Medium risk. | Split types; TUI keeps `uiMsg` internally |
| `internal/core/run/ui_model.go:24-121` | modified | TUI becomes bus subscriber; translates `events.Event` to internal `uiMsg` via adapter; tea.Model logic unchanged. Medium risk. | Add bus-to-uiMsg adapter goroutine |
| `internal/core/run/ui_view.go`, `ui_update.go` | unchanged | Rendering and message handling untouched. | None |
| `internal/core/agent/session.go:59,102` | modified | Grow buffer 128→1024 (line 59); replace `select { default: }` with timed backpressure + drop logging (line 102). `publish` gains `ctx context.Context` param; callers updated. Medium risk. | Per ADR-006 |
| `internal/core/agent/client.go` | unchanged | `SessionRequest` shape unchanged. | None |
| `internal/core/plan/prepare.go:24` | modified | `Prepare` constructs the journal alongside run artifacts; returns journal handle in `SolvePreparation`. Low risk. | Add journal.Open call |
| `internal/core/model/model.go:155` | unchanged | `RunArtifacts.EventsPath` already defined. | None |
| `internal/core/model/content.go:153` | unchanged | `SessionUpdate` becomes payload of `session.update` event. | None |
| `internal/core/workspace/config.go` | unchanged | Config loading unaffected. | None |
| `internal/core/fetch.go:19` | modified | `FetchReviews` call becomes `ReviewsFetch` handler body. Low risk. | Thin refactor |
| `internal/core/migrate.go`, `internal/core/sync.go`, `internal/core/archive.go` | modified | Each wrapped in a kernel handler. Low risk. | Create handlers |
| `internal/core/tasks/store.go:43` | modified | `MarkTaskCompleted` + `RefreshTaskMeta` callers emit `task.file_updated`, `task.metadata_refreshed` (emit-after-success). Low risk. | Add journal.Submit at call sites in execution.go, NOT in store.go |
| `internal/core/reviews/store.go:198` | modified | `FinalizeIssueStatuses` + `RefreshRoundMeta` callers emit `review.status_finalized`, `review.round_refreshed`. Low risk. | Add journal.Submit at call sites in execution.go |
| `internal/cli/root.go:72-107,886` | modified | Construct `KernelDeps`, call `kernel.BuildDefault`, pass dispatcher into each command constructor. `commandState.runWorkflow` signature PRESERVED (dispatcher captured via closure). Medium risk. | Bootstrap dispatcher in root; update each command constructor signature |
| `internal/cli/*.go` (9 command files: fetch-reviews, fix-reviews, start, exec, migrate, sync, archive, validate-tasks, setup) | modified | Each command's `runWorkflow` closure calls `kernel.Dispatch` with its typed command. Pattern is mechanical. Medium risk. | 9 closure refactors |
| `go.mod` | modified | Add `github.com/nxadm/tail`, `github.com/fsnotify/fsnotify`. Low risk. | `go get`; `go mod tidy` |
| `docs/events.md` (new) | new | Canonical public reference for event taxonomy. Low risk. | Generate from kinds/ Go structs |
| `docs/reader-library.md` (new) | new | Usage examples for `pkg/compozy/runs/`. Low risk. | Write with tested examples |
| Tests: `internal/core/run/execution_test.go` | modified | Replace `uiCh` assertions with bus subscription + journal assertions. High risk (test churn ~15 cases). | Rewrite |
| Tests: `internal/core/run/execution_ui_test.go:60` | modified | Assert via event bus subscription → uiMsg adapter. Medium risk. | Rewrite |
| Tests: `internal/core/run/logging_test.go` | modified | Test journal write semantics instead of `uiCh` drops. Medium risk. | Rewrite |
| Tests: `internal/core/run/execution_acp_test.go` | modified | Mock event bus + journal. Medium risk. | Update mocks |
| Tests: `internal/core/run/ui_update_test.go:96` | modified | Update for bus-to-uiMsg adapter. Low risk. | Update fixtures |
| Tests: `internal/core/agent/session_test.go` | modified | Add backpressure/drop/timeout tests per ADR-006. Low risk. | Add 3 test cases |
| Tests: `internal/cli/root_test.go` | modified | Inject mock dispatcher via `KernelDeps`. Medium risk. | Refactor test injection |
| Tests: `internal/cli/root_command_execution_test.go:90` | modified | Narrow parity checks to stable external contracts (stdout JSON, artifact files). Medium risk. | Update assertions |
| Tests: new kernel/events/journal/runs tests | new | Unit + integration coverage for all new packages. Low risk. | ~20 new test files |

**Summary**: 6 new packages (`pkg/compozy/events/`, `pkg/compozy/events/kinds/`, `pkg/compozy/runs/`, `internal/core/kernel/`, `internal/core/kernel/commands/`, `internal/core/run/journal/`), 15 modified files in `internal/core/`, 10 modified CLI files, 8 existing test files modified, ~20 new test files, 2 new dependencies, 2 new documentation files. Approximately 53 files.

## Testing Approach

### Unit Tests

- **Kernel dispatcher**: register multiple handlers; assert Dispatch routes to correct handler; assert type mismatch returns error; assert concurrent Register and Dispatch are race-free.
- **Kernel registry self-test** (NEW): `BuildDefault` is called; test asserts all 6 Phase A command types are registered (RunStart, WorkflowPrepare, WorkflowSync, WorkflowArchive, WorkspaceMigrate, ReviewsFetch). Fails loudly if any handler is forgotten.
- **Event bus**: publish to N subscribers; assert fanout reaches all; assert slow subscriber drops without blocking publisher; assert per-subscriber `dropped` counter increments correctly; assert `DroppedFor(id)` returns accurate count.
- **Event bus unsubscribe-during-publish** (NEW): start N subscribers, publisher fires continuously, randomly call unsubscribe on some; assert no panic, no leaked goroutines, unsubscribed channels are closed exactly once.
- **Event bus Close** (NEW): Close with active subscribers; assert all channels closed; assert Close is idempotent; assert Close unblocks pending publishers via ctx.
- **Event bus goroutine leak** (NEW): after N Subscribe+unsub cycles, assert goroutine count has not grown (via `runtime.NumGoroutine` before/after with stabilization pause).
- **Journal writer**: submit events concurrently; assert seq numbers monotonically increase 1,2,3… with no gaps; assert batch flush fires on threshold and interval; assert terminal events force immediate sync; assert Close drains queued events.
- **Journal crash recovery** (NEW): inject a `flushHook func()` test seam in journal writer; simulate crash between write and flush; re-read events.jsonl via reader library; assert all events up to last flush are parseable; assert any trailing partial-line is tolerated.
- **Reader library — List**: scan directory with N runs; assert ListOptions filters work (status, mode, time range); assert missing run.json is skipped with warning.
- **Reader library — Replay**: write known events.jsonl, replay from seq=0 and seq=N; assert yielded events match; assert malformed trailing line is tolerated.
- **Reader library — Tail**: start tail mid-stream via nxadm/tail; write new events; assert live delivery; assert rotation is detected.
- **Reader library — WatchWorkspace** (NEW): subscribe to workspace via fsnotify; create new run subdirectory; assert `RunEvent{Kind: RunEventCreated}` fires; modify run.json status field; assert `RunEvent{Kind: RunEventStatusChanged}` fires; remove run directory; assert `RunEvent{Kind: RunEventRemoved}` fires.
- **ACP buffer (ADR-006)**: slow consumer causes buffer to fill; assert fast-path sends succeed; assert backpressure blocks with timeout; assert drop counter increments on timeout; assert ctx cancel returns cleanly.
- **Event taxonomy round-trip**: each EventKind has a payload struct; each payload struct round-trips through json.Marshal/Unmarshal; schema_version present in envelope; unknown fields tolerated by consumer (forward compat).
- **Schema version compatibility** (NEW): write events with schema_version="1.0" + future additive field; reader library parses successfully and exposes core fields; write events with schema_version="99.0"; assert reader library returns a typed error caller can branch on.

### Integration Tests

- **Kernel → journal → reader roundtrip**: invoke RunStart handler with fake ACP runtime; assert events.jsonl is written with expected kinds and payloads; open run via reader library; assert Replay produces the same events.
- **CLI → kernel parity**: run `compozy start --name foo`, `compozy exec ...`, `compozy fix-reviews ...` through refactored commands; assert stable external contracts match pre-refactor baselines: (a) process exit code, (b) stdout JSON shape (when `--format json`), (c) presence and content of `run.json`, `result.json`, `events.jsonl`, per-job `.prompt.md`, `.out.log`, `.err.log` artifacts, (d) no regression against existing assertions in `internal/cli/root_command_execution_test.go:90` (stdout/JSON/artifact contracts) and `internal/core/run/execution_ui_test.go:60` (UI message sequence). TUI visual output is NOT pinned byte-for-byte.
- **Post-execution events**: complete a PRD task job; assert `task.file_updated` and `task.metadata_refreshed` events appear in events.jsonl. Complete a review job; assert `review.status_finalized`, `review.issue_resolved`, `review.round_refreshed` events appear. Assert provider calls emit `provider.call_*` events with latency and status.
- **Crash recovery**: write events, simulate crash (exit before final flush); re-read events.jsonl via reader library; assert events within last batch interval may be missing but all earlier events are present and parseable.
- **Concurrent runs (unit + integration)**: run two RunStart invocations concurrently (simulated kernel calls); assert each has its own journal and events.jsonl; assert no cross-contamination of seq numbers.

## Development Sequencing

### Build Order

1. **Event types + taxonomy** (`pkg/compozy/events/`, `pkg/compozy/events/kinds/`) — no dependencies. Define `Event`, `EventKind`, `SubID`, payload structs per domain (run/job/session/tool_call/usage/task/review/provider/shutdown). Unit tests for serialization round-trip.
2. **Event bus** (`pkg/compozy/events/bus.go`) — depends on step 1. Implement `Bus[T]`, per-subscriber subscription struct with drop counter, snapshot-and-publish, `Close(ctx)`. Unit tests: fanout, backpressure, unsubscribe-during-publish, leak detection, Close idempotency.
3. **Journal writer** (`internal/core/run/journal/`) — depends on steps 1, 2. Writer goroutine owns seq as loop-local, batch flush on size/interval, fsync on terminal, ctx-aware Close. Unit tests: ordering, crash recovery (via injectable flush hook), ctx cancel, concurrent Submit.
4. **ACP buffer fix** (`internal/core/agent/session.go`) — no dependencies. Grow buffer 128→1024, `publish(ctx, update)` with 5s timed backpressure + per-session drop counters (ADR-006). Unit tests: fast path, backpressure path, timeout path, ctx cancel path.
5. **Service Kernel scaffolding** (`internal/core/kernel/`) — no dependencies. `Dispatcher`, `Handler[C,R]`, `Register`, `Dispatch`, `KernelDeps`, `BuildDefault`. Unit tests: type-keyed routing, registry self-test (asserts all 6 Phase A commands registered), concurrent Register/Dispatch race-free.
6. **Command structs + translators** (`internal/core/kernel/commands/`) — depends on step 5. Define 6 Phase A commands (RunStart, WorkflowPrepare, WorkflowSync, WorkflowArchive, WorkspaceMigrate, ReviewsFetch) with typed input/output structs + `*FromConfig(cfg core.Config)` translators. Unit tests for each translator.
7. **Event emitter integration** (`internal/core/run/execution.go`, `logging.go`) — depends on steps 2, 3, 4, 5. Thread journal + bus through `run.Execute` and `HandleUpdate`. Replace 7 `uiCh` send sites at `execution.go:300,312,333,335,359,388,610` with `journal.Submit`. Rewrite `logging.go:83` to emit `session.update` via journal (removes both lines 105, 117 drop sites). No TUI changes yet — bus has zero subscribers in this step.
8. **TUI bus-to-uiMsg adapter** (`internal/core/run/ui_model.go`) — depends on step 7. Add an adapter goroutine that subscribes to the bus, translates `events.Event` to internal `uiMsg` types, pipes into existing TUI channel. Keep `tea.Model` (`ui_update.go`, `ui_view.go`) unchanged. The executor and TUI run decoupled after this step.
9. **Final `uiCh` removal from executor** (`internal/core/run/execution.go`, `types.go`, `command_io.go`) — depends on step 8. Delete the now-unused `uiCh chan uiMsg` field and related wiring. TUI owns its own channel internally.
10. **Post-execution event emission** (`internal/core/run/execution.go:733-824`) — depends on steps 1, 7. Instrument `afterTaskJobSuccess` → `task.file_updated`, `task.metadata_refreshed` (emit-after-success per ADR-003). Instrument `afterReviewJobSuccess` + `resolveProviderBackedIssues` → `review.status_finalized`, `provider.call_started/completed/failed`, `review.issue_resolved`, `review.round_refreshed`. Integration tests assert full event sequences.
11. **Reader library** (`pkg/compozy/runs/`) — depends on step 1. Add `nxadm/tail` + `fsnotify` dependencies via `go get`. Implement `List`, `Open`, `Replay` (iter.Seq2), `Tail` (nxadm/tail over events.jsonl), `WatchWorkspace` (fsnotify on runs directory). Unit + integration tests including WatchWorkspace subdirectory-create detection.
12. **Cobra refactor + kernel bootstrapping** (`internal/cli/*`) — depends on steps 5, 6, 7, 8. Construct `KernelDeps` at root (logger, bus, workspace ctx, agent registry), call `kernel.BuildDefault`, pass dispatcher into each of the 9 command constructors. Each constructor wires `runWorkflow` closure that translates flags → typed command → `kernel.Dispatch`. Integration tests verify behavioral parity on stdout contracts, artifact files, exit codes.
13. **Legacy `core.Config` cleanup** (`internal/core/api.go`) — depends on step 12. Once all callers use typed commands, rewrite `core.Run`/`core.Prepare`/etc. as thin adapters that build typed commands and call kernel.Dispatch. Mark `core.Config` as transitional in doc comments; public external migration documented for eventual removal.
14. **Documentation** — depends on all prior steps. Generate `docs/events.md` from `pkg/compozy/events/kinds/` structs; write `docs/reader-library.md` with tested examples; update CLAUDE.md with new package paths.

### Technical Dependencies

- `github.com/nxadm/tail` (new) — file tailing with rotation/truncation handling.
- `github.com/fsnotify/fsnotify` (new direct; already transitive via nxadm/tail) — directory watching for new runs.
- Go 1.23+ for generics, `iter.Seq2`, and `log/slog` attribute-based filtering.
- Existing dependencies unchanged: cobra, bubbletea, acp-go-sdk, go-toml, yaml.

## Monitoring and Observability

**Structured log fields** (via `log/slog`):
- Every kernel command dispatch: `component=kernel`, `command=<type>`, `duration_ms`, `error`
- Every journal write: `component=journal`, `run_id`, `seq`, `bytes`, `flush_latency_ms`
- Every bus drop: `component=bus`, `subscriber_id`, `event_kind`, `dropped_total`
- Every ACP buffer backpressure or drop: `component=acp`, `session_id`, `slow_publishes`, `dropped_updates`

**Metrics** (counters and gauges exposed via structured logs now; Prometheus scrape in Phase B):
- `kernel_command_dispatches_total{command}` — counter
- `kernel_command_duration_seconds{command}` — histogram
- `events_published_total{kind}` — counter
- `events_dropped_total{subscriber_id}` — counter
- `journal_events_written_total{run_id}` — counter
- `journal_buffer_depth{run_id}` — gauge
- `journal_flush_duration_seconds` — histogram
- `acp_slow_publishes_total{session_id}` — counter
- `acp_dropped_updates_total{session_id}` — counter

**Failure investigation flow**:
1. User reports "the TUI froze" → check `events_dropped_total{subscriber_id=<tui>}`
2. User reports "run history incomplete" → check `acp_dropped_updates_total{session_id=<x>}` and `journal_drops_total`
3. User reports "run hung on a job" → check `kernel_command_duration_seconds{command=RunStart}` and `journal_buffer_depth`
4. Operator wants provider call audit → `grep provider.call_completed events.jsonl | jq`

## Technical Considerations

### Key Decisions

- **Decision**: Introduce a Service Kernel layer that owns typed command dispatch, replacing direct calls from CLI to `core.Run()`.
  **Rationale**: One execution path for current and future ingresses, eliminating the two-path equivalence problem before it exists.
  **Trade-offs**: Refactor cost in test rewiring and CLI flag-to-command mapping; slightly more ceremony per new command.
  **Alternatives rejected**: Keep `core.Run()` as the shared seam (two-path problem); single `Command` interface with type union (loses compile-time safety); mediator framework (`go-mediatr`, overkill).

- **Decision**: Build a custom typed event bus with bounded per-subscriber channels and non-blocking publish.
  **Rationale**: Backpressure policy is load-bearing and must not block publishers; custom cost is ~150 LOC.
  **Trade-offs**: Drops are silent unless metrics are observed; backpressure policy is hardcoded.
  **Alternatives rejected**: `watermill` (framework scale), `cskr/pubsub/v2` (blocks publisher), `asaskevich/EventBus` (reflection, no generics), NATS embedded (15MB bloat for local CLI).

- **Decision**: Make the journal upstream of event bus fanout; events are durable before any subscriber sees them.
  **Rationale**: `events.jsonl` must be canonical source of truth; fast-first breaks replay on crash.
  **Trade-offs**: Small latency between emit and subscriber receipt (batch flush interval); journal becomes throughput bottleneck.
  **Alternatives rejected**: Journal as parallel subscriber (fast-first, loses events on crash); fsync per event (too slow); persistence inside subscribers (no canonical order).

- **Decision**: Extend event taxonomy to include post-execution mutations and provider I/O, with explicit emission ordering policy (emit-after-success for mutations, emit-on-attempt+completion for provider calls) and warn-and-continue provider failure policy.
  **Rationale**: Incomplete taxonomy means replay cannot reconstruct run side effects; consumers lose trust in the log. Ordering and failure policy must be explicit or future code regresses toward silent drift.
  **Trade-offs**: More event kinds to maintain; more emission call sites in existing code.
  **Alternatives rejected**: Minimal taxonomy with ad-hoc additions (breaking changes per addition); protobuf schema (binary format loses NDJSON inspectability); fail-run-on-provider-failure (punishes user for external infra outage).

- **Decision**: Use `nxadm/tail` + `fsnotify` in the reader library rather than custom tailer.
  **Rationale**: Rotation and truncation edge cases are hard; these libraries solve them with community track record (Ginkgo uses nxadm/tail).
  **Trade-offs**: Two new direct dependencies; nxadm/tail carries `tomb.v1` transitive.
  **Alternatives rejected**: Custom tailer (~100 LOC, risks rotation bugs); `rjeczalik/notify` (unneeded recursion); raw file access (pushes complexity to every consumer).

- **Decision**: Grow ACP ingress buffer to 1024, add 5-second backpressure deadline, emit drop metrics.
  **Rationale**: Silent drops at the most upstream point invalidate journal's source-of-truth claim.
  **Trade-offs**: Publisher may block up to 5s on pathological slowness.
  **Alternatives rejected**: Keep 128 with silent drop (observability failure); unbounded queue (memory risk); synchronous publish (couples ACP loop to journal I/O).

### Known Risks

- **Test churn**: ~23 existing test files assert on `uiCh` semantics or executor internals that change shape. Mitigation: test-first refactor pattern — write new tests against new APIs, then migrate old tests, then delete obsolete assertions. Allocate ~30% of refactor effort to testing.

- **Behavioral drift during refactor**: executor behavior may change subtly when `uiCh` sends are replaced with journal.Submit + bus.Publish. Mitigation: integration test harness that exercises `compozy start`, `compozy exec`, `compozy fix-reviews` end-to-end and compares artifact contents (run.json, result.json, event counts) against a recorded baseline before refactor.

- **Public API stability for reader library**: `pkg/compozy/runs/` is public; early consumers will churn during Phase B integration. Mitigation: keep package at `v0.x` until Phase B daemon validates it; communicate breaking changes in CHANGELOG; stabilize to `v1.0` after Phase B ships.

- **Journal buffer overflow under ACP bursts**: default 1024-event submit buffer may fill during intense tool-call streaming. Mitigation: 5s backpressure + explicit drop counter (same as ACP buffer). Monitor `journal_buffer_depth` metric.

- **Schema versioning discipline**: contributors may bump minor version for breaking changes. Mitigation: require CHANGELOG entry for every schema bump; CI diff check between PRs for schema drift.

- **Event emission gap in post-execution helpers**: adding journal.Submit calls inside `afterTaskJobSuccess`, `afterReviewJobSuccess`, provider helpers is tedious and error-prone. Mitigation: write a table-driven integration test that asserts a mapping of {operation → expected events} for each mode.

- **Provider call failure behavior**: v1 policy is warn-and-continue (defined in ADR-003). A code change that elevates provider failures to run-fatal is a breaking contract change. Mitigation: test case pins the current behavior; any code path that returns error from `resolveProviderBackedIssues` fails that test.

- **Mutation/event ordering drift**: if a future contributor emits `task.file_updated` BEFORE `MarkTaskCompleted` writes to disk, replay consumers reconstruct inconsistent state. Mitigation: ADR-003 documents emit-after-success as the default policy; every new mutation event emission is code-reviewed against that policy.

- **Forgotten handler registration**: a new command defined without `Register` call in `BuildDefault` fails only at runtime when Dispatch is called. Mitigation: kernel registry self-test (unit) asserts exhaustive registration of all 6 Phase A commands; same pattern extends when Phase B adds commands.

- **Events package public API churn**: `pkg/compozy/events/` is public and consumed by `pkg/compozy/runs/` + future phases. Breaking changes cascade. Mitigation: keep packages at `v0.x` until Phase B daemon validates them; add semver discipline via CI schema diff checks.

## Architecture Decision Records

- [ADR-001: Service Kernel Pattern with Typed Per-Command Handlers](adrs/adr-001.md) — Introduces a typed command dispatcher at `internal/core/kernel/` so CLI and future RPC share one execution layer.
- [ADR-002: Custom Event Bus with Bounded Per-Subscriber Backpressure](adrs/adr-002.md) — Builds a ~150 LOC generic event bus with non-blocking fanout and drop metrics rather than adopting watermill, cskr/pubsub, or NATS.
- [ADR-003: Event Taxonomy with Schema Versioning and Complete Side-Effect Coverage](adrs/adr-003.md) — Defines 27 event kinds across 9 domains including post-execution mutations and provider I/O so `events.jsonl` is a complete source of truth.
- [ADR-004: Journal Upstream of Fanout with Single-Writer Per-Run Model](adrs/adr-004.md) — Assigns seq numbers and appends to `events.jsonl` BEFORE bus fanout, making durability a precondition of live delivery.
- [ADR-005: Reader Library over `.compozy/runs/` using nxadm/tail and fsnotify](adrs/adr-005.md) — Publishes `pkg/compozy/runs/` as the first public API, with typed List/Open/Replay/Tail/WatchWorkspace operations.
- [ADR-006: ACP Ingress Buffer with Grown Capacity and Timed Backpressure](adrs/adr-006.md) — Grows `sessionImpl.updates` from 128 to 1024 and replaces silent drop with 5-second backpressure plus explicit metrics.
