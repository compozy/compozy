# Resource → Cleanup Pairings

Canonical pairs to write defer-adjacent or release explicitly on every exit.
When cleanup returns an error, return it, join it with the primary error, or
report it through the resource owner's structured logger when the API is
explicitly best-effort. Never use `_` or a bare cleanup call that silently
discards a result.

| Allocation | Cleanup |
|------------|---------|
| `ctx, cancel := context.WithCancel(parent)` | `defer cancel()` next line |
| `ctx, cancel := context.WithTimeout(parent, d)` | `defer cancel()` next line |
| `ctx, cancel := context.WithDeadline(parent, t)` | `defer cancel()` next line |
| `detached := context.WithoutCancel(ctx)` | re-attach deadline if needed; pair with explicit `CancelPrompt`/`Stop` API |
| `f, err := os.Open(...)` | deferred `Close`; propagate or join the close error |
| `tx, err := db.Begin(...)` | deferred rollback that explicitly accepts `sql.ErrTxDone` and joins any other error; explicit `Commit` on success |
| `lis, err := net.Listen(...)` | deferred `Close`; propagate, join, or report the close error according to owner semantics |
| `resp, err := client.Do(req)` | deferred drain then close; account for both `io.Copy` and `Close` errors |
| `mu.Lock()` | `defer mu.Unlock()` next line |
| `wg.Add(1)` then `go func() { ... }()` | `defer wg.Done()` inside the goroutine |
| `proc, err := acp.Start(...)` | bounded cancel-then-grace stop; join or report shutdown/kill errors |
| `claim, err := task.ClaimNextRun(...)` | release on every non-settled exit; handle the release result |
| `lease, err := lease.Acquire(...)` | deferred release; handle the release result |
| `regHandle := registry.Register(...)` | deferred unregister; handle the unregister result when fallible |
| `tmp, err := os.CreateTemp(...)` | deferred close and remove; account for both results |
| `entry := observe.StartSpan(...)` | `defer entry.End(err)` next line; pass current `err` into End |

## Cancel-then-grace stop semantics

Subprocess stop respects both context cancellation AND graceful shutdown:

Create a bounded stop context at cleanup time, request graceful shutdown, wait
on either `proc.Done()` or the stop deadline, and force the process group only
after the deadline. Preserve the graceful-shutdown error and handle the forced
kill result; join both with the primary failure when applicable.

For ACP wrappers (npm exec → node → native), kill the entire process group — not just the wrapper. See `internal/procutil/process_group_unix.go` for the helper. Windows uses forced-exit fallback (`internal/procutil/process_tree_windows.go`).

## Sequencing rules

- **Public flip after private cleanup.** When state has both a public-visible flip (e.g., registry-visible disable) and a private resource (in-memory hook unregister), the public flip happens AFTER the private cleanup, not before. See `docs/_memory/_synthesis.md` extension manager L3 lesson.
- **Boot recovery before scheduler accepts traffic.** When initializing the daemon, recovery completes before claim/wake traffic begins. (See autonomy `_techspec.md` lease invariants.)
- **Reaper releases leases before stopping a child session.** Lease release is a precondition to stop, not a side effect.

## Drain rules (HTTP)

- Always drain even on non-2xx responses; otherwise the connection is poisoned for keep-alive.
- For SSE consumption, the reader handles drain on its own; do not double-drain.
