# QA Run Report — 2026-07-08 — session observability

- **Scope:** Task 40 session pipeline observability for the web return flow and stream telemetry.
- **Cadence tier:** targeted
- **Build:** local worktree `4ea584c41` + task_40 diff · **Environment:** isolated QA bootstrap lab, daemon-served local `web/dist` via `AGH_WEB_DIST_DIR`, Desktop Chrome headless, acpmock provider
- **Started:** 2026-07-08T16:58:39Z · **Closed:** 2026-07-08T17:05:26Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Operator | Power User | Desktop Chrome / local network / default locale | targeted return-flow probe |

## Flows in Scope

- Task 40 return-flow probe — open a session, route away from the thread, return to the session, and confirm web debug counters/events are emitted for the stream lifecycle.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | targeted probe | Task 40 / return-flow stream telemetry | Operator | Network / Interrupt | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Evidence

- Bootstrap manifest: `/Users/pedronauck/dev/qa-labs/agh-session-observability-20260708-164634-393992-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Return-flow debug JSON: `/Users/pedronauck/dev/qa-labs/agh-session-observability-20260708-164634-393992-lab/qa-artifacts/qa/test-cases/session-observability/return-flow-debug-events.json`
- Return-flow screenshot: `/Users/pedronauck/dev/qa-labs/agh-session-observability-20260708-164634-393992-lab/qa-artifacts/qa/test-cases/session-observability/return-flow.png`
- QA runner command: `rtk env QA_OUTPUT_PATH=/Users/pedronauck/dev/qa-labs/agh-session-observability-20260708-164634-393992-lab/qa-artifacts AGH_WEB_DIST_DIR=/Users/pedronauck/Dev/compozy/agh/web/dist bun web/.tmp/session-observability-qa.ts`

Observed debug counters:

| Checkpoint | `sse_open` | `sse_close` |
|---|---:|---:|
| After session open | 1 | 0 |
| After routing away | 1 | 1 |
| After return | 2 | 1 |

Observed debug events:

| Event | Cursor | Notes |
|---|---:|---|
| `sse_open` | 0 | Initial session-thread mount for `sess-fb0b9b0cec286b1d` in workspace `ws_4f99ecc65ae3b2ab`. |
| `sse_close` | 2 | Route-away cleanup recorded with `reason: cleanup`. |
| `sse_open` | 0 | Return to the same session remounted the live-tail stream. |

## What Was Fixed

No QA-time product fixes were made.

## Paper Cuts

None recorded in this targeted telemetry probe.

## Runtime Errors Observed

- The first local QA attempt used the daemon's embedded web-assets package, so the new web debug globals were absent. The run was repeated with `AGH_WEB_DIST_DIR=/Users/pedronauck/Dev/compozy/agh/web/dist`, which served the current task_40 bundle and produced the expected counters. This was a harness parity correction, not a product failure.
- The bootstrap manifest reported `BROWSER_BLOCKER=browser-use skill not found in CODEX_HOME plugin cache`; the probe used the Playwright/agent-browser-compatible harness instead.

## Human Verifications Needed

None for this targeted telemetry probe.

## Decisions for a Human

None.

## Learnings

- A freshly created but idle session is `state: active`, `badge: idle`, with no activity/health payload. That state correctly emits stream lifecycle telemetry, but it does not satisfy the `thread_empty_while_active` predicate; that predicate is covered by the focused component unit test where `isSessionRunning=true`.
- Transcript fetch failure, reconnect, and gap-recovery debug events are exercised by the focused live-tail unit suite. The manual probe intentionally covered the open → background/route-away → return stream lifecycle.

## Final Status

- **Exit gate (full automated suite):** `rtk make verify` exited 0 on 2026-07-08T17:05:26Z; no residual `make verify` / `gotestsum` / `go test` processes remained.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** targeted return-flow probe walked; no user-visible behavior changed, so `docs/qa/state.csv` was not reset.
- **Verdict:** ready — targeted return-flow telemetry emitted cursor-bearing stream lifecycle counters, and the full completion gate passed.
