---
id: TA-web-automations-first-run
area: TA
title: Start a first automation from the empty Automations window
persona: Cora
journey: J-start-from-empty-catalogs
expected: "With zero automations and no filters, `q` or `start`, the Automations window hides the toolbar and reads \"No automations in <profile> yet\" · \"An automation runs an agent, a Loop or a task on a schedule, or when something happens.\" (\"No automations in any profile yet\" in aggregate mode). Two neutral buttons, On a schedule and When something happens, open the editor with Starts preselected (`?create=1&start=schedule|event`); cancelling returns to the unchanged empty state with nothing created. In workspace scope, live daemon suggestions render under Suggested automations as sentences with Create automation and Dismiss; Create automation toasts the name and the listing shows the new row, Dismiss removes the proposal, and both persist across refresh. Zero suggestions render nothing; Global scope and aggregate mode offer none. A populated or filtered listing never shows the first-run state or suggestions; filtered-empty offers Clear filters."
entry_points: web `/automations` on a project with no automations
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/automations/screens/; .compozy/tasks/automations/reports/q1-qa-walk.md (Round 2)
last_report: .compozy/tasks/automations/reports/q1-qa-walk.md
overlaps: TA-automation-suggestions; TA-web-automations-listing; TA-web-automation-editor
---

Replaces `TA-web-jobs-zero-inventory-suggestions` and `TA-web-triggers-zero-inventory-intro` (Automations spec task 07; US-008, US-016 filtered-empty rule). Accept/dismiss semantics stay owned by `TA-automation-suggestions`; this scenario owns the zero-inventory composition and first-use interaction. Predecessor history lives in `docs/qa/reports/2026-08-15-pr409-empty-states.md`.

The event start has no suggestions by design: the daemon only proposes scheduled jobs, so the absence of event suggestions is runtime truth, not a gap. Walk loading, error, Global-scope and no-workspace states deliberately, as the predecessors did. Visual contract: `docs/design/opendesign/automations/automations-list.html` (empty-state artboards).
