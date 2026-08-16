# QA Run Report — 2026-08-16 — issue-413-bounded-liveness

- **Scope:** Issue #413 bounded Desktop liveness endpoint and full-status canary
- **Cadence tier:** targeted
- **Build:** 17f70f99 + working tree · **Environment:** isolated bootstrap lab, HTTP and UDS
- **Started:** 2026-08-16T02:55:54-03:00 · **Finished:** 2026-08-16T03:04:27-03:00 · **Status:** passed
- **Behavioral verdict:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-bounded-runtime-identity |

## Flows in Scope

- `J-operate-daemon-schema` — start and inspect the daemon through structured surfaces (`../journeys/J-operate-daemon-schema.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-bounded-runtime-identity | J-operate-daemon-schema / RT-bounded-runtime-identity | Ada | Feature Tour | Pass | | |
| 2 | CH-bounded-runtime-identity | J-operate-daemon-schema / RT-001 | Ada | Feature Tour | Pass | | |

## Session Debriefs

### CH-bounded-runtime-identity — Ada

- Entered through the documented HTTP endpoint, then independently repeated the read over UDS and the CLI.
- 500 HTTP and 500 UDS identity reads all returned status 200 and the same 214-byte process identity. Worst observed times were 2.783 ms over HTTP and 0.844 ms over UDS.
- Ten complete status snapshots remained readable during the identity burst. The worst observed status time was 11.313 ms, and every snapshot retained the running daemon PID plus the ordered global and memory schema streams.
- HTTP, UDS, and CLI complete-status reads agreed on PID 76366, listener port 53846, and both schema streams.
- An unknown identity subpath returned the router's standard 404; the next documented identity read returned the unchanged payload in 0.598 ms.
- Automated evidence passed for the Go session/observer/API suites under the race detector and for the Rust probe's actual TCP request line.
- Goal reached: yes. True end state: confirmed. Scenarios settled: `RT-bounded-runtime-identity=pass`, `RT-001=pass`.

Evidence root: `/Users/pedronauck/dev/qa-labs/compozy-issue-413-bounded-liveness-20260816-055839-132323-lab/qa-artifacts/qa/logs/`.
Teardown evidence: `/Users/pedronauck/dev/qa-labs/compozy-issue-413-bounded-liveness-20260816-055839-132323-lab/qa-artifacts/qa/teardown.json` (`clean: true`, no survivors).

## What Was Fixed

The Desktop health probe now reads a bounded runtime identity instead of the complete status aggregate. Internal memory, title, and dreaming sessions no longer wake the public session catalog, and the built-in dreaming curator resolves through effective workspace configuration.

## Paper Cuts

- Dull: an unknown API subpath returns the router's existing plain-text 404 rather than a structured error. Recovery is immediate and this issue does not change the router-wide error contract.

## Runtime Errors Observed

None.

## Human Verifications Needed

None. The native WebView was not launched in this targeted structured-surface run; its request path is covered by the Rust TCP transport regression.

## Decisions for a Human

None.

## Learnings

- The bounded payload stayed 85 times smaller than the complete fresh-daemon snapshot in this run (214 bytes versus roughly 18.3 KB).
- Moving liveness off the diagnostic aggregate preserves full status for operators while preventing the Desktop's two-second monitor from paying for session and subsystem aggregation.
- Production-parity qualification: the run used the source-built dirty binary in a fresh isolated home, not a packaged macOS application; provider and browser surfaces were intentionally outside this read-only journey.

## Final Status

- **Exit gate (full automated suite):** pending final `make gate-full`; the immutable run log will be written to `/Users/pedronauck/dev/qa-labs/compozy-issue-413-bounded-liveness-20260816-055839-132323-lab/qa-artifacts/qa/logs/final-make-verify.log` after this report is finalized.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked; 2/2 scenarios passed
- **Verdict:** ready for the final full automated gate.
