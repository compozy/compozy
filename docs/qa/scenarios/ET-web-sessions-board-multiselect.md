---
id: ET-web-sessions-board-multiselect
area: ET
title: Multi-select Kanban cards and run the session list's bulk actions on the board
persona: Bruno
journey: J-24
expected: On the Sessions Kanban, Cmd/Ctrl-click toggles a card, Shift-click ranges inside the clicked card's column (an anchor in another column starts a new range), and while a selection is open every click, Enter or Space on a card toggles it instead of opening it. Hovering or focusing a card reveals a named checkbox in place of its provider mark; column headers (glyph) and swimlane headers carry a select-all over the cards they show, and clearing one removes only those cards. Cmd/Ctrl+A selects every shown card; Esc clears, also right after Escape closes a card menu. While cards are selected the session list's selection bar (select-all, "N selected · M hidden", Delete, More actions, Done) replaces the board head's actions without moving the board, and a polite status announces the count. More actions and a selected card's right-click show the "N sessions" menu with per-verb counts (Stop N active, Archive N stopped / Stop and archive N sessions, Unarchive N archived, Copy session IDs, Delete N sessions with confirmation, Move to folder when the selection sits in one project+profile pair, else its reason); a right-click outside the selection retargets it (Finder rule). Batch toasts name skipped sessions by reason. Selected cards survive a column move (stop moves Working → Done and keeps them); cards gone from every read (archived with the Archive bar closed, deleted elsewhere) leave the selection; folded Done keeps its selected cards as hidden; archived cards are selectable only while the Archive bar is open. List and Kanban keep separate selections; scope, profile lens or grouping changes reset it. With the session drawer (#1052): selection gestures never open the drawer, a plain click with nothing selected does, and a bulk archive or delete that includes the drawer's session closes it and lands focus on a card outside the batch.
entry_points: SessionBoardWindow (sessions-board-selection-bar, sessions-board-selection-more, sessions-board-selection-delete); session-board-card-<id> and session-board-card-select-<id>; sessions-board[-lane-<key>]-column-select-<column>; sessions-board-lane-<key>-select; sessions-board-selection-menu; sessions-board-selection-status
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits: 7addc5149
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/01-light-A-selection-bar.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/02-dark-A-selection-bar.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/03-light-B-context-menu.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/04-dark-B-context-menu.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/05-light-C-lanes-move.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/06-dark-C-lanes-move.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/07-light-01-hover-checkbox.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/08-light-05-done-folded-hidden.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/09-light-06-archive-selected.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/10-light-07-retarget-own-menu.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/11-light-09-after-stop-moved-kept.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1053-kanban-multiselect/7addc5149/12-light-10-archive-toast.png
last_report:
overlaps: ET-web-sessions-board-drawer; ET-web-session-list-bulk-actions; ET-web-sessions-board-card-actions-group-sort; ET-web-sessions-board-paging-archive
---

Introduced by issue #1053.

Walk (2026-10-30, isolated lab on the branch daemon with `demo-seed` plus three acpmock
sessions held running, All projects, light and dark): switch a session window to Kanban.
Hover a card: its checkbox replaces the provider mark. Cmd-click a Working card: the selection
bar replaces View options / New session and the board does not move.
Click a Done card, Shift-click two cards further down Done, click the Needs you card: the bar
reads 6 selected and each column header shows all/some/none. Right-click a selected card: the
"6 sessions" menu shows Stop 1 active, Stop and archive 6 sessions, Unarchive 0 archived.
Fold Done: "6 selected · 4 hidden". Open the Archive bar and click an archived card: 7 selected.
Right-click a card outside the selection: it becomes the only selection and its own menu opens;
Escape closes the menu onto the card and a second Escape clears the selection. Keyboard only:
Tab from a card to its checkbox, Space toggles, Shift+Space ranges, Enter on a card toggles
while selecting, Cmd+A selects every shown card. Select two running cards and one stopped
card, More actions → Stop: the running two move to Done and stay selected (toast reports
the stopped one as skipped). More actions → Archive: all three leave the columns, the Archive
count grows by three, and the selection empties. Group by → Project: tick a lane header to
select the whole lane; More actions → Move to folder lists the lane pair's folders and New
folder….

Owning automated coverage: `sessions-board.test.tsx` ("SessionsBoard selection": gestures,
column/lane select-all, Esc, context menu inside/outside the selection and Escape focus return,
bar verbs and delete confirmation, column move kept, vanished card pruned, folded Done hidden,
Archive bar join/drop, per-card re-render isolation), `use-session-actions.test.tsx`
(batch skipped reasons), `session-list.test.tsx` (list selection unchanged), E2E-008 in
`web/e2e/__tests__/sessions-board.spec.ts` (multi-select + Stop and archive against the daemon).
