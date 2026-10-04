# BUG-20261004-automation-preview-stale-directions: Preview guidance points to an absent pane

- **Status:** fixed — verified
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Dora
- **Journey Step:** J-24, inspect an automation draft and recover from an invalid schedule
- **Scenarios:** TA-web-automation-preview-toggle
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The automation editors swap their form and preview. Webhook setup still directs Dora to the
right for the endpoint example, while an invalid schedule preview directs her above for fields
that are currently unmounted. The prompt-template help contains the same obsolete direction.

## Reproduction

- **Charter:** CH-untested-026-24-dora, row 63 only · **Tour:** Garbage Tour
- **Environment:** desktop Chrome, 1512x862, en-US, real isolated daemon on 55651;
  source 3f53932aa, documentation-only head 32417c599.

1. Open Create trigger and choose Incoming webhook.
2. Read the instruction that the URL and curl example are on the right; only the form is mounted.
3. Cancel, open Create job, choose Every interval and enter `two hours`.
4. Open Show live preview; the empty next-run state says to fix the schedule above.
5. Observe that the interval field is absent and Back to form is the available recovery action.

**Expected:** instructions identify Show live preview or Back to form as appropriate.
**Actual:** obsolete spatial directions describe the former permanent preview layout.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/:

- automation-preview-dora-directions-session-ended.json
- automation-preview-dora-webhook-stale-direction.png
- automation-preview-dora-schedule-stale-direction.png

Both screenshots were inspected. The original recording stopped with 144 frames at
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/automation-preview-dora.

## Fix

- **Root cause:** three instructional strings retained assumptions from the former split layout.
  The owning form and preview components already implement the correct view swap.
- **Owning layer:** next-runs-card, event-sub-config and prompt-template-field presentation.
- **Repair:** name the existing footer actions; preserve draft, validation and submission behavior.
- **Regression proof:** before/after real Dora replay. No prose-only automated assertion is added;
  existing automation job/trigger form and editor-dialog suites own behavior coverage.
- **Fix commit:** a2917318c96359b507b0e0a0fc06ad410f7dd648.

## Verification

Fresh Dora replay confirms all three corrected directions in the rendered UI, task preview,
webhook endpoint/curl and template hover help. The original draft-retention walk remains valid.
Evidence: automation-preview-replay-directions-ended.json and the inspected replay screenshots.
Root Turbo build, the affected delivery gate and existing form suites pass. Fix SHA: a2917318c96359b507b0e0a0fc06ad410f7dd648.
