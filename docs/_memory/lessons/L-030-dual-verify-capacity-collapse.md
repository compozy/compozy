# L-030 — Two concurrent `make verify` runs collapse the machine; serialize the gate

**Class:** Workflow / Build tooling
**Date discovered:** 2026-07-09 (operator report: two worktree agents ran `make verify` concurrently; both stalled indefinitely)
**Evidence sources:** operator incident + magefile lane audit + shared-state elimination sweep

## Context

Running agents in parallel git worktrees is the intended AGH workflow (L-009 isolated their runtime state). But whenever two agents reached their completion gates simultaneously, both `make verify` runs stalled and never finished, making real parallel development impossible.

## Root cause

There is no single blocking lock — the stall is **capacity collapse from multiplicative unbounded parallelism**. `make verify` is deliberately machine-sized: the unit lane runs `go test -race ./...` with no `-p` cap (defaults to `GOMAXPROCS` — 16 package binaries at once on the operator machine, race-instrumented, several spawning subprocess trees), `turbo run test` fans vitest thread pools (default = cores) across workspaces, plus Vite builds and typechecking. Two verifies double all of it: up to ~32 concurrent race binaries plus two vitest fleets on 16 cores — worsened by orphaned QA processes (L-029). The machine enters starvation; neither run progresses observably, which reads as a mutual deadlock.

Eliminated as causes (checked 2026-07-09): daemon singleton lock uses `flock.TryLock` (errors, never blocks); golangci-lint runs with `--allow-parallel-runners`; `GOCACHE`/`GOMODCACHE`/golangci caches are concurrency-safe; no fixed ports, fixed sockets, or wait-forever loops in the unit-lane tests or `internal/testutil`.

## Rule

> The full `make verify` gate is machine-sized by design; exactly one may run per machine. Concurrent verifies queue behind a machine-wide lock with explicit progress messages — they must never silently share the machine.

## Operationalization

- `Verify()` (magefile) acquires `~/Library/Caches/agh-dev/verify.lock` (`os.UserCacheDir()`-based) before running steps; a queued run prints `make verify is queued: pid N (worktree X) holds the machine-wide verify lock` every 15s. Lock release is automatic on process exit (kernel flock), so a crashed holder never wedges the queue.
- The lock is capacity management, not correctness: every failure path (no cache dir, flock error) warns and proceeds. `AGH_VERIFY_LOCK=off` bypasses for single-checkout machines (CI runners don't contend anyway).
- Agents seeing the queue message must wait, not kill: a queued verify is working as designed. During iteration, scoped lanes (`make lint` + scoped `go test`, turbo `--filter`) remain the concurrency-friendly path — the queue only serializes the machine-sized gates (verify and the E2E lanes, which share the lock).
- Scoped lanes are bounded instead of locked: the unit lane budgets combined `go test -p` × `-parallel` concurrency against half the effective Go CPU capacity (`AGH_GO_TEST_P` overrides the package cap), and every Vitest project caps `maxWorkers` at 50%, so iteration in one worktree stays safe next to another worktree's work.
- Helpers live in `magefiles/verifylock.go`; suite: `magefiles/verifylock_test.go` (`go test -tags mage -race ./magefiles`).

## Detection signals

- Two worktrees both showing a silent, non-progressing `make verify`.
- Load average far above core count while verifies run; multiple `gotestsum`/race test binaries from different worktree paths in `ps`.

## Source

- Operator incident report, 2026-07-09 ("quando 2 agentes rodam make verify juntos, os dois ficam travados e nunca finalizam").
- `magefiles/test.go` `Test()` (unit lane, `-p` via `goUnitTestPackageLimit`) vs `TestIntegration()` (`-p 2` — the bound already existed for the integration lane).
- Elimination sweep: `internal/daemon/lock.go` (TryLock), `runGolangCILint` (`--allow-parallel-runners`), unit-lane port/socket grep (clean).
- Fix: `magefile_verifylock.go` + `Verify()` wiring (this change).
- Related: [L-009](L-009-concurrent-worktree-deadlock.md) (runtime-state isolation), [L-029](L-029-qa-labs-must-tear-down-processes.md) (QA process load amplifying the collapse).
