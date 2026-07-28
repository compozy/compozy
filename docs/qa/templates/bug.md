# BUG-<YYYYMMDD>-<slug>: <one-line title, user-first>

- **Status:** open <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** <Blocks-Completion | Data-Loss | Trust-Damage | Friction | Cosmetic>
- **Severity:** <Critical | High | Medium | Low> · **Priority:** <P0 | P1 | P2 | P3>
- **Persona Affected:** <persona name>
- **Journey Step:** <J-<slug> name>, step <N>
- **Scenarios:** <scenario ids, semicolon-separated>
- **Found:** <YYYY-MM-DD> · **Report:** <path to the run report>
- **Origin:** <only for migrated bugs — path of the pre-registry artifact>

## Summary

<What the user experienced, in user language. Lead with the person, not the stack trace.>

## Reproduction

- **Charter:** CH-<slug> · **Tour:** <tour name>
- **Environment:** <device / viewport / network / locale, per the persona row>

1. <exact step from the persona's entry point>
2. <...>

**Expected:** <the observable the flow promises>
**Actual:** <what happened instead>

## Evidence

- <screenshot / log path>
- <independent read path check — what a fresh load / second surface showed>

## Fix

<!-- filled when status moves to fixed -->
- **Root cause:** <symptom vs cause, stated separately>
- **Fix commit:** <short SHA>
- **Regression test:** <test path — failed before, passes after> <!-- or: documented replay + why no automated test is meaningful -->

## Verification

<!-- filled when status moves to verified -->
- **Retested:** <YYYY-MM-DD>, same persona/journey · **Report:** <path>
- **Result:** <observable confirmed>

<!-- Append `## Re-found (<date>)` or `## Regressed (<date>)` sections instead of filing a new bug for the same symptom. -->
