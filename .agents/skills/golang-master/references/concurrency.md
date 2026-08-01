# Concurrency

Structured concurrency: every goroutine has a clear owner, a predictable exit, and error propagation. A goroutine is a liability until proven necessary — concurrency is added for measured need, not by default.

## The spawn checklist

Before every `go` statement, answer all five:

- **How does it exit?** — context cancellation, channel close, or explicit signal.
- **Can the owner stop it?** — it receives `ctx` or a done channel.
- **Can the owner wait for it?** — `sync.WaitGroup`, `errgroup`, or a result channel.
- **Who owns its channels?** — the creator/sender owns and closes.
- **Should this be synchronous instead?** — the strongest answer is often no goroutine at all.

## Channel rules

- Only the **sender** closes a channel; closing from the receiver panics on the sender's next write.
- Declare direction in signatures (`chan<- T`, `<-chan T`) — the compiler then enforces misuse.
- Default to **unbuffered**. A buffer size is a measured decision; large buffers mask backpressure and delay deadlock discovery.
- Send copies or immutable values. A pointer through a channel is invisible shared memory — it defeats the ownership transfer that justified the channel.
- Every blocking `select` in a goroutine includes `case <-ctx.Done(): return` — without it, cancellation cannot reach the goroutine.
- A nil channel blocks forever on both send and receive — useful deliberately (disabling a `select` case), a deadlock otherwise.

## Choosing the primitive

| Scenario | Use | Why |
| --- | --- | --- |
| Passing data between goroutines | Channel | Communicates ownership transfer |
| Coordinating lifecycle/shutdown | Channel + context | Clean exit via `select` |
| Protecting shared struct fields | `sync.Mutex` / `sync.RWMutex` | Simple critical sections |
| Counters, flags | `sync/atomic` typed atomics (`atomic.Int64`, `atomic.Bool`) | Lock-free, no ceremony |
| Read-heavy shared map | `sync.Map` | Concurrent plain-map access is a hard crash; prefer `RWMutex`+map when writes are frequent |
| Exactly-once init | `sync.Once` / `sync.OnceValue` (1.21+) | Safe lazy initialization |
| Deduplicating concurrent identical calls | `x/sync/singleflight` | Cache-stampede prevention |

| Waiting need | Use |
| --- | --- |
| Wait only, errors irrelevant | `sync.WaitGroup` — Go 1.25+: `wg.Go(fn)` replaces manual `Add`/`Done` |
| Wait + first error | `errgroup.Group` |
| Wait + cancel siblings on first error | `errgroup.WithContext` |
| Wait + bound concurrency | `errgroup` with `SetLimit(n)` |

Mutex discipline: keep critical sections short, never hold a lock across I/O, never upgrade `RLock` to `Lock` (deadlock), and store the mutex above the fields it guards.

## Worker pools and pipelines

`errgroup.SetLimit` replaces hand-rolled worker pools:

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(8)
for _, job := range jobs {
    g.Go(func() error {
        return process(ctx, job) // must honor ctx cancellation
    })
}
if err := g.Wait(); err != nil {
    return fmt.Errorf("processing batch: %w", err)
}
```

Pipeline stages own their output channel — create it, write to it, `defer close(out)`, and select on `ctx.Done()` for every send:

```go
func stage(ctx context.Context, in <-chan Job) <-chan Result {
    out := make(chan Result)
    go func() {
        defer close(out)
        for job := range in {
            select {
            case out <- transform(job):
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}
```

Go 1.23+ iterators (`iter.Seq[T]`) replace many channel-shaped generator pipelines with cheaper, cancellation-free pull iteration — reach for channels only when work is genuinely concurrent.

## Timers

`time.After` in a loop allocates a fresh timer per iteration and holds it until it fires. In long-running loops use one timer:

```go
t := time.NewTimer(interval)
defer t.Stop()
for {
    select {
    case <-t.C:
        tick()
        t.Reset(interval)
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

## Common mistakes

| Mistake | Fix |
| --- | --- |
| Fire-and-forget `go func()` | Give it a stop signal and an owner that waits |
| Closing a channel from the receiver | Only the sender closes |
| `wg.Add` inside the spawned goroutine | Call `Add` before `go` — `Wait` may otherwise return early (moot with 1.25 `wg.Go`) |
| Missing `ctx.Done()` case in `select` | Add it — cancellation must always be able to win |
| Unbounded spawning per request/item | `errgroup.SetLimit(n)` or a semaphore |
| Sharing a pointer via channel | Send a copy or transfer ownership explicitly and stop using it |
| Mutex held across I/O | Narrow the critical section to the shared-state touch |
| Races "handled" by sleeps | Fix the ordering with channels/locks; sleeps hide, never fix |

## Verification

- `go test -race ./...` on any package that spawns goroutines — always in CI, not just locally.
- `go.uber.org/goleak` in `TestMain` for packages that own goroutines: `goleak.VerifyTestMain(m)`.
- Runtime diagnosis: `runtime.NumGoroutine()`, stack dump at `/debug/pprof/goroutine?debug=2`. Go 1.26 adds an experimental goroutine-leak profile behind `GOEXPERIMENT=goroutineleakprofile`.
