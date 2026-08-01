# Context

`context.Context` is the session of a unit of work — it ties together every operation belonging to the same request so one cancellation reaches all of them. The core rule: **one context, propagated through the entire call chain**; breaking the chain orphans everything downstream.

## Creating

| Situation | Use |
| --- | --- |
| Process entry point (`main`, `TestMain`) | `context.Background()` |
| Inside an HTTP handler | `r.Context()` |
| Context needed but not yet threaded through | `context.TODO()` — a visible marker to fix, never a design choice |
| Need manual cancellation | `context.WithCancel(parent)` |
| Need a relative timeout | `context.WithTimeout(parent, d)` |
| Need an absolute deadline | `context.WithDeadline(parent, t)` |
| Background work that outlives the request | `context.WithoutCancel(parent)` (1.21+) |

Rules:

- `ctx context.Context` is the first parameter. Never store a context in a struct — it freezes one request's lifetime into an object that outlives it; pass it per call.
- `context.Background()` appears only at entry points. A fresh `Background()` mid-path silently detaches downstream work from cancellation — the classic source of queries running after the client is gone.
- Never pass `nil`; use `context.TODO()` while wiring is incomplete.
- Always use the `*Context` API variants when they exist: `QueryContext`, `ExecContext`, `NewRequestWithContext`.

## Cancellation discipline

- `defer cancel()` immediately on the line after `WithCancel`/`WithTimeout`/`WithDeadline` — a lost cancel leaks the context's timer and children until the parent dies. The only exception is deliberately returning the cancel to a caller who takes ownership.
- Every external call gets a timeout — an upstream with no deadline hangs a goroutine forever:

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
resp, err := client.Do(req.WithContext(ctx))
```

- Retry loops check cancellation between attempts and back off via `select`:

```go
for attempt := range maxRetries {
    if err = call(ctx); err == nil {
        return nil
    }
    select {
    case <-time.After(backoff(attempt)):
    case <-ctx.Done():
        return fmt.Errorf("retrying: %w", ctx.Err())
    }
}
```

- Long CPU-bound loops poll `ctx.Err()` periodically — cancellation only works where code listens for it.
- `context.AfterFunc(ctx, fn)` (1.21+) registers cleanup to run on cancellation without dedicating a goroutine to `<-ctx.Done()`.

## Detached work

Work that must survive the request (audit log, cache fill, async notification) uses `context.WithoutCancel(parent)`: values and tracing survive, cancellation does not. `WithoutCancel` drops the deadline too — re-attach a bound with `WithTimeout` so detached work cannot run forever:

```go
bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
go func() {
    defer cancel()
    audit.Record(bg, event)
}()
```

## Values

Context values are for **request-scoped metadata that crosses API boundaries** — request ID, authenticated principal, trace context. Anything a function's logic depends on is a parameter, not a context value; values hide data flow from the signature.

- Keys are unexported named types, preventing cross-package collisions:

```go
type ctxKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, ctxKey{}, id)
}

func RequestID(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(ctxKey{}).(string)
    return id, ok
}
```

- Provide typed accessors (as above); raw `ctx.Value` calls scattered through the codebase are unauditable.
