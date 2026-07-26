---
id: MS-web-task-editor-window-modal
area: MS
title: Task create and edit present as window-scoped modals over their origin surface
persona: Dora
journey:
expected: "`/tasks/new` renders the tasks catalog with the task editor layered over it as a window-scoped dialog on the `--width-modal-md` host, and `/tasks/$id/edit` renders the task detail with the same dialog over it — the scrim dims only the owning window and other windows stay interactive. The location carries the surface underneath: opening the editor from Kanban, Dashboard, or Inbox keeps that view behind the dialog and returns to it on Cancel/Escape/close, and opening Edit from the Runs or Activity tab returns to that tab. Template selection stays addressable through `?template=`; a successful create navigates to the created task and a successful edit returns to the task detail. The edit host never renders a half-bound form: it shows a loading state while the task resolves and an explicit unavailable state when it cannot be read, with dismissal reachable in both."
entry_points: web `/tasks/new`; web `/tasks/$id/edit`; tasks topbar "New task"; tasks empty-state template cards; Kanban create action; task detail Edit action
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task-editor-window-modal/
last_report:
overlaps: MS-web-entity-modal-shell; TA-001; TA-004; ET-web-tasks-mode-url
---

story: As an operator I create and edit tasks in a modal over the list I was reading, so the surface I came from stays visible and dismissal puts me back exactly where I was.

The OS-shell migration (`feat: os shell implementation (#330)`) had converted both editors into full-window locations under the "route-backed modals become in-window locations" decision. That decision's own rule of thumb reserves internal navigation for wizard-class (`lg`/`xl`) flows; the task editor is a `--width-modal-md` single-entity form, so `MODAL-STANDARD.md` § Hosts puts it back on the dialog host. Restored 2026-07-25.

The dialog host is `TaskEditorModal`; the window scoping comes from `OverlayContainerContext` supplied by `os-window.tsx`, the same seam bridge, knowledge, and confirm dialogs already use.

src: web/src/systems/os/apps/tasks/task-editor-dialogs.tsx; web/src/systems/os/apps/tasks/tasks-window.tsx; web/src/systems/os/apps/tasks/task-window-location.ts; web/src/systems/tasks/components/task-editor-modal.tsx

inventory: Needs QA
