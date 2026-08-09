# QA Run Report — 2026-08-09 — Release Runtime Startup

- **Scope:** Targeted replay of MCP relay startup and bridge enable-to-ready behavior after release-candidate failures.
- **Cadence tier:** targeted
- **Build:** `4aeb4cd3` plus the current release-runtime fix tree · **Environment:** fresh isolated local lab; current-source binary; public CLI/API and official MCP SDK/reference bridge adapter
- **Started:** 2026-08-09T16:27:52-03:00 · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | J-operate-compozy-from-mcp-client | desktop / wifi-fast / en-US | CH-mcp-client-operates-compozy (targeted seam) |
| Omar | J-diagnose-repair-bridge | desktop / wifi-fast / en-US | NB-029 targeted lifecycle replay |

## Flows in Scope

- `J-operate-compozy-from-mcp-client` — an external MCP client operates exactly one Compozy workspace (`../journeys/J-operate-compozy-from-mcp-client.md`).
- `J-diagnose-repair-bridge` — an operator enables a bridge and reads its runtime health (`../journeys/J-diagnose-repair-bridge.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-mcp-client-operates-compozy | J-operate-compozy-from-mcp-client / ET-session-input-mcp-projection | Ada | Feature Tour | Pass | | current tree |
| 2 | Targeted bridge lifecycle replay | J-diagnose-repair-bridge / NB-029 | Omar | Feature Tour | Pass | | current tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-mcp-client-operates-compozy — Ada

- **Ran:** 2026-08-09T16:33:33-03:00 → 2026-08-09T16:34:10-03:00
- **Findings:** The official Go MCP SDK negotiated protocol `2026-07-28`, discovered all four session-input tools, created session `sess-535e26a44ff986b5`, and received an empty input queue. The public CLI independently observed the same active session and reported 248 native tools with zero Host API projection IDs.
- **Bugs filed/updated:** none.
- **Scenarios settled:** `ET-session-input-mcp-projection` → pass.
- **Paper cuts:** none.
- **Surprises:** none.

### Targeted bridge lifecycle replay — Omar

- **Ran:** 2026-08-09T16:25:00-03:00 → 2026-08-09T16:26:10-03:00
- **Findings:** Five consecutive public HTTP/reference-adapter runs completed explicit enable through `starting` to `ready`; the nightly daemon lane then passed all 137 E2E tests.
- **Bugs filed/updated:** none; the SQLite writer race was fixed before this clean replay.
- **Scenarios settled:** `NB-029` → pass.
- **Paper cuts:** none.
- **Production parity:** the bounded replay used the shipped reference adapter instead of a live Telegram account; live delivery is outside NB-029's startup-state observable.

## What Was Fixed

### MCP relay dependency capture

- **Symptom:** `compozy mcp serve` panicked during initialize because the installed closure captured daemon dependencies before defaults were applied.
- **Root cause:** `withMCPRuntimeDefaults` ran before `withDaemonRuntimeDefaults` on a value receiver.
- **Fix:** current tree; install daemon defaults before the MCP runner closure.
- **Regression test:** `internal/cli/mcp_serve_test.go`.
- **Retested:** official Go MCP SDK plus independent public CLI read in a fresh isolated lab.

### Bridge operational-state persistence

- **Symptom:** the provider's `ready` report returned `Invalid params` and the bridge stayed `starting`.
- **Root cause:** bridge read-modify-write transactions used deferred SQLite transactions and leaked `SQLITE_BUSY_SNAPSHOT` (`database is locked (517)`) during desired-state reconciliation.
- **Fix:** current tree; run bridge update and replacement transactions through the shared `BEGIN IMMEDIATE` writer with bounded retries.
- **Regression test:** `internal/store/globaldb/global_db_bridges_test.go`.
- **Retested:** 20/20 focused bridge E2E nodes plus 137/137 nightly daemon E2E tests.

## Paper Cuts

None.

## Runtime Errors Observed

- The first nightly rerun exposed a stale HTTP gateway integration fixture that no longer implemented `GatewayService.Audit`; the fixture was updated and all eight HTTP transport integration nodes passed.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- JSON-RPC `Invalid params` masked the underlying SQLite extended error. Capturing adapter markers separated payload validation from storage contention.

## Final Status

- **Exit gate (full automated suite):** workstream-closing `make gate-full` is tracked after the last repository mutation; targeted evidence includes 137/137 daemon E2E tests and 8/8 HTTP transport integration tests.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 targeted scenarios walked
- **Verdict:** ready — both release-candidate runtime startup failures passed through their public integration surfaces.
