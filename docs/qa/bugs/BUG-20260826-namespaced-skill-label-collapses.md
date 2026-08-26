# BUG-20260826-namespaced-skill-label-collapses: Namespaced skill commands can show the same label

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-use-session-slash-commands, choose a skill command
- **Scenarios:** ET-session-command-catalog-parity, ET-session-composer-skill-chip
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

When two physical skills shared the same authored name, the daemon correctly assigned distinct
canonical tokens such as `/commit-hygiene` and `/agents:commit-hygiene`. The Web command picker
rendered both rows from the shared display name, so the two choices appeared identical.

## Reproduction

- **Charter:** CH-skill-session-suppression-matrix · **Tour:** Consistency Tour
- **Environment:** isolated production Playwright runtime / Chromium / en-US

1. Start a session whose effective catalog contains two `commit-hygiene` skills from different
   roots.
2. Open the slash-command picker.
3. Compare the two skill rows.

**Expected:** The bare winner is labeled `commit-hygiene`; the collision is labeled
`agents:commit-hygiene`, matching its canonical token.
**Actual:** Both rows were labeled `commit-hygiene`, despite having different canonical tokens.

## Evidence

- `docs/qa/evidence/2026-08-25-skill-sources/e2e-010-picker-origin-label.png`
- The focused E2E asserts exactly one bare label and one visible `agents:commit-hygiene` label.

## Fix

- **Root cause:** The Web projection used `display_name` for every command lane. Skill display names
  preserve the authored name and therefore cannot distinguish deterministic collision namespaces.
- **Fix:** Skill rows derive their label from `canonical_token`; built-in and agent commands keep
  their humanized display names.
- **Regression tests:**
  `web/src/systems/session/hooks/__tests__/use-session-commands.test.ts` and E2E-010 in
  `web/e2e/__tests__/skill-sources.spec.ts`.

## Verification

- **Focused unit:** 4/4 command-catalog projection tests passed on 2026-08-26.
- **Browser retest:** E2E-010 passed against the rebuilt production bundle on 2026-08-26; the
  captured picker shows distinct `agents:commit-hygiene` and `commit-hygiene` rows.
