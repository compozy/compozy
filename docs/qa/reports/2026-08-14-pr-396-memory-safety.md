# QA Run Report — 2026-08-14 — PR 396 Memory Safety

- **Scope:** PR #396 scanner namespace coverage and autonomous filename-collision safety.
- **Cadence tier:** targeted
- **Build:** `90713850` dirty worktree · **Environment:** isolated local daemon at `http://127.0.0.1:63183`; real CLI/API/runtime, no provider or Web required.
- **Started:** 2026-08-14T23:00:14Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
| --- | --- | --- | --- |
| Ada | Power User | desktop / wifi-fast / en-US | CH-memory-operational-state-boundary |

## Flows in Scope

- `J-store-durable-memory-safely` — reject operational chatter without blocking a nearby durable write (`../journeys/J-store-durable-memory-safely.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | CH-memory-operational-state-boundary | J-store-durable-memory-safely / MS-reject-operational-memory-state | Ada | Garbage Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-memory-operational-state-boundary — Ada

- **Ran:** 2026-08-14T23:00:31Z → 2026-08-14T23:03:50Z (box respected: yes)
- **Findings:**
  - No product defect found. CLI rejection, structured HTTP rejection, autonomous no-op, explicit update, and fresh-read behavior matched the contract.
- **Bugs filed/updated:** none
- **Scenarios settled:** MS-reject-operational-memory-state → pass
- **Paper cuts:** none
- **Surprises:** The documented HTTP request exposes the origin field, so the public dogfood could prove all three autonomous-origin branches directly instead of relying only on unit evidence.
- **Suggested next charter:** Provider-backed extractor harvesting remains the adjacent release-grade canary in CH-dream-pipeline-canary.

## What Was Fixed

No QA-session fixes were required. The review remediation was already present in the build under test and is covered by the red-before/green-after unit evidence.

## Paper Cuts

None observed.

## Runtime Errors Observed

None observed.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Namespace-level scanner matching keeps new Memory v2 identifiers safe without copying the generated tool catalog into policy tests.
- Public HTTP origin selection provides a production-like path for collision-boundary dogfood in an isolated daemon.
- Garbage Tour probes covered qualified native, dotted event, policy, all three autonomous origins, explicit identity, unchanged-state rejection, and a safe adjacent write.

## Final Status

- **Exit gate (full automated suite):** `make gate-full`; authoritative current result is recorded by `make gate-status` at handoff and indexed in the lab verification report.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journey walked; CLI, HTTP, and runtime surfaces passed. Provider/Web were deliberately out of scope for this bounded data-integrity journey.
- **Verdict:** ready — the scenario passed; merge readiness additionally requires the current full-gate record named above.
