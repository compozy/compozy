---
id: ET-web-session-list-bulk-actions
area: ET
title: Select and act on sessions in the shared workspace list
persona: Bruno
journey: J-14
expected: Both the session window sidebar and Sessions modal support a transient selection in workspace scope. Eligible actions run with bounded concurrency through existing session mutations; related session trees retain their ordering. Selection follows current catalog membership; successful Stop keeps rows selected, Archive and Unarchive prune rows that leave the view, and failed rows remain selected. Delete confirms the set, reports progress, and retries only failed sessions.
entry_points: SessionSidebar; Sessions modal in workspace scope; row checkboxes and modifier clicks; Selected sessions toolbar
qa_status: skipped
overlaps: ET-web-session-sidebar-threads; ET-web-session-folders-manual-order
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
   In the filter, Cmd/Ctrl+A keeps its text-selection behavior and Cmd/Ctrl+Backspace
   edits text without selecting sessions or opening Delete.
4. Stop the running selection. Verify completion and one batch toast. Rows stay
   selected, and Archive becomes eligible from the updated payload. Archive them:
   successful rows leave the active catalog and selection ends. Open Archived,
   select them, and Unarchive: successful rows leave that catalog and are pruned.
   A failed mutation leaves its row selected and exposes the daemon message.
5. Switch between workspace and all-workspaces scope, or change the Archived view:
   selection clears. Grouped lists (Group by project or profile) keep one selection
   across their expanded groups, with checkboxes, the bulk toolbar, and the keyboard
   shortcuts; verbs route per owner (issue #904, ET-web-session-list-aggregate-parity).
6. Select three sessions and Delete (toolbar, More, or Cmd/Ctrl+Backspace). The set
   dialog names the count, shows status rows, and notes the active count (daemon state; idle sessions count too). Cancel
   preserves selection. With one selected session the existing singular dialog is
   unchanged before deletion. A one-session bulk failure shows the daemon error
   and offers Retry 1 and Close. More than five targets show five rows plus the overflow count.
7. Confirm: deletion progresses with at most three independent trees in flight, Cancel/close are unavailable, and
   neither the dialog nor its host dismisses on Esc/outside click. On success the
   dialog closes, one toast reports the deleted count, rows disappear from the
   real catalog, and selection clears while the host remains open.
8. For a reproducible daemon failure, verify successful and failed results remain
   visible, including the name and error of each failed target beyond the five-row
   preview. With many failures or long errors, the results region scrolls while
   Close and Retry stay inside the viewport. The error text names the daemon cause, Close keeps only failed rows
   selected, and Retry acts only on failed IDs. A partial result emits no toast
   while the dialog is open. Close or a successful Retry emits one success toast
   with the total deleted across all attempts (singular for one, none for zero).
   The deterministic lifecycle suite owns this I/O failure case when a real
   failure cannot be induced safely.

9. With folders in the workspace, the More menu starts with Move to folder; moving
   the selection into a folder or out of folders, and dragging a selected row, are
   specified and walked in ET-web-session-folders-manual-order. While a folder move
   runs, the selection bar's actions are disabled, and a running lifecycle batch
   disables folder moves.

Verification ownership: `session-bulk-actions.spec.ts` exercises confirmed deletion
against an isolated daemon using the existing ACP lifecycle fixture. The selection,
list, modal, dialog, and lifecycle suites own keyboard/range, scope, eligibility,
progress, partial-failure, and retry invariants. Storybook `SelectionMode`,
`SelectionMenuOpen`, `Single`, `ConfirmSet`, `DeletingSet`, and `PartialFailure`
provide the controller's visual-review entry points. This new manual scenario
remains `untested` until that complete walk is recorded; automated evidence is
reported separately in the delivery report.

## Goal cancellation regression

Include a stopped session that previously owned a Goal in the selected set. Deletion must settle
its owned Run without an internal outbox validation error, then remove the row. Preserve an
unselected neighboring session. For a genuine partial failure, retain its error and retry only the
remaining IDs; previously successful deletions must not be replayed. After upgrading the daemon,
retry a previously failed deletion through the same control without resetting any user state.

The browser suite now exercises mixed active/stopped selection and an unselected neighbor, with
before/confirmation/after screenshots. The exact unbound Goal state belongs to the real SQLite
store/Manager integration suites; the existing bulk lifecycle hook suite owns partial-error and
retry behavior. The issue-specific automated verdict, exact commit, CI jobs and selection/confirmation/completion
screenshots are recorded in [PR #647 — How you verified it](https://github.com/compozy/compozy/pull/647).
The bulk lifecycle suite verifies per-ID error reporting, no premature success toast, retry of only
failed IDs, and the final cumulative count. This scoped automated record does not claim a complete
manual keyboard/range/archive walk.

Issue #651 regression: repeat single and bulk deletion with sessions owned by a
non-default profile. Each DELETE must carry the target's profile and workspace,
even if the active selection changes before confirmation. Verify selected rows
and windows reconcile, unselected sessions remain, and an explicitly wrong-profile
DELETE still returns 404 without removing the target.

With catalog SSE disconnected, delete a session and verify global attention rows
and counts refresh alongside its workspace catalog. Preserve each profile’s
before/confirmation/after captures under distinct names.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

## Selection context menu (issue #1029)

Added 2026-10-29. Walk in both the session window sidebar and the Sessions modal, flat and
grouped (Group by project under All projects).

1. With no selection, right-click a row: the per-session menu (Copy session ID, Copy title,
   Archive/Stop and archive session, Delete session) opens for that row, unchanged.
2. Select one row and right-click it: still the per-session menu for that row.
3. Select two or more rows and right-click any selected row (or focus it and press Shift+F10):
   the menu is headed "N sessions" and offers the selection bar's More verbs with the same
   counts and eligibility (Move to folder when folders exist, Stop · N active, Archive · N
   stopped or Stop and archive · N sessions, Unarchive · N archived), plus Copy session IDs
   (one ID per line, "Session IDs copied") and Delete N sessions, which opens the same set
   confirmation as the toolbar. No single-session verb appears.
4. Right-click a row outside the selection (Finder convention): the selection becomes that one
   row, the bar reads "1 selected", and the per-session menu opens for it.
5. Select a parent and its own child, plus a session with another child. Delete shows the child
   count without the selected child and unchecked. Check it and confirm: the parent's tree goes in
   one request, the selected child reads done without a second request, and a failure keeps its row
   in the result list. Bulk archive asks once with the same count, checked by default.

## Batch menu parity and concurrency (#1464)

Select dismissible Needs you sessions and provenance children alongside ordinary sessions. Both
the selection toolbar and selected-row context menu offer Mark as seen, Make top-level and Copy
titles with the row menu's eligibility. Copy titles preserves selection order. Rename, Fork and
Continue with another agent remain single-session actions. Mark as seen clears only dismissible
attention; Make top-level detaches only eligible nested user sessions. Ineligible rows are counted
with a reason, and failed requests name each target without hiding successes.

For about 15 sessions, exercise Stop, Archive, Stop and archive, Delete, Later, Unarchive and Mark
as seen. Observe up to three independent trees progressing together; parent/child archive and
delete operations retain root-before-covered-child ordering. Compare elapsed time with the
sequential baseline, retaining errors and retry receipts. Verify a batch stays locked until its
last outstanding request completes, even when later requests finish first.

Issue #1464 evidence (2026-11-10): see
[the targeted validation report](../reports/2026-11-10-issue-1464.md). The report distinguishes
rendered session-menu/lifecycle checks, real API/Git timings, deterministic partial-failure
coverage, and the public screenshot-upload blocker. This does not replace unrelated journeys
in this scenario.

## Batch follow-ups: progress, shared admission and lineage lock (#1470, #1472, #1475)

Added 2026-11-11. Hold several genuine requests in flight (for example, with browser network
latency emulation on an isolated lab) while walking these checks.

1. Select at least four eligible independent sessions and run Make top-level, Back from Later,
   Stop, Archive, Unarchive or Mark as seen. Press Done to clear the selection while requests are
   in flight: every row whose request is running shows a spinner on its row-actions control,
   queued and finished rows do not, and every row's controls stay disabled until the last request
   settles. Failures keep their per-target receipt.
2. Start a batch from one family (for example Back from Later) and immediately invoke another
   (Make top-level, Unarchive): the second starts no request while the first runs, so at most
   three requests are in flight. After the first batch settles, the second runs normally.
3. Start a single Nest under (drop) or Make top-level and, before it settles, invoke Make
   top-level on a selection: the batch starts no request until the single move settles. While a
   batch Make top-level runs, a row drop or single Make top-level is declined.

The lifecycle hook suite (`use-session-actions.test.tsx`) owns same-turn admission in every
family order, running-target progress transitions and the singular/batch lineage boundary; the
session list suite owns per-row progress and the row lock.
