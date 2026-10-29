# QA Run Report — 2026-10-29 — Overlay focus handoff

- Scope: issue #1035; project menu, overview, and creation/confirmation dialogs.
- Cadence: targeted. Persona: Ada, a keyboard-first desktop user managing projects.
- Build: `fix/issue-1035-overlay-focus`, based on `3ce25a0c29f95395305ed527cce901d703eb7b33`.
- Environment: isolated daemon 0.4.5, current worktree Vite frontend, Chrome 156 on macOS.
- Status: PASS for targeted browser behavior and focused local checks. PR CI/reviews are separate delivery gates.

## Session matrix

| Journey | Scenario | Status | Evidence |
| --- | --- | --- | --- |
| Project menu → Workspace picker → Escape | ET-web-menubar-menu-set | PASS | Keyboard walk and focus/AX inspection |
| Overview → New project → Cancel | RT-workspace-overview-command-tab | PASS | Keyboard walk and focus/AX inspection |
| Project menu → Add project → Cancel | ET-web-menubar-menu-set | PASS | Keyboard walk and focus/AX inspection |
| Overview tile context menu → Archive project → Cancel | RT-workspace-overview-command-tab | PASS | Keyboard walk and focus/AX inspection |
| Nested dialog focus and restoration | Shared Dialog story | PASS | Play function completed; focus returned to Open projects |
| Context menu focus and restoration | Shared ContextMenu story | PASS | Play function completed; focus returned to Project tile |

## Baseline investigation

The overview → New project keyboard transition produced a `focusin` on the project
menubar trigger while its ancestor already had `aria-hidden="true"`, followed by
focus entering Add project. The existing overview suite reproduced that same
observable and failed before the fix (40 passed, one regression failed).

Overview → tile context menu → Archive project emitted Chrome's native
`Blocked aria-hidden on an element because its descendant retained focus` warning.
The focused element in this capture was a context-menu item. The nested-dialog
regression independently observed the originating control still focused when its
dialog became hidden (19 passed, one regression failed before the fix).

Opening the tile context menu also produced a MutationObserver record of the
focused overview tile beneath an aria-hidden ancestor. Both original focus
locations were therefore reproduced as DOM focus violations.

The exact original tile and menubar warning strings have not both been captured
deterministically. DOM focus violations and native console warnings are distinct
evidence; the captured context-menu warning is not relabeled as either original.

## Fix and focused checks

- The overview suppresses trigger restoration when handing off to project/worktree creation.
  Ordinary Escape still restores the project trigger.
- Dialog's mounted popup transfers default/reference focus in a layout effect before
  Base UI's passive isolation effect. Base UI retains boolean/callback focus policies.
- ContextMenu transfers focus before hiding the containing overview; submenus retain
  their existing hover/keyboard focus policy. Its new regression failed before the fix.
- The overview ignores keyboard events from portaled surfaces. Before this change,
  Enter on Archive project also selected the tile and closed the overview. The owning
  regression failed before the fix and passed afterward.
- Overview suite: 42 passed after the fix.
- Dialog, ConfirmDialog, ContextMenu and DropdownMenu suites: 38 passed.
- React Doctor: no issues in the nine changed source/test/story files, via Turbo.
- Web/UI typecheck and lint: four Turbo tasks passed after building the SDK client
  prerequisite; lint reported zero warnings/errors. The first web typecheck lacked
  the SDK's generated distribution and was not a product regression.

## Recorded browser walk

On Chrome 156, with focus-event/attribute observers and browser log capture:

1. Project menu → End → ArrowUp → Enter opened Projects. Escape returned focus to
   the project trigger. Project menu → End → Enter opened Add project; Tab stayed
   inside the dialog and Escape returned to the visible project trigger.
2. Cmd+Shift+O → End → Enter transferred focus from New project to Add project's
   dialog container. No intermediate focus landed on the hidden project trigger.
3. Cmd+Shift+O → right-click project tile → ArrowDown → Enter opened Archive
   confirmation. The accessibility tree exposed the named dialog and focused Cancel.
   Enter canceled without archiving and returned focus to the project tile.
4. Both added Storybook play functions completed in a real browser. Nested Dialog
   returned focus through its originating action to Open projects. ContextMenu
   returned focus to Project tile after Escape.

The recorded final application walks had zero hidden-focus observations and zero
retained-focus aria-hidden console warnings. Menu/dialog roles and accessible names
were checked in Chrome's accessibility tree; modal isolation remained enabled.

## Evidence boundaries

Browser Use's shared default connection changed its active tab to another agent's
lab during investigation. Observations from that interval were discarded; subsequent
walks explicitly select and verify this lab's target. Vite initially retained
stale transformed modules; affected walks were repeated after restarting the owned
dev server and confirming the corrected source was served. No data mutation was performed
in the other lab.

Accessibility-tree inspection verifies names, roles, and focused nodes. Native
screen-reader speech has not been verified. Heavy local gates and full E2E are
excluded by the assignment; current-head CI and both PR reviews remain required.
