# BUG-20260713-loop-preview-ignores-config: A saved Loop configuration is ignored by the detail and run preview

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-05 configure a Loop without forking, step 3
- **Scenarios:** LP-017
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Bruno saved `software-delivery` with an iteration cap of 3, `full-body` re-attempts, a no-progress window of 2, fan-out ceiling of 4, gate revisions of 2, human approval enabled, and budget escalation. Reopening Configure round-tripped every value and the next real run correctly used `Generation 2 of 3 · full-body`. However, the Loop detail continued to advertise the authored 50-generation defaults and the next-run preview said `Iterates up to 50 generations`; its Advanced section appeared empty with `halt` selected. The operator cannot trust the preview to describe the run that will start.

## Reproduction

- **Charter:** CH-006 · **Tour:** Back-Button Tour
- **Environment:** desktop / wifi-fast / en-US; isolated daemon at `http://127.0.0.1:58941`; in-app browser.

1. Open `software-delivery` and choose Configure.
2. Save iteration cap 3, full-body re-attempt, no-progress 2, fan-out 4, gate revisions 2, human approval on, and budget-on-exceeded `escalate`.
3. Reopen Configure and confirm those exact values round-trip.
4. Return to the Loop detail and inspect Limits & budget.
5. Choose Run Loop, expand Advanced, and inspect What will run without entering per-run overrides.
6. Start the run with a missing task-set slug and inspect its effective facts.

**Expected:** Detail and run preview resolve the saved per-Loop configuration, distinguish inherited defaults from per-run overrides, and describe the same effective cap/strategy that the runtime applies.
**Actual:** Detail and preview show authored defaults (`50` generations and empty/halt Advanced state), while run `looprun-acb65149c8fc91a5` applies the saved `3` generation cap and `full-body` strategy.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-006-saved-config-sheet.png`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-006-stale-run-preview.png`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-006-effective-config-run.png`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-006-effective-config-preview-fixed.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-006-loop-run-override-fixed.png`
- Independent runtime read: `looprun-acb65149c8fc91a5` rendered `Generation 2 of 3 · full-body re-attempt`.

## Fix

- **Root cause:** Run creation resolved definition defaults, saved workspace-scoped `loop_config`, and per-run overrides, while the detail and Run routes loaded only the Loop definition. The Run form also maintained a second stale preview projection that ignored its current override draft.
- **Fix:** Preload and query the existing `(workspace_id, loop_name)` configuration read model on both pre-run routes; project saved values into limits, strategy, lifecycle, and override placeholders; derive `config_overrides` once in the form view-model and reuse that exact projection for both the request and the live preview. A daemon Dry-run `effective_config` remains authoritative when present.
- **Fix commit:** pending final task commit
- **Regression test:** Canonical Loop config/limits/override model suites, route-preloading integration, Loop detail and Run-form component suites, and the interactive Run-form Storybook state. The final Web gate passed 391 files and 3,326 tests.

## Verification

- Same-persona in-app-browser replay passed on 2026-07-13. Detail rendered cap 3, `escalate`, no-progress 2, fan-out 4, and gate revisions 2. The untouched Run form rendered 3 generations with full-body re-attempts and saved values as placeholders rather than overrides. Entering a per-run cap of 4 changed the badge to `overrides set` and the live preview to 4 generations; Cancel and reopen restored the saved baseline of 3.
- Scoped format/lint passed with zero warnings, React Doctor scored 100/100, and the repo-root Web typecheck/test lane passed 3,326/3,326 tests. The isolated Storybook and capture-browser processes were torn down; controller daemon/Web PIDs remained alive.
