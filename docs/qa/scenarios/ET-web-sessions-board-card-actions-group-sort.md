---
id: ET-web-sessions-board-card-actions-group-sort
area: ET
title: Act on Kanban cards and group or sort the board like the list, per window
persona: Bruno
journey:
expected: Right-clicking a Sessions Kanban card opens a context menu (no native Copy menu) with the session list row verbs (Rename, Copy session ID, Copy title, Continue, Fork, Stop, Archive/Unarchive, Delete as the session allows); Shift+F10 opens it from the keyboard. Hovering or focusing a card reveals a `…` button, reachable with Tab right after the card, that opens the same verbs; choosing a verb never opens the session, and Rename/Delete open their confirmations. The board head shows one View options button; its menu holds Sort by, Group by and, on the board only, a Filter section with "Hide child sessions". Group by offers None plus Project under All projects and Profile under All profiles (the control is absent when only None applies) and defaults to None. Grouped, the board renders one swimlane per group with members (project badge or profile tag, member count), each holding the four derived columns with that group's own counts; the Archive bar stays ungrouped. Sort by offers Created, Name, Last activity, Attention first and orders cards inside every column; a List-only Manual sort reads Last activity on the board. The session list toolbar shows the same View options menu. Sort and Group by are per window and shared by that window's List and Kanban: a choice made in one view holds after the toggle and a reload, another window keeps its own, and a window that never chose starts from `shell.sessions.sort/group_by`, which the web never writes. The session window head has no per-session `…` menu; Clear conversation lives in the row menus of the window's own session.
entry_points: SessionBoardWindow head (sessions-board-view-options-trigger); session-board-card context menu and session-row-actions-<id>; session list toolbar (session-sidebar-view-options-trigger); GET /api/sessions?board_column=<id>&sort=<sort>&workspace_id|profile=<group>; GET /api/sessions/facets; localStorage compozy:session:sidebar:v1 (`byScope.<window>.sort|groupBy`); compozy config get shell.sessions.*
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/04-kanban-header-group-sort.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/05-kanban-card-dots-menu.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/06-kanban-card-context-menu.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/07-kanban-grouped-by-project.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/08-kanban-grouped-by-profile.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/09-list-keeps-window-group-and-sort.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-952-kanban/e16bcd082f99/03b-clear-conversation-confirm.png
last_report: docs/qa/reports/2026-10-24-issue-952-kanban-actions-group-sort.md
overlaps: ET-web-sessions-board-toggle; RT-session-list-row-actions; ET-web-session-list-aggregate-parity; ET-web-session-sidebar-per-window
---

Introduced by issue #952.

Walk: under All projects with sessions in two projects, open a session window and switch to
Kanban. Right-click a card: the shared context menu opens; Escape closes it. Hover the card,
Tab from it to its `…`, press Enter, pick Rename: the rename dialog opens and the window stays on
the board. Pick Group by → Project: one swimlane per project, column counts matching each
project's `facets.board`. Pick Sort by → Name: each column reads `sort=name`. Toggle to List: the
list is grouped by project and sorted by name, with Sort by and Group by in one View options menu. Reload:
both views keep the choice. Open a second session window: it starts from the daemon value, and
changing it leaves the first window unchanged. Switch to one project: Group by disappears and
the board renders ungrouped. In List mode, open the row menu of the window's own session: Clear
conversation is there (disabled while a turn runs); other rows do not offer it; the head has no `…`.

Owning automated coverage: `sessions-board.test.tsx` (card context menu, `…` without opening,
keyboard reach, swimlanes reading their own scope), `use-sessions.test.tsx` (board read model:
manual → last_activity, lanes from facets, inapplicable grouping renders flat),
`use-session-sidebar-state.test.ts` (per-window order shared across the toggle, independent
windows, daemon seed), `session-list.test.tsx` (one View options menu, Clear conversation only on the
window's own row), `use-session-topbar-slot.test.tsx` (no session overflow menu).

QA impact 2026-11-01 (issue #1132): one View options menu replaces the Sort and Group by icons in the
List and Kanban. Walk: grouped by project under All projects, check Show empty projects in either view
and see the empty project in both ("No sessions in {project} yet." on the board, which reads no columns
for it); fold a swimlane and see the same project folded in List, and the reverse. Collapse all in List
folds folders, project groups, child session lists and subagent lists, then the item reads Expand all;
Expand all opens them again, opening subagents only on rows with no child sessions. On the board the
action covers swimlanes and leaves column folds alone. Open one group after Collapse all, reload: the
choice and that exception hold, and a second window is unchanged. The Later and Archive shelves keep working.

QA impact 2026-11-01 (PR #1133): "Hide child sessions" moved from the board head into View options as a checkbox in a Filter section. The board head no longer shows it, and the List menu has no Filter section. The trigger is marked while child sessions show. The choice still survives a reload per window.
