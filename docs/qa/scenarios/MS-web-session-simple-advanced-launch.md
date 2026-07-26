---
id: MS-web-session-simple-advanced-launch
area: MS
title: Start session combines simple identity fields with a first-message composer
persona: Dora
journey:
expected: Opening Start session shows the Simple identity fields — agent picker, workspace picker, and optional session name — plus the required first-message composer. The composer owns the chromeless RuntimeSelector and its Send disc is the only creation action. Switching to Advanced reveals Network participation and working path without hiding any Simple field or the composer. Switching back to Simple resets only those hidden advanced values, so they cannot block Send; the visible runtime choice remains intact. Sending atomically queues the trimmed first prompt, includes `name` and `workspace_path` only when non-empty, and omits `provider`, `model`, and `reasoning_effort` while runtime inheritance remains untouched. Choosing another workspace retargets the launch and clears the workspace-scoped agent, prompt, runtime, and Network selections. Catalog state (loading, refreshing, stale, empty, error) stays inside the RuntimeSelector popover. The header carries the only close control.
entry_points: web desktop shell → Start session (dock, command palette, agent catalog, agent detail, dashboard)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-01; .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-02; .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-09
last_report:
overlaps: MS-web-entity-modal-shell; NB-participation-controls-serialize
---

story: As an operator I choose who runs and where, write the first message, and send one durable session-create request without losing access to runtime or advanced launch controls.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.2), task_02, implemented 2026-07-25. Before this change the dialog was runtime-first: it had no workspace picker and no session name, and it always sent the active workspace.

The workspace picker and session name are new writes against `CreateSessionRequest` fields the daemon already accepted (`internal/api/contract/session_runtime_payloads.go`). Network participation keeps its existing control (mode, channel id, strategy) rather than the single channel select in the artboard, because `network_participation` is the only channel-bearing field on the contract.

QA impact 2026-07-26: first-message creation is atomic and RuntimeSelector lives in the composer. The modal shell keeps Simple/Advanced disclosure, while only Network participation and working path remain hidden in Advanced. Scenario remains untested; flag only.

src: web/src/systems/session/components/session-create-dialog.tsx; web/src/systems/session/components/session-create-simple-section.tsx; web/src/systems/session/components/session-create-advanced-section.tsx; web/src/systems/session/hooks/use-session-create-dialog.ts

inventory: Needs QA
