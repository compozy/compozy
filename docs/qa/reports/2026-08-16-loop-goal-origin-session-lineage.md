# QA Run Report — 2026-08-16 — loop-goal-origin-session-lineage

- **Scope:** issue #416 branch — preserve informational origin lineage for Loop-owned Goal sessions and render that lineage as a neutral session thread without changing safe-spawn governance
- **Cadence tier:** targeted
- **Build:** `v0.3.0-beta.16-17-gf610cc8f-dirty`, binary SHA-256 `5d689d7bc312a8192d2455e58d9197fcc1348dbcdb01e6f7a52e62a38d7262f3` · **Environment:** fresh isolated lab `compozy-loop-goal-origin-session-lineage-rerun-20260816-163846-085170-lab`; browser blocked by missing driver
- **Started:** 2026-08-16T16:20:52-03:00 · **Status:** blocked-verify (structured journey passed; visual journey needs a human browser walk)

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-goal-origin-lineage |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-session-sidebar-navigation |

## Flows in Scope

- `J-15` — drive and read the Goal session deterministically through structured session surfaces (`../journeys/J-15-operate-session-via-cli-api.md`)
- `J-14` — inspect the same persisted lineage through the session sidebar and survive the missing-parent branch (`../journeys/J-14-read-a-finished-transcript.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-goal-origin-lineage | J-15 / RT-loop-goal-origin-session-lineage | Ada | Feature Tour | Pass | compozy/compozy#416 | pending |
| 2 | CH-session-sidebar-navigation | J-14 / ET-web-session-sidebar-threads | Bruno | Feature Tour | Blocked (needs human verify) | compozy/compozy#416 | pending |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-goal-origin-lineage — Ada (first walk)

- **Ran:** 2026-08-16T16:23:50-03:00 → 2026-08-16T16:29:17-03:00 (box respected: yes)
- **Findings:**
  - The provider-backed origin dispatched the Loop and the system Goal received the exact informational parent/root lineage. After origin deletion the Goal remained active, but its first public agent spawn returned 404 because child creation still required the deleted visual root. This is an in-scope Blocks-Completion finding for Goal orchestration after origin cleanup.
- **Bugs filed/updated:** compozy/compozy#416 (existing issue scope; local regression proof pending)
- **Scenarios settled:** RT-loop-goal-origin-session-lineage remains pending until the governed fix and fresh re-walk
- **Paper cuts:** None.
- **Surprises:** The initial fixture used `completed` instead of the Goal report contract's `complete`; fixture version 2 corrected that authoring error before the product finding was reproduced.
- **Suggested next charter:** Re-run this charter from a fresh lab after the missing informational-root creation path is fixed.

### CH-loop-goal-origin-lineage — Ada (fresh re-walk)

- **Ran:** 2026-08-16T16:38:46-03:00 → 2026-08-16T17:18:17-03:00 (box respected: yes)
- **Findings:** None in the product fix. The authored QA Loop later failed because the provider omitted
  required `summary` from its structured result; the exact workspace file had already been written
  and every lineage/spawn acceptance assertion had passed.
- **Bugs filed/updated:** compozy/compozy#416 (existing issue; structured fix verified locally)
- **Scenarios settled:** RT-loop-goal-origin-session-lineage passed; ET-web-session-sidebar-threads
  remains blocked for a human visual walk because bootstrap found no browser driver.
- **Evidence:** `docs/qa/evidence/2026-08-16-loop-goal-origin-session-lineage/structured-walk.md`
- **Teardown:** `docs/qa/evidence/2026-08-16-loop-goal-origin-session-lineage/teardown.json` (`clean: true`, no survivors)

## What Was Fixed

- The new child lineage now checks whether its inherited visual root still exists. When that
  informational root was removed, only the new child rebases to the live governance root; the
  existing Goal keeps its historical origin unchanged.
- The Web hierarchy fixture was corrected to use the generated `type` field, and the full TypeScript
  typecheck now covers it.

## Paper Cuts

- First walk: public `POST /api/agent/spawn` from the live Goal returned 404 after origin deletion:
  `validate root lineage ... session not found`. The fresh re-walk passed through the same public
  creation seam and produced spawned child `sess-665d776c2a781ec9`.

## Runtime Errors Observed

- Browser automation is unavailable in the bootstrap environment (`Neither browser-use nor agent-browser CLI is available`). The Web row remains pending for operator visual verification on the final build.

## Human Verifications Needed

- Open the final candidate in the Web shell and confirm the loaded origin → Goal → spawned-child
  connector/count hierarchy, then remove or omit the origin and confirm the Goal remains a root.

## Decisions for a Human

None recorded yet.

## Learnings

- The runtime provenance and Web hierarchy are separate promises: structured lineage does not imply safe-spawn governance, and visual nesting does not prove it.
- A dangling informational root is safe for the existing Goal but must also be handled when that live non-spawned Goal becomes a governed spawn boundary.

## Final Status

- **Exit gate (full automated suite):** `make verify` passed — 22,571 Go tests under race (3 skips),
  5,088 Web tests, typecheck, lint, builds, and boundaries.
- **Issues by user impact:** Blocks-Completion 1 found and fixed locally · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/2 journeys walked; the structured journey passed, and the Web journey is explicitly blocked on human visual verification.
- **Verdict:** blocked-verify — runtime/API/CLI behavior passes on the candidate, but visual acceptance remains unclaimed until the operator walks the Web hierarchy.
