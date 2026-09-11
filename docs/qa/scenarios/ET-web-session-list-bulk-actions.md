---
id: ET-web-session-list-bulk-actions
area: ET
title: Select and act on sessions in the shared workspace list
persona: Bruno
journey: J-14
expected: Both the session window sidebar and Sessions modal support a transient selection in workspace scope. Eligible actions run sequentially through existing session mutations. Selection follows current catalog membership; successful Stop keeps rows selected, Archive and Unarchive prune rows that leave the view, and failed rows remain selected. Delete confirms the set, reports progress, and retries only failed sessions.
entry_points: SessionSidebar; Sessions modal in workspace scope; row checkboxes and modifier clicks; Selected sessions toolbar
qa_status: untested
overlaps: ET-web-session-sidebar-threads
---

Added 2026-09-11. The visual contract is
`docs/design/opendesign/sessions-bulk-actions/index.html`; runtime and selection
scope follow the controller's membership and row-action decisions.

Use an isolated workspace with at least three sessions: one running and two
stopped. Include a root with expanded children for the range walk. Repeat the
selection controls in the in-window sidebar and the dock's Sessions modal.

1. Hover or keyboard-focus a row: its named checkbox replaces the status mark
   without moving the title. Plain click opens a session; checkbox or Cmd/Ctrl-click
   selects without navigation. Selecting the first row replaces the toolbar;
   New session and row kebabs disappear while thread toggles remain available.
2. Select three rows. Check the selected count, mixed select-all state, and More
   menu eligibility: Stop for running/starting, Archive for stopped, Unarchive for
   archived. Zero-count actions remain disabled. Selected rows retain a neutral
   plate and the current session keeps its accent marker.
3. Shift-click selects the range in rendered order, excluding collapsed children.
   Filtering and folding preserve selection; the hidden count reflects selected
   rows outside the visible set. Cmd/Ctrl+A inside the list selects visible rows;
   Enter/Space toggles a focused row. Esc or Done clears selection without closing
   the Sessions modal. Outside the list, these selection shortcuts do not apply.
4. Stop the running selection. Verify completion and one batch toast. Rows stay
   selected, and Archive becomes eligible from the updated payload. Archive them:
   successful rows leave the active catalog and selection ends. Open Archived,
   select them, and Unarchive: successful rows leave that catalog and are pruned.
   A failed mutation leaves its row selected and exposes the daemon message.
5. Switch between workspace and all-workspaces scope, or change the Archived view:
   selection clears. All-workspaces groups have no checkboxes, bulk toolbar, or
   select-all shortcut because their per-row lifecycle actions are unavailable.
6. Select three sessions and Delete (toolbar, More, or Cmd/Ctrl+Backspace). The set
   dialog names the count, shows status rows, and notes the active count (daemon state; idle sessions count too). Cancel
   preserves selection. With one selected session the existing singular dialog is
   unchanged. More than five targets show five rows plus the overflow count.
7. Confirm: deletion progresses sequentially, Cancel/close are unavailable, and
   neither the dialog nor its host dismisses on Esc/outside click. On success the
   dialog closes, one toast reports the deleted count, rows disappear from the
   real catalog, and selection clears while the host remains open.
8. For a reproducible daemon failure, verify successful and failed results remain
   visible, the error text names the daemon cause, Close keeps only failed rows
   selected, and Retry acts only on failed IDs. A partial result emits no toast
   while the dialog is open. Close or a successful Retry emits one success toast
   with the total deleted across all attempts (singular for one, none for zero).
   The deterministic lifecycle suite owns this I/O failure case when a real
   failure cannot be induced safely.

Verification ownership: `session-bulk-actions.spec.ts` exercises confirmed deletion
against an isolated daemon using the existing ACP lifecycle fixture. The selection,
list, modal, dialog, and lifecycle suites own keyboard/range, scope, eligibility,
progress, partial-failure, and retry invariants. Storybook `SelectionMode`,
`SelectionMenuOpen`, `Single`, `ConfirmSet`, `DeletingSet`, and `PartialFailure`
provide the controller's visual-review entry points. This new manual scenario
remains `untested` until that complete walk is recorded; automated evidence is
reported separately in the delivery report.
