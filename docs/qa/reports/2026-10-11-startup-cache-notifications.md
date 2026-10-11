# Session startup, cache reuse, and attention polling

Scope: issues #726, #727, #728, #729, and #730. Branch:
`fix/startup-cache-notifications`, based on `5a9d801d3` in a separate worktree.

## Root causes and corrections

1. `session/new` shared the ordinary 30-second ACP control budget despite loading native
   provider plugins, hooks, and MCP servers. Its default is now 90 seconds; an explicit
   timeout, including exactly 30 seconds, still wins. Caller cancellation and bounded
   retry/cleanup retain their existing behavior.
2. Detached stdio held the structured rotating log open for the process lifetime.
   Raw stdio now has its own file, retaining failure-tail reporting without blocking
   structured log renames on Windows.
3. Discovery captured directory-index metadata and compared it against live Lstat data.
   Capture now uses Lstat too. NTFS directory allocation size is excluded from snapshots;
   file content, membership, entry type, and symlink changes still invalidate discovery.
4. Attention polling computed inbox totals and facets it never consumed. The internal
   items-only query skips that statement while retaining the same page and cursor.
   Normal inbox metadata materializes its common filtered population once.
5. Every new Claude/Cursor binding forced live discovery. A recent live row now binds
   directly, bounded by the existing five-minute discovery TTL and execution context.
   Revalidation remains mandatory for unavailable, expired, stale, future-dated, absent,
   or unusable bindings. This is reuse of authoritative discovery, not curated fallback.

## Invariants and canonical suites

| Invariant | Owner | Existing suite |
| --- | --- | --- |
| Cold creation has its own bounded default, overrides/cancellation survive | ACP driver | `TestACPControlRequestDeadlines` |
| Structured logs rotate with child stdio open; raw failures remain inspectable | CLI process boundary | `TestSpawnDetachedDaemonProcess`, `TestSpawnDetachedDaemonProcessInjectsMirrorOverrideEnv` |
| Stable NTFS directories reuse discovery; actual changes invalidate | Filesystem snapshots/discovery | `TestFromInfo`, `TestDirectoryResultUnchanged` |
| Items-only reads execute one statement and preserve pagination | GlobalDB, real SQLite | `TestGlobalDBTaskInboxUsesTwoStatementPaging` |
| Recent bindings reuse only the scoped live source | Session runtime | `TestCreateAppliesRuntimeModelOverride` |

## Verification

Local checks use `GOMAXPROCS=2`, `CGO_ENABLED=1`, `go test -p 1 -race` through `mise exec --`.
No `make gate`, full suite, or local E2E gate is run, as explicitly requested.

- Passed: `./internal/filesnap ./internal/skillscan`.
- Passed: `./internal/acp -run TestACPControlRequestDeadlines -count=1`.
- Passed: `./internal/store/globaldb -run TestGlobalDBTaskInbox -count=1`;
  strengthened items-only continuation regression also passed separately.
- Passed: `./internal/cli ./internal/session ./internal/observe` with
  `-run 'TestSpawnDetachedDaemonProcess|TestCreateAppliesRuntimeModelOverride|Attention|Notification' -count=1`.
- Pending: native Windows regression step and all required checks at the final PR head.
- Test-shape heuristic: touched files contain inherited standalone-test diagnostics;
  subprocess entry points are intentional. New/changed cases use canonical subtests.
  CLI detached-process, filesnap, and skillscan files pass the heuristic.

The native Windows CI step exercises the real filesystem and a Go child process whose
structured logger rotates while its raw stdio handles remain open. Real Claude/Cursor
latencies quoted in the issue reports are reporter evidence, not new measurements.
No browser/live-provider walkthrough has been claimed.

## Cross-surface audit

See `docs/_memory/change-impact.md`, section “Session startup, Windows caching, and
attention polling — 2026-10-11”.
