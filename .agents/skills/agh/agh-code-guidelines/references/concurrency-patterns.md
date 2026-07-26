# AGH Concurrency Patterns — Canonical Rules

Verbatim canonical rules. Reviewers will quote these. Companion skills cover deeper analysis: `systematic-debugging` for race/deadlock investigation, `agh-cleanup-failure-paths` for error-path cancellation discipline.

## Goroutine Ownership

- Every goroutine has explicit ownership and shutdown via `context.Context` cancellation.
- No fire-and-forget goroutines. Track with `sync.WaitGroup` or equivalent owner-side primitive and join on shutdown.
- Long-running loops use `select { case <-ctx.Done(): return; case ... }`. Never busy-wait.
- Goroutines spawned by `internal/session/manager_*.go` MUST be tracked by a Manager-owned WaitGroup and joined in Manager shutdown.
- Never put goroutine-owned channels in a struct field that another goroutine mutates — use a per-run handle.

## Synchronization

- Prefer channels over shared memory with mutexes when practical.
- `sync.RWMutex` for read-heavy shared state, `sync.Mutex` for write-heavy.
- No `time.Sleep()` in orchestration. Use timers, tickers, or context deadlines.

## Detached Execution

- Any work that outlives an HTTP/UDS request — prompts, network channel sends, automation jobs — MUST detach via `context.WithoutCancel(ctx)`.
- Never tie execution lifetime to request lifetime.
- Expose explicit cancel endpoints (e.g., `POST /api/sessions/:id/prompt/cancel`).
- `context.WithoutCancel` does NOT preserve deadlines. Re-attach with `WithDeadline` if needed.
- The writer loop stays bound to the request context — detach the *execution*, not the *response*.

## Subprocess Supervision

- Subprocess managed-stop respects `ctx.Done()` between Shutdown and Wait. Wrap `proc.Wait()` in `select { case <-proc.Done(): case <-ctx.Done(): }`.
- Process-group supervision parity: Unix uses process groups, Windows uses forced-exit fallback. Always cross-build with `GOOS=windows GOARCH=amd64 go build` before claiming subprocess work complete.
- Centralize signaling helpers in `internal/procutil`. Do not reinvent process-group signaling per package.

## Race / cgo

- `make verify` runs `-race`. Race-enabled tests need `CGO_ENABLED=1`.
- `runRaceEnabledGoCommand` (or equivalent) clones caller env and forces `CGO_ENABLED=1` for race subprocesses. Do not trust ambient env.
- Before claiming `make verify` complete on race-sensitive packages (`internal/session`, `internal/acp`, `internal/hooks`, `internal/subprocess`, `internal/resources`), reproduce locally with `act workflow_dispatch -W .github/workflows/ci.yml -j verify --container-architecture linux/amd64`.

## Authoritative Primitives (do not replicate)

- `task.Service.ClaimNextRun` is the canonical claim primitive — no peer package may replicate it.
- Wake / observe / sweep are allowed; claim / own is not.
- The mechanical scheduler does NOT call `ClaimNextRun` directly in MVP.
- Hooks dispatch at the call site that owns the state transition. Never tail event/log tables to fire hooks.

## Common Failure Modes

- Goroutine leak on error path: every error return that ran above a `go func()` spawn must signal that goroutine to exit (via `cancel()` or close of an owner-controlled channel).
- Deadlock on shutdown: a goroutine reading from a channel that the owner stopped writing to without closing — close channels you own when shutting down.
- Race on map / slice mutation: take the appropriate mutex, or use `sync.Map` for genuinely concurrent maps. Concurrent slice append without sync is always a bug.
- Lost cancellation: storing `context.Background()` instead of caller-supplied `ctx` breaks deadline propagation. Always thread `ctx` through.
