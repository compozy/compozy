# BUG-20260910-disabled-session-ledger: Disabled memory appears as a broken session ledger

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-11 Inspect a stopped session
- **Scenarios:** RT-024
- **Found:** 2026-09-10 · **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

- **Charter:** CH-014 · **Tour:** Interrupt Tour
- **Environment:** Isolated daemon, memory disabled, native Codex gpt-5.6-luna xhigh, desktop Web, HTTP and UDS.

1. Complete a managed session and stop it through the public CLI.
2. Open its Web inspector and select Memory.
3. Read the same session ledger through HTTP and UDS.

**Expected:** The configured absence of session memory is an unavailable capability, using the existing `memory.unsupported` response, and the inspector explains that state.
**Actual:** The Web shows "Unable to load session ledger". HTTP and UDS return 500 `memory.internal`; the UDS message says the ledger root is not configured.

## Evidence

All paths below are under `docs/qa/evidence/2026-09-10-qa-execution-unblock/`:

- `ledger-disabled-http.json`, `ledger-disabled-uds.json`: public response parity.
- `rt024-ledger-failure.png`, `rt024-ledger-failure.txt`: stopped-session inspector failure.
- `canary-settled-http.json`, `canary-stop.json`: real provider and terminal lifecycle.

## Diagnosis and repair

The disabled constructor returns a typed nil pointer, which becomes a non-nil service interface in runtime dependencies. The existing unsupported-service guard is bypassed. The Web also needs to recognize the existing unsupported response distinctly from an unmaterialized ledger. Repair and original-persona verification are pending.

## Cross-surface impact

Audit follows `docs/_memory/change-impact.md`. Session memory ledger/replay/prune/repair HTTP, UDS and native-tool operations share this optional dependency; their existing unsupported response is preserved. No routes, IDs, DTOs, hooks, configuration keys, workspace authorization or persisted data shapes change. The Web inspector gains a truthful unavailable state. The official Compozy skill continues to use the same operations and requires no command migration.

## Local verification checkpoint

The Go factory now returns the service interface so disabled and unconfigured cases are absent. The Web recognizes only the existing501 `memory.unsupported` code as unavailable. Race-enabled daemon coverage and92 root-Turbo adapter/inspector tests pass; the red evidence proves both original failures. Go/Web builds and Web typecheck pass; React Doctor100/100. `make gate` is running; no fix commit has been created yet.

Original-persona re-walk on fresh real Luna xhigh session `sess-f4ff28e4f5f576a6` confirms stopped Memory now shows unavailable and HTTP/UDS both return501. The transcript task was canceled after invalid agent-authored terminal arguments; the memory-state observation is independent of its file-reading outcome. See `ledger-retake-stopped-web.png`, `ledger-retake-stopped-http.json`, `ledger-retake-stopped-uds.json` in the same evidence directory.

## Verified local fix

Commit `4f5ae5298` contains the production correction and canonical regressions. `make gate` passed all affected lanes (`profile-context-make-gate-retry.txt`); the live RT-024 Web/HTTP/UDS retest and restart parity are recorded in `profile-patch-parity-summary.json` and `profile-patch-memory-web.png`. No push or CI claim.
