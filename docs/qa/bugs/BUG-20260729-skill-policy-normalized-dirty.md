# BUG-20260729-skill-policy-normalized-dirty: Saved skill policy remained marked unsaved

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Vera
- **Journey Step:** J-extension-policy-admin, save skill registry policy
- **Scenarios:** ET-013
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated browser replay

## Summary

The daemon persisted a multi-field Skills policy edit and correctly required a restart, but the
settings page continued to show `Unsaved changes`. A user could repeatedly submit the same policy
without knowing that the first save had succeeded.

## Reproduction

1. Open `/settings/skills` in global scope.
2. Change the poll interval from `3s` to `11m`, configure the marketplace endpoint, and add MCP and
   hook allowlist entries before saving.
3. Save and observe the returned policy plus the save bar.

**Expected:** The page adopts the daemon's canonical duration (`11m0s`), clears its dirty state, and
keeps the truthful restart notice.
**Actual before the fix:** The daemon returned `11m0s`, while the draft retained `11m` and the save
bar continued to report `Unsaved changes`.

## Evidence

- Browser, HTTP, UDS, CLI, and cleanup assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/015-skill-policy-update-remove`.
- The canonical hook regression failed with draft `10m` versus server `10m0s`, including the
  production case where `disabled_skills` is absent, then passed after the correction.

## Fix

- **Root cause:** The Skills draft machine did not advance its baseline after a successful save.
  Policy serialization also materialized an absent `disabled_skills` field as `[]`, so the draft
  looked structurally dirty even after the canonical server refetch completed.
- **Correction:** Successful saves now record the exact submitted config as their confirmed
  baseline. Policy saves preserve the server's optional tombstone representation, allowing the
  subsequent refetch to replace the draft with the daemon's canonical values without erasing
  unsaved changes from the other save channel.
- **Fix commit:** pending completion gate
- **Regression test:** `Should adopt the normalized policy after save without leaving a dirty draft`
  in `web/src/systems/settings/hooks/__tests__/use-settings-skills-page.test.tsx`.

## Verification

- The live repaired SPA submitted `16m`, adopted `16m0s`, cleared `Unsaved changes`, and retained
  `Restart needed` in the same mounted settings surface.
- The focused canonical suite passes with 9/9 tests.
- **Retested:** pending fix commit
