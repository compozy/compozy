---
id: NB-participation-controls-serialize
area: NB
title: Session/task/automation participation controls serialize Local by default
persona: Bruno
journey: J-network-local-default
expected: Session create, task editor, and automation task drafts serialize network_participation with Local default and never include legacy participation channel/network_channel/coordination_channel_id fields on those create payloads.
entry_points: web session create, task editor, Loop run, and automation job/trigger drafts; HTTP/UDS/CLI/native owner create/edit/start verbs
qa_status: untested
bug_ids: BUG-20260715-loop-participation-contract-dropped;BUG-20260715-automation-task-participation-control-missing;BUG-20260715-automation-editor-compact-layout-clipped;BUG-20260715-loop-run-compact-layout-collapsed
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md
last_report: docs/qa/reports/2026-07-14-network-changes.md
overlaps: NB-execution-participation-defaults
---

Planning flag for Task 05 participation controls co-ship.

QA impact 2026-07-25: task_01 of the modal redesign replaced the task editor's hand-rolled footer/toolbar with the shared EntityDialogFooter/EntityModeToolbar and lifted the automation editor header onto EntityDialogHeader. Serialization is unchanged, but the surfaces this scenario exercises changed visibly, so the prior pass verdict is stale. Reset to untested — flag, do not retest in task_01.
