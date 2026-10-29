---
id: ET-web-sessions-board-toggle
area: ET
title: Switch a session window between List and Kanban and open a session from a card
persona: Bruno
journey:
expected: The session window identity zone shows a "List | Kanban" switch beside the title, in every session window and in a window with no session selected; no per-session "…" menu sits beside it (#952: those verbs live in the session list row menus). Selecting Kanban (click, or Tab to the switch, ArrowRight, Enter) turns the window body into four columns — Needs you, Working, In review, Done — whose header counts equal `facets.board` for the window's scope (project or All projects); exactly one mode stays selected. The board head reads "Sessions" with Group by, Sort by, "Hide child sessions" and "New session" (ET-web-sessions-board-card-actions-group-sort). Cards show three rows (meta + at most one status, title with an unread dot for finished-unseen sessions, agent + lineage + time). Clicking a card returns the window to List with that session open. A reload keeps each window's last view; a new session window always opens in List. Changing the scope reloads the board for the new scope.
entry_points: web session window topbar (session-view-toggle); SessionBoardWindow (sessions-board); GET /api/sessions?board_column=<id>; GET /api/sessions/facets; localStorage compozy:session:sidebar:v1 (`byScope.<window>.view`)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-sidebar-per-window; ET-web-sessions-board-paging-archive; ET-web-sessions-board-children; ET-web-sessions-board-card-actions-group-sort
---

Introduced by the Sessions Kanban spec (`.compozy/tasks/sessions-kanban`, US-011..US-013, E2E-005, visual contract VC-19..VC-29).

Walk: in a workspace with sessions in each column (one waiting for input, one running, one idle with an open PR, one done), open a session window. The switch sits beside the title with List selected. Tab to it, press ArrowRight and Enter: four columns render with counts matching `GET /api/sessions/facets` → `facets.board`. Click the In review card; the window shows that session in List. Switch to Kanban again and reload; the window is still on the board. Open a second session window; it opens in List. Switch the topbar scope to All projects; cards without a PR show their project name in row 1 and counts follow the aggregate.

Owning automated coverage: E2E-005 (`web/e2e`), `use-session-sidebar-state.test.ts` (UT-067: per-window view restored, never seeded into new windows), `use-session-topbar-slot.test.tsx` (switch present for every session type, no session overflow menu), `session-board-card.test.ts` (UT-068/UT-069 row-1 status and meta).

Regression walk: seed more than 50 stopped sessions in one project, with an open PR on the
oldest session's non-default branch and no cached PR facts. Open In review without loading
Done history. The asynchronous refresh must discover the old branch and move that session
into In review on the next catalog read. Repeat with another project/profile in the database;
its branches must not enter the selected scope's refresh batch. Facet-only reads must also
schedule discovery. Backend coverage: `TestManagerBoardCatalog` with real SQLite.

PR #973 restore regression: save one window in Kanban and another in List, then cold-reload with the lazy Session app chunk delayed. The Kanban window must show the board skeleton immediately, without a sidebar/transcript flash; the List window retains its saved rail layout.

QA impact 2026-10-28 (session intent): board cards share the session-row intent
policy. Hover/focus a cold card, verify one authorized transcript head before activation,
then select it and confirm the matching session without a duplicate head read. Brief
intent must cancel; profile/workspace changes must prevent the old target from warming
in the new scope. List/board switching retains its existing behavior. Focused suites and
the targeted real-daemon walk are recorded in
`docs/qa/reports/2026-10-28-tanstack-data-loading.md`.
