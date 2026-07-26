# BUG-20260713-workspace-trigger-loop-submit-inert: A workspace Trigger cannot create its selected Loop target

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-24 create a workspace-scoped Loop-target Trigger
- **Scenarios:** TA-automation-crud-loop-target; LP-035
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md

## Summary

The corrected Trigger selector offers only `reviews-watch` for both ordinary `trigger` events and `webhook`, but the original workspace-scoped creation path did not bind the selected Loop target to the selected workspace. The live preview said `Workspace: Not selected`, the normalized outer request contained `workspace_id`, and the nested `loop_target.workspace_id` was empty. `Create trigger` remained enabled, but activating it left the modal open, created no Trigger, and showed no error.

The first repair fixed the Web binding and mutation error visibility. A backend residual then returned HTTP 500 because registration normalization dropped the validated typed Loop target before persistence. The final same-persona replay created the Trigger successfully and read the workspace, Loop, typed input, filter, enabled state, and empty initial run history back from the detail surface.

Webhook correctly forces Global scope and explains that constraint. Switching back to `session.stopped` and explicitly reselecting Workspace restores the outer scope but not the nested Loop target workspace.

## Reproduction

1. Open Triggers and choose Create Trigger; keep the initial Workspace scope.
2. Keep `session.stopped`, choose Run loop, and select the only compatible option, `reviews-watch`.
3. Enter required static input `pr = 2`.
4. Select webhook and confirm the explicit forced-Global explanation, then return to `session.stopped` and explicitly select Workspace again.
5. Inspect the live Loop target and normalized request.
6. Activate the enabled Create trigger action once and inspect the list/modal/log surfaces.

**Expected:** A workspace-scoped Trigger binds its Loop target to the selected workspace. Preview and request agree, and enabled submit either creates the Trigger or exposes an actionable error. Global/webhook semantics remain distinct.
**Actual:** The first replay reported `Not selected`, an empty nested workspace, and a silent no-op; the intermediate Web-only repair reached HTTP 500. After the complete fix, one UI submission created `qa-workspace-loop-trigger-fixed`. Fresh detail reads show `session.stopped`, `data.session_type=system`, `reviews-watch`, `ws_06366aad69887872`, `{ "pr": 2 }`, ENABLED, and zero runs before its first matching event.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-scope-initial-workspace.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-scope-reset-global.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-workspace-loop-submit-inert.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-workspace-loop-fixed-ready.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-workspace-loop-fixed-created.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-workspace-loop-fixed-ready-final.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-workspace-loop-fixed-created-final.dom.txt`
- Isolated daemon log: `POST /api/automation/triggers` returned status 500 in 56 ms at `2026-07-13T07:08:41-03:00`.

## Fix

- **Root cause:** Confirmed. The Web draft treated the outer Trigger workspace and nested Loop target workspace as independent values. After that projection was corrected, backend normalization/clone code still rebuilt the registration without the already-validated typed Loop target, so persistence/dispatch lost the target and failed internally.
- **Fix commit:** pending
- **Regression test:** Canonical Trigger form/route coverage proves workspace projection, compatible start-kind filtering, typed inputs, submit failure visibility, and Global/webhook preservation. Automation/core owning suites prove typed Loop target preservation through registration/clone and truthful missing-Loop 404 mapping.

## Verification

- 2026-07-13 same-persona UI retest passed create and fresh detail read-back with a workspace-scoped, filtered, enabled Loop target and typed static input. Integrated matching-event dispatch plus edit/disable/safe-delete remain in the parent automation scenario, not this defect's acceptance boundary.
