# QA Run Report — <YYYY-MM-DD> — <scope>

- **Scope:** <branch/PR diff summary or cycle name — what user-visible change this run covers>
- **Cadence tier:** <smoke | targeted | full | sanity>
- **Build:** <commit SHA / deploy id> · **Environment:** <URL, parity notes>
- **Started:** <ISO timestamp> · **Status:** in-progress <!-- in-progress | closed -->

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| <name> | <seed base> | <profile> | CH-<slug>, ... |

## Flows in Scope

<!-- One entry per journey; link the journey file, embed or reference its Mermaid flow. -->

- `J-<slug>` — <one-line value statement> (`../journeys/J-<slug>.md`)

## Session Matrix & Results

<!-- Created with every row Pending BEFORE the first session. Updated after each session and each fix. -->

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-<slug> | J-<slug> / <id> | <name> | <tour> | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

<!-- One block per charter run, written within 5 minutes of the box ending. The charter file stays untouched. -->

### CH-<slug> — <persona>

- **Ran:** <started> → <ended> (box respected: yes/no)
- **Findings:**
  - <finding, with impact-tier rationale>
- **Bugs filed/updated:** [BUG-<YYYYMMDD>-<slug>, ...]
- **Scenarios settled:** <id → verdict, ...>
- **Paper cuts:** <persona-felt friction, sharpness noted>
- **Surprises:** <unexpected observations>
- **Suggested next charter:** <what this session did not reach>

## What Was Fixed

<!-- One entry per governed auto-fix. -->

### BUG-<YYYYMMDD>-<slug>: <title>
- **Symptom:** <user-side observable>
- **Root cause:** <stated separately from the symptom>
- **Fix:** <commit SHA, one logical fix>
- **Regression test:** <path — failed before, passes after> <!-- or: documented replay + reason + automation-backlog entry -->
- **Retested:** <impacted journey + adjacent journeys, fresh sessions>

## Paper Cuts

<!-- Persona-felt friction that no functional check failed. Sharp ones entered the fix loop. -->

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| <name> | J-<slug> step <N> | "<persona language>" | sharp/dull | fixed (SHA) / deferred / watching |

## Runtime Errors Observed

<!-- Console/log errors surfaced during walks, even when the flow visually passed. -->

- <error> — <where, evidence path, filed as a registry bug or explained>

## Human Verifications Needed

<!-- Legs only a human can complete. Exact instructions; terminal for this run. -->

- [ ] <what to do, from which entry point, what observable confirms it> (row #N)

## Decisions for a Human

<!-- Escalations from the fix-loop governor: what's broken, options with trade-offs, recommendation. -->

### <finding title> (BUG-<YYYYMMDD>-<slug>)
- What's broken: <user-side, evidence path>
- Why not auto-fixed: <governor bound it fails>
- Options: 1. <option — trade-off> 2. <option — trade-off>
- Recommendation: <option + reason>

## Learnings

<!-- Patterns worth carrying forward: candidate tours, persona insights, planning gaps found. -->

- <learning>

## Final Status

<!-- Written LAST, after the exit gate. -->

- **Exit gate (full automated suite):** <command + result, verbatim>
- **Issues by user impact:** Blocks-Completion <N> · Data-Loss <N> · Trust-Damage <N> · Friction <N> · Cosmetic <N>
- **Coverage:** <journeys walked / in scope; skips disclosed>
- **Verdict:** <ready | not ready | ready with blocked items> — <one actionable sentence>
