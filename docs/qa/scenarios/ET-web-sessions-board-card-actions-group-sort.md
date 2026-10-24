---
id: ET-web-sessions-board-card-actions-group-sort
area: ET
title: Act on Kanban cards and group or sort the board like the list, per window
persona: Bruno
journey:
expected: Right-clicking a Sessions Kanban card opens a context menu (no native Copy menu) with the session list row verbs (Rename, Copy session ID, Copy title, Continue, Fork, Stop, Archive/Unarchive, Delete as the session allows); Shift+F10 opens it from the keyboard. Hovering or focusing a card reveals a `…` button, reachable with Tab right after the card, that opens the same verbs; choosing a verb never opens the session, and Rename/Delete open their confirmations. The board head shows Group by and Sort by icons before "Hide child sessions". Group by offers None plus Project under All projects and Profile under All profiles (the control is absent when only None applies) and defaults to None. Grouped, the board renders one swimlane per group with members (project badge or profile tag, member count), each holding the four derived columns with that group's own counts; the Archive bar stays ungrouped. Sort by offers Created, Name, Last activity, Attention first and orders cards inside every column; a List-only Manual sort reads Last activity on the board. The session list toolbar shows Sort and Group by as two separate icons. Sort and Group by are per window and shared by that window's List and Kanban: a choice made in one view holds after the toggle and a reload, another window keeps its own, and a window that never chose starts from `shell.sessions.sort/group_by`, which the web never writes. The session window head has no per-session `…` menu; Clear conversation lives in the row menus of the window's own session.
entry_points: SessionBoardWindow head (sessions-board-group-by-trigger, sessions-board-sort-trigger); session-board-card context menu and session-row-actions-<id>; session list toolbar (session-sidebar-sort-trigger, session-sidebar-group-by-trigger); GET /api/sessions?board_column=<id>&sort=<sort>&workspace_id|profile=<group>; GET /api/sessions/facets; localStorage compozy:session:sidebar:v1 (`byScope.<window>.sort|groupBy`); compozy config get shell.sessions.*
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-sessions-board-toggle; RT-session-list-row-actions; ET-web-session-list-aggregate-parity; ET-web-session-sidebar-per-window
---

Introduced by issue #952.

Walk: under All projects with sessions in two projects, open a session window and switch to
Kanban. Right-click a card: the shared context menu opens; Escape closes it. Hover the card,
Tab from it to its `…`, press Enter, pick Rename: the rename dialog opens and the window stays on
the board. Pick Group by → Project: one swimlane per project, column counts matching each
project's `facets.board`. Pick Sort by → Name: each column reads `sort=name`. Toggle to List: the
list is grouped by project and sorted by name, with Sort and Group by as two icons. Reload:
both views keep the choice. Open a second session window: it starts from the daemon value, and
changing it leaves the first window unchanged. Switch to one project: Group by disappears and
the board renders ungrouped. In List mode, open the row menu of the window's own session: Clear
conversation is there (disabled while a turn runs); other rows do not offer it; the head has no `…`.

Owning automated coverage: `sessions-board.test.tsx` (card context menu, `…` without opening,
keyboard reach, swimlanes reading their own scope), `use-sessions.test.tsx` (board read model:
manual → last_activity, lanes from facets, inapplicable grouping renders flat),
`use-session-sidebar-state.test.ts` (per-window order shared across the toggle, independent
windows, daemon seed), `session-list.test.tsx` (split Sort/Group by, Clear conversation only on the
window's own row), `use-session-topbar-slot.test.tsx` (no session overflow menu).
