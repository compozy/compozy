# QA Run Report — 2026-08-05 — issue-313-loop-manifest

- **Scope:** Issue #313 — Loop executed-definition template manifest round-trip from preview to persisted Run
- **Cadence tier:** targeted
- **Build:** `4cf544a4` + branch working tree · **Environment:** isolated local daemon at `http://127.0.0.1:52702`, CLI over `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-bd990b9f3485/runtime/compozyd.sock`
- **Started:** 2026-08-05T19:31:59Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-template-snapshot-round-trip |

## Flows in Scope

- `J-01` — preview and start a Loop, then confirm the persisted Run (`../journeys/J-01-arrive-and-use-run.md`)
- Adjacent canary: `J-02` — dry-run returns a plan with no persisted Run (`../journeys/J-02-dry-run-preview.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-template-snapshot-round-trip | J-01 / LP-loop-template-snapshot-round-trip | Ada | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-template-snapshot-round-trip — Ada

- **Ran:** 2026-08-05T19:36:22Z → 2026-08-05T19:39:35Z (box respected: yes)
- **Findings:** No issue #313 regression. Both Loop sources were visible, two dry-runs were deterministic and state-free, and the real submission persisted a Run whose executed definition loaded over CLI/UDS and HTTP.
- **Bugs filed/updated:** None.
- **Scenarios settled:** LP-loop-template-snapshot-round-trip → pass.
- **Paper cuts:** None.
- **Surprises:** The unconfigured provider made the draft node quarantine after submission. This happened after the Run and executed definition were persisted, so it did not affect the charter's true end state.
- **Suggested next charter:** Exercise the same definition with a live provider only when child execution, rather than the submission boundary, changes.

## What Was Fixed

The issue #313 production fix and automated regression proofs predate this QA walk; no new finding has been fixed inside the session.

## Paper Cuts

None.

## Runtime Errors Observed

The draft agent reported the expected provider-bound action failure in this no-provider targeted lab. The persisted Run remained readable and its executed definition rehydrated through both public read paths.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Journey and functional coverage use CLI, HTTP, UDS, and runtime persistence. Visual, mobile, and browser compatibility are out of scope because no rendered surface changed.
- The adjacent canary is the no-state dry-run behavior from J-02.
- Edge probes covered repeated dry-run and a post-preview real submission. Structured output remained clear and deterministic; accessibility, browser compatibility, and viewport lenses do not apply to this non-visual change.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` evidence is recorded at `/Users/pedronauck/dev/qa-labs/compozy-issue-313-loop-manifest-20260805-193142-599093-lab/qa-artifacts/qa/issue-313/final-make-verify.log`; `make gate-status` confirms the record matches the final tree.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1 changed journey walked; J-02 no-state behavior exercised as the adjacent canary; no skips inside the structured surfaces in scope.
- **Verdict:** PASS — the public Loop submission boundary and persisted readback passed, and `teardown.json` records `"clean": true` with no surviving processes.
