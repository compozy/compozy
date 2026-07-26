# BUG-20260713-loop-automation-start-mismatch-late: Automation forms present incompatible Loop starts as valid

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-24 create a Loop-target Job or Trigger
- **Scenarios:** TA-automation-crud-loop-target; LP-035
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md

## Summary

The Job and Trigger modals list every workspace Loop without reconciling the automation kind with the Loop's declared `start[]` allowlist. The Job form allowed Bruno to select trigger-only `reviews-watch`, rendered a complete valid-looking schedule preview, and enabled Create. Only the submit response revealed `start_kind_not_allowed`. The Trigger form likewise offers schedule-only `software-delivery`.

The runtime correctly rejects the invalid binding, but the user-facing validation is late and contradicts the preview.

## Reproduction

1. Open Jobs and choose Create Job.
2. Select Workspace, Run loop, and trigger-only `reviews-watch`.
3. Enter the required typed `pr` input and inspect the Live preview.
4. Choose Create job.
5. Repeat the inverse in Create Trigger with schedule-only `software-delivery`.

**Expected:** The modal uses the authoritative Loop start contract to exclude or clearly disable incompatible targets before submit. An existing automation whose Loop becomes incompatible remains inspectable but cannot be saved or fired without a truthful explanation.
**Actual:** The preview and enabled submit action present the incompatible binding as valid; the backend then rejects it with `start_kind_not_allowed`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-loop-job-incompatible-start.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-job-schedule-filter-fixed.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-workspace-loop-submit-inert.dom.txt`
- Browser rejection: `start_kind_not_allowed: loop: validation failed: loop "reviews-watch" does not declare start kind "schedule" (kind=schedule, loop=reviews-watch)`.

## Fix

- **Root cause:** The automation target selector consumed the workspace Loop catalog without projecting each definition's declared `start[]` kinds into option availability or submit validation.
- **Fix:** Centralize start-kind availability in the workspace-scoped Loop target catalog. Job requires `schedule`; ordinary event Triggers require `trigger`; webhook Triggers require `webhook`. A persisted incompatible target remains visible and explained in edit mode while preview, request, and Save stay blocked.
- **Fix commit:** pending final task commit
- **Regression test:** Canonical shared Loop-target field plus Job/Trigger form suites own compatible filtering, persisted incompatible options, event-kind transitions, and blocked requests. The worker's repo-root Web lane passed 3,360 tests and typecheck; scoped format/lint and React Doctor 100/100 also passed.

## Verification

- Same-persona in-app-browser replay passed on 2026-07-13. Create Job offered only schedule-capable `software-delivery`; trigger-only `reviews-watch` was absent. Create Trigger offered only `reviews-watch` for `session.stopped`, and switching to webhook retained only the webhook-capable option; schedule-only `software-delivery` was absent. Job preview/request used the selected compatible Loop with typed inputs.
- A separate workspace Trigger target-binding/submit defect discovered during the same replay is tracked as BUG-20260713-workspace-trigger-loop-submit-inert; it does not regress start-kind filtering.
