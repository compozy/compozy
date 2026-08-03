# QA Run Report — 2026-08-03 — Dev WebSocket recovery

- **Scope:** Targeted regression walk for stale workspace recovery after a daemon state change, including the `make dev` WebSocket proxy boundary.
- **Cadence tier:** targeted
- **Build:** 530a78df + working tree · **Environment:** isolated local daemon at `http://127.0.0.1:64386`; Web development server proxies to that daemon
- **Started:** 2026-08-03T15:51:42Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-prune-missing-workspace |

## Flows in Scope

- `J-prune-missing-workspace` — Remove a missing local workspace without leaving a ghost selection (`../journeys/J-prune-missing-workspace.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-prune-missing-workspace | J-prune-missing-workspace / RT-missing-workspace-pruned | Bruno | Interrupt Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-prune-missing-workspace — Bruno

- **Ran:** 2026-08-03T15:54:49Z → 2026-08-03T16:00:01Z (box respected: yes)
- **Findings:** None. The live tab moved from the removed workspace to the healthy home workspace without an error state.
- **Bugs filed/updated:** None.
- **Scenarios settled:** RT-missing-workspace-pruned → pass.
- **Paper cuts:** None.
- **Surprises:** The Web client reconciled before a manual refresh; the refresh preserved the recovered selection.
- **Suggested next charter:** Multi-tab daemon-state replacement can extend this coverage if the development lifecycle changes again.
- **Edge probes:** active-folder removal; background reconciliation; fresh reload; removed-ID deep read; CLI/HTTP catalog parity; direct missing-workspace WebSocket through Vite.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/screenshots/ch-prune-recovered-after-refresh.png`; `/Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/logs/missing-workspace-websocket.log`; `/Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/logs/workspace-list-cli.json`; `/Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/logs/removed-workspace-http.log`.

## What Was Fixed

### Keep missing-workspace preflight failures inside WebSocket transport

- **Symptom:** Stale browser tabs repeatedly logged Vite WebSocket proxy `EPIPE` errors while reconnecting after the daemon no longer knew their workspace.
- **Root cause:** The daemon returned an HTTP 404 body during an accepted WebSocket upgrade attempt; the browser closed the socket before Vite finished forwarding that body.
- **Fix:** Upgrade subscription preflight failures, emit the existing terminal error frame, and let the Web client reconcile its canonical workspace list.
- **Regression test:** `internal/api/core/window_manager_ws_test.go`; `web/src/systems/os/hooks/__tests__/use-window-manager-stream.test.tsx` — both failed before and pass after the fix.
- **Retested:** Bruno completed `J-prune-missing-workspace`; the removed ID returned HTTP 404, CLI listed only the healthy workspace, and the proxy delivered `window_manager_workspace_not_found` followed by WebSocket close code 1008.

## Paper Cuts

None.

## Runtime Errors Observed

No browser errors, Vite module-loader warning, `ws proxy error`, `ws proxy socket error`, or `EPIPE` appeared in the isolated `make dev` log.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A stale workspace is a normal terminal stream condition; keeping it inside the WebSocket protocol lets the client recover its authoritative workspace list without making the development proxy write an HTTP body to a closed socket.
- Lens pass: usability, perceived performance, error recovery, and local production parity passed on the changed recovery path; accessibility and cross-browser layout were unchanged by this transport-only fix.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` — PASS; evidence: `/Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/logs/final-make-verify.log`.
- **Strict evidence audit:** PASS; evidence: `/Users/pedronauck/dev/qa-labs/compozy-dev-websocket-recovery-20260803-155044-571985-lab/qa-artifacts/qa/qa-audit-report.json`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked; healthy-workspace Home is the adjacent canary.
- **Verdict:** PASS — the missing-workspace recovery is ready with no open findings from this targeted run.
