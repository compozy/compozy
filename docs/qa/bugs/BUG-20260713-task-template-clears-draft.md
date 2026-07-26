# BUG-20260713-task-template-clears-draft: Changing the task template erases the draft

- **Status:** verified
- **Impact (user-side):** Data-Loss
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-task-tree create parent, step 1
- **Scenarios:** TA-task-template-preserves-draft
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Bruno filled the title and description in the Simple Create task modal, then changed the template from `Run once` to `Break into steps`. Both authored fields were immediately cleared and `Enqueue task` became disabled without warning. Re-entering the same content after choosing the template succeeded, proving that template selection resets the whole draft rather than only template-owned fields.

## Reproduction

- **Charter:** CH-task-template-draft · **Tour:** Back-Button Tour
- **Environment:** desktop / wifi-fast / en-US; isolated workspace Tasks UI.

1. Click `Task` and keep Simple mode.
2. Enter a non-empty title and description.
3. Select `Break into steps`.
4. Inspect the contract fields and submit state.

**Expected:** Changing a template preserves operator-authored title/description and changes only template-controlled defaults, or explicitly asks before discarding input.
**Actual:** Title and description are silently emptied and the action becomes disabled.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/journey-log.jsonl`
- Parent task `task-4b4a98ccf636c99b` was created only after entering the same fields a second time.

## Fix

- **Root cause:** The create-route draft identity included the selected template ID. Changing templates therefore selected a brand-new initialized draft, discarding operator-authored fields along with template-owned defaults.
- **Resolution:** Keep one draft per workspace/global scope and apply only the selected template's priority, retry, approval, network-channel, and draft defaults to the existing editor state. Simple/Advanced mode transitions now share the same draft.
- **Fix commit:** pending
- **Regression test:** The canonical Task create-route suite exercises every Simple preset plus a Simple → Advanced → Simple round trip while asserting authored contract and advanced fields remain stable; the Task editor projection suite covers preset application as a pure draft transformation.

## Verification

- **Passed 2026-07-13:** Bruno entered `QA retained template draft` and its authored description, selected `Break into steps`, switched Simple → Advanced → Simple, and observed both fields unchanged on every fresh DOM read. The modal was then cancelled, so the verification created no task.
- Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-template-draft-fixed.dom.txt`.
