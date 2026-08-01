# Errors

An error is an event that is either handled or propagated with context — silent failure and duplicate logging are equally wrong.

## Creation

- Error strings are lowercase, acronyms included, with no trailing punctuation — they compose mid-sentence when wrapped: `"invalid message id"`, not `"Invalid message ID."`.
- Messages describe what happened, not what the caller should do.
- Sentinel errors include their package as a prefix so the origin survives wrapping: `errors.New("apiclient: not found")`.
- Keep messages low-cardinality: stable template text, variable data as wrapping context or structured attributes — high-cardinality message strings destroy log grouping in aggregators.

### Sentinel vs custom type

| Need | Use | Example |
| --- | --- | --- |
| Expected, matchable condition | Sentinel `var Err… = errors.New(…)` | `ErrNotFound`, `ErrTimeout` |
| Error must carry data | Custom type with `Error() string` | `*PathError{Op, Path, Err}` |
| One-off failure, no matching needed | Inline `fmt.Errorf` | `fmt.Errorf("parse header: %w", err)` |

Naming: sentinel variables take the `Err` prefix (`ErrNotFound`); error types take the `Error` suffix (`SyntaxError`).

```go
var ErrNotFound = errors.New("store: not found")

type ValidationError struct {
    Field string
    Rule  string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("field %q fails rule %q", e.Field, e.Rule)
}
```

## Wrapping and inspection

- Wrap with `%w` to preserve the chain: `fmt.Errorf("loading config: %w", err)`. `%v` flattens the chain into text — use it only at system boundaries where internal error types must not leak (API responses, user-facing messages).
- Add context at each hop that knows something the callee didn't: operation, key identifiers. Skip wrapping that adds no information (`fmt.Errorf("error: %w", err)`).
- Match sentinels with `errors.Is(err, ErrNotFound)`; extract types with `errors.As(err, &target)`. Go 1.26+: prefer `errors.AsType[*ValidationError](err)` when the target implements `error`.
- `errors.Join(errA, errB)` combines independent failures (parallel workers, multi-step cleanup); `errors.Is`/`As` traverse all branches of the joined tree.
- String matching on `err.Error()` is forbidden — it breaks on every message edit.

```go
if err := store.Get(ctx, id); err != nil {
    if errors.Is(err, ErrNotFound) {
        return nil, fmt.Errorf("session %s: %w", id, err)
    }
    return nil, fmt.Errorf("loading session %s: %w", id, err)
}
```

## The single-handling rule

An error is **either logged or returned — never both**. Log-and-return produces one failure reported N times, burying the actual frequency of problems. Propagate upward with context; the top of the call chain (HTTP middleware, worker loop, `main`) logs once with full chain and structured attributes.

```go
// Wrong — every caller up the stack logs the same failure again
if err != nil {
    slog.Error("query failed", "err", err)
    return err
}

// Right — propagate with context; the boundary logs once
if err != nil {
    return fmt.Errorf("querying user %s: %w", userID, err)
}
```

Discarding is a decision, not a default: a genuinely ignorable error gets an explicit justification comment at the discard site.

## Logging errors

- Use `log/slog` with structured attributes — never `fmt.Println`/`log.Printf` in operational paths.
- Attach identifiers as attributes, not into the message: `slog.Error("session load failed", "session_id", id, "err", err)`.
- Levels carry meaning: `Error` = failed and needs attention, `Warn` = degraded but proceeding, `Info` = state change, `Debug` = diagnosis detail.
- Never expose internal error text to end users — translate to a stable, user-safe message at the boundary and log the technical chain separately.

## Panic and recover

- Return errors for anything a caller could plausibly handle: bad input, missing file, network failure, timeout.
- `panic` marks violated invariants — states the programmer proved impossible. Message them as such: `panic("invariant: registry accessed before Init")`.
- `Must*` variants (`regexp.MustCompile`, `template.Must`) are legitimate only for package-level initialization of static values, where failure means the binary itself is wrong.
- Recover at goroutine boundaries you own (a panicking goroutine kills the whole process) and at subsystem boundaries that must survive a bad request; convert to an error, log with stack, continue:

```go
defer func() {
    if r := recover(); r != nil {
        err = fmt.Errorf("worker panic: %v\n%s", r, debug.Stack())
    }
}()
```

`recover` as routine control flow is forbidden — it hides bugs and costs a stack unwind.
