# QA Run Report — 2026-08-04 — Loop Lifecycle MCP Inputs

- **Scope:** Targeted public-interface replay for the session-input Host API projection repaired while closing the Loop node lifecycle branch.
- **Cadence tier:** targeted
- **Build:** final pre-commit Loop lifecycle tree · **Environment:** fresh isolated local lab; current-source binary; official Go MCP SDK over stdio
- **Started:** 2026-08-04T01:45:00-03:00 · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | J-operate-compozy-from-mcp-client | desktop / wifi-fast / en-US | CH-mcp-client-operates-compozy (targeted seam) |

## Flows in Scope

- `J-operate-compozy-from-mcp-client` — an external MCP client operates exactly one Compozy workspace (`../journeys/J-operate-compozy-from-mcp-client.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-mcp-client-operates-compozy | J-operate-compozy-from-mcp-client / ET-session-input-mcp-projection | Ada | Feature | Pass | MCP projection omitted four canonical methods | final remediation commit |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-mcp-client-operates-compozy — Ada

- **Ran:** 2026-08-04T01:47:09-03:00 → 2026-08-04T01:55:03-03:00
- **Findings:** The official Go MCP SDK discovered the list, replace, cancel, and promote tools with object schemas. It created session `sess-515f1c7d49742b73`, called the workspace-bound list tool, and received an empty input queue with a complete result. The public CLI observed the same active session in workspace `ws_359eaaecc7b4cf55` before cleanup.
- **Boundary proof:** `compozy tool list --workspace qa-mcp-inputs -o json` returned 238 native tools and zero IDs prefixed by `compozy_host__`.
- **Bugs filed/updated:** none; this replay verified the remediation found by the branch gate.
- **Scenarios settled:** `ET-session-input-mcp-projection` → pass.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-lifecycle-mcp-inputs-20260804-044709-893107-lab/qa-artifacts/qa/mcp-projection-evidence.md`

## What Was Fixed

### Session-input Host API projection coverage

- **Symptom:** `compozy mcp serve` failed closed before accepting an MCP client because four canonical Host API methods had no explicit projection decision.
- **Root cause:** PR #304 registered the methods in the Host API contract but did not extend `hostAPIProjectionDecisions`.
- **Fix:** working tree; publish the four workspace-bound session-input methods and make the real CLI regression fixture fail fast instead of deadlocking on an unread pipe.
- **Regression test:** `internal/mcp/serve_test.go` plus `internal/cli/mcp_serve_test.go` — failed before, pass after.
- **Retested:** passed through the official Go MCP SDK plus independent public CLI reads in a fresh isolated lab.

## Paper Cuts

None.

## Runtime Errors Observed

- The first client probe intentionally used a nonexistent session ID and received `Not found`; the final walk replaced it with a real MCP-created session so the evidence covers Host API dispatch and storage.
- The fresh lab initially had no default provider. `defaults.provider=codex` was set through the public config CLI and the owned daemon was restarted before the final journey.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A narrow integration seam should prove both sides of the boundary: successful Host API projection and absence from the native registry.
- Creating the session through the same external MCP client, then reading it independently through the CLI, proves workspace binding without relying on internal test helpers.

## Final Status

- **Exit gate (full automated suite):** tracked separately by the branch closeout; this report owns the targeted public-interface replay.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted scenario walked
- **Verdict:** PASS — all four session-input methods are discoverable and callable through the workspace-bound MCP relay, while the native registry boundary remains unchanged.
