# BUG-20260825-expose-preflight-misattributes-target: Expose preflight blames every target for one conflict

- **Status:** verified <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-share-skills-with-other-tools, expose one skill to several provider folders
- **Scenarios:** ET-skill-exposure-lifecycle; ET-web-skill-expose-panel
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

When Dora exposes one skill to `agents` and `claude` and the Claude path is occupied, the response
reports `expose_name_conflict` for both targets. Both result rows name the Claude path, even though
the agents target passed preflight and was never written. The envelope therefore invents a conflict
on a healthy target and gives the UI no truthful state to render.

## Reproduction

- **Charter:** CH-skill-expose-lifecycle-trust · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US · isolated lab daemon `127.0.0.1:55384`, build `c1c9468`

1. Create a workspace skill and enable the `agents` and `claude` preset sources.
2. Put a foreign directory at the skill's Claude exposure path.
3. Run `compozy skill expose <name> --to agents,claude --workspace <workspace> -o json`.

**Expected:** Claude carries `expose_name_conflict`; agents is explicitly not applied; no link or
ownership record is written and `rolled_back` is false.
**Actual:** both targets carry `expose_name_conflict`, both name the Claude path, and the summary says
`2 of 2 targets failed`.

## Evidence

- `<lab>/qa-artifacts/qa/expose-multi-target-conflict.json` — the misattributed failure envelope.
- Filesystem inspection confirmed no agents link and no partial exposure record were created.

## Fix

- **Root cause:** `preflightExpose` correctly attached the conflict to Claude, then
  `failUnresolvedExposureTargets` copied the same error object to every unresolved target. That
  duplicated the failing target, path, and code and emitted false failure events for untouched
  targets.
- **Fix commit:** `8dc4eb1` (`fix: report untouched exposure targets accurately`).
- **Regression tests:** the canonical expose suite proves target-local attribution and no mutation;
  the API suite proves the `1 of 2 targets failed` envelope; the CLI and web suites distinguish
  `expose_not_applied` from `rolled_back`.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** the rebuilt daemon returned `1 of 2 targets failed`; agents carried
  `expose_not_applied` with no foreign path, Claude carried `expose_name_conflict`, and
  `rolled_back` remained false. The agents path stayed absent, the foreign Claude directory stayed
  intact, and a clean expose/unexpose retry converged with no residue. Evidence:
  `<lab>/qa-artifacts/qa/expose-multi-target-conflict-fixed.json`,
  `<lab>/qa-artifacts/qa/expose-after-preflight-fix.json`, and
  `<lab>/qa-artifacts/qa/unexpose-after-preflight-fix.json`.
