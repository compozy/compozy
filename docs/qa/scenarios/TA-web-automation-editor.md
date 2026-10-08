---
id: TA-web-automation-editor
area: TA
title: One automation editor creates and edits any automation
persona: Dora
journey: J-24
expected: "One dialog (Automation / New automation, \"Choose when it starts and what it does. You can turn it off any time.\") orders Name → Starts → Only if (events and links only) → Does → Options. Starts = On a schedule creates a job; When something happens or When another app calls a link creates a trigger. The schedule builder offers Repeats (quick picks, days, time, Edit expression with the min · hour · day · month · weekday hint) · Every… · Once with live readouts and the past-time and malformed-expression warnings; there is no time-zone picker. Event cards: A session starts · A session stops · A hook finishes (hook name field) · An extension sends an event; `memory.consolidated` is never offered. The link start shows the Global notice, Link name, Webhook id and a write-only Signing secret. Only if rows are field = value with the friendly name and path and Add a value or remove this condition for empty values. Does = Ask an agent (searchable agent picker, Sent as written for schedules, detail chips for events), Start a Loop (inputs, Fill Loop inputs from the event details. for events, Loop start-availability warning), or Create a task (schedules only; disabled for events with Only scheduled automations can create tasks). Options folds with a mono summary and opens itself in edit mode, with retries on, or when Off; task targets lock Retry. The sentence bar (aria-live) reads Ready or Needs a fix with dashed missing parts and disables Create automation. Show preview swaps the body (next runs, or sample event with matches this sample / won't start on this sample) and Back to form keeps every value; the footer keeps the destination statement, Cancel and the primary in both views, and a blocking target issue is stated in the form view. Create toasts Created <name>. and opens the detail; a name conflict keeps the dialog open with the field error. Edit reads Edit automation · Changes apply from the next run. with Starts and Does locked and their reasons. Deep links `?create=1&start=…` and `?create=loop&start=…&loop=<name>` open preselected and leave the URL on close."
entry_points: web Automations New automation; empty-state starts; palette New scheduled automation / New automation on an event; Loop page Automate ▾; detail Edit and Set up retries
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-web-automations-first-run; TA-web-automations-shell-entry; TA-automation-crud-loop-target; MS-web-entity-modal-shell; LP-017; LP-047
---

Replaces `TA-web-automation-preview-toggle` (Automations spec task 07; US-019–US-027). The job and trigger forms merged into one editor; the footer preview toggle, the form-view target issue and the shared agent picker carry over. Predecessor history lives in `docs/qa/reports/2026-10-02-untested.md`.

Regression obligations carried over: preview and recovery directions name the available footer action (BUG-20261004-automation-preview-stale-directions); a locked or incompatible saved Loop target explains its repair (BUG-20261004-automation-locked-target-repair-copy); rejected mapping examples and hidden server validation errors stay visible (BUG-20261003-loop-mapping-example-rejected); a workspace Loop-target submit is never inert (BUG-20260713-workspace-trigger-loop-submit-inert). The unsaved-changes confirm applies on close and pointer-outside does not dismiss. Visual contract: `docs/design/opendesign/automations/automations-editor.html` (editor VC-01…05).
