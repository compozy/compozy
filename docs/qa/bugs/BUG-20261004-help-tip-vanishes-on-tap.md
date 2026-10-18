# BUG-20261004-help-tip-vanishes-on-tap: Help guidance disappears immediately after tapping

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, read field guidance using a pointer or touch
- **Scenarios:** MS-web-modal-help-tips; MS-web-entity-modal-shell
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Clicking the task editor's help button does not leave its explanation readable. On a narrow touch
viewport, the sentence appears briefly after tapping and disappears without another action.
Keyboard focus and pointer hover can expose it, but touch has no usable hover alternative.

## Reproduction

- **Charter:** CH-untested-041-administer-runtime-settings-dora · **Tour:** Back-Button Tour
- **Environment:** real isolated daemon and production Web bundle, Chrome en-US; desktop
  1512 × 862 and touch emulation at 720 × 900, one touch point

1. Open Tasks, then New task. Click the Title field and click About what needs doing.
2. The help button receives focus, but its explanatory sentence is absent after three seconds.
3. Reopen the task editor at the narrow touch viewport and tap the same 44 × 44 CSS-pixel button.
4. Without another interaction, observe the tip disappear. A fresh repetition captures it in the
   first sample and absent in the following 18 samples across two seconds.

**Expected:** Activation leaves the explanation readable until the user dismisses it.
**Actual:** Click/tap opens and immediately closes the guidance.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/:

- help-tip-escape-dora-click-observed.json / help-tip-escape-dora-task-click.png.
- help-tip-escape-dora-touch-job-entry.json: 44px target; transient AX text is not a stable pass.
- help-tip-escape-dora-canary-complete-ended.json / help-tip-escape-dora-touch-stability.png:
  repeated fresh tap, continuous samples, restored desktop and closed 19-frame recording.
- help-tip-escape-dora-task-after.json / help-tip-escape-dora-jobs-after.json: no synthetic save.

All five replay screenshots were inspected. The separate Escape draft-loss repair succeeds for
task and job forms; this finding concerns activation, not dismissal of their enclosing dialogs.

## Fix

- **Root cause:** HelpTip explicitly opens on click but left TooltipTrigger's default
  `closeOnClick` behavior enabled. Disabling it repairs mouse activation. Touch also emits a
  compatibility `mouseleave` after the tap; the explicit controlled opener does not tell Base UI's
  hover interaction that touch owns this open lifetime. That mouse event then closes the tip.
  The original test asserted transient mounting before the exit animation finished.
- **Fix commit:** 6aec027349214211c68315607d9ad7b4c5b93276.
- **Regression test:** Existing shared HelpTip suite; require mouse/touch activation to retain
  prose through a one-second reading interval, and retain ordinary Escape dismissal. The touch
  case fails before repair (nine pass / one fail); ordinary mouse is clean in jsdom, while its
  browser reproduction is retained separately. The initial fake-clock harness stalled user-event
  and is not product regression proof. Real timers keep the full interaction sequence intact.

The repair sets the documented `closeOnClick={false}` on HelpTip's TooltipTrigger and records
touch input from pointerdown before the synthesized click. While touch owns the tip, the component
cancels only hover dismissal through Base UI's public change-event API. Outside press, blur and
Escape still dismiss normally and clear that ownership. No timer, focus workaround, library
patch or generic-tooltip behavior change is introduced.

## Re-found after the first activation repair

The final production bundle (help-tip-final-build-identity.json) retains desktop-click guidance,
but actual Chrome touch still disappears. A fresh Create task dialog reproduces one visible sample
and 18 absent samples without another action. See help-tip-final-dora-fresh-touch-job-entry.json
and help-tip-final-dora-fresh-touch.png. The closeOnClick conflict is only a partial cause;
further diagnosis is required before this finding can be verified.

## Causal probe and regression

The closed engineer-only recording help-tip-touch-diagnostic and receipt
help-tip-touch-engineer-event-trace.json capture the actual event order: touch click at 2892ms,
open content at 2897ms, compatibility mouseleave at 2899ms, and closed content at 2900ms.
No second user interaction occurs. This rules out a stale exit-animation callback as the cause.
The existing HelpTip touch test now includes that native leave event and fails before the repair
(help-tip-touch-leave-regression-red.json); the existing Sheet suite passes in that same run after
waiting for its documented asynchronous initial-focus readiness.

A first ownership implementation read pointerType from click. The installed user-event emits a
compatibility mouse click without that metadata, so its touch regression correctly stayed red.
The final boundary captures pointerType from pointerdown, where the standard input event carries
it, and preserves the consumer's handler. The browser trace already shows this earlier event.
The receipt named help-tip-touch-leave-focused-green.json has exit 1 despite its intended name;
its contents, not the filename, own the result.

## Successful original-persona replay

Dora's fresh help-tip-touch-dora session passes against source hashes recorded in
help-tip-touch-pointer-build-identity.json. Task hover/focus/click and repeated narrow touch
retain readable guidance and preserve the form's input on first Escape. Outside touch dismisses
only the tip; second Escape explicitly closes the form. The Job canary retains name, prompt and
caret, with continued typing. No drafts are saved, confirmed by independent CLI queries.
All five screenshots are inspected and the 20-frame recording is closed. Evidence:
help-tip-touch-dora-task-paths.json, help-tip-touch-dora-job-ended.json and the paired
help-tip-touch-dora-*-after.json receipts. The frozen staged gate and commit proof confirm delivery of this repair locally.

Verified at 6aec02734: all affected local gate lanes are CURRENT-PASS for tree
8cc4147cbf9e055535073ccc95237cb8a3f77e37, exactly preserved by the commit. See
help-tip-frozen-staged-delivery-gate.json, help-tip-frozen-staged-gate-status.json and
help-tip-preserve-guidance-commit-proof.json. Current-head PR CI remains outside this local claim.
