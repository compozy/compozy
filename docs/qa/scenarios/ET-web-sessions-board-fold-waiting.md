---
id: ET-web-sessions-board-fold-waiting
area: ET
title: Board columns fold into rails and waiting sessions get their own column
persona: Bruno
journey:
expected: The board shows five columns in the order Needs you · Working · Waiting · In review · Done. A session idle on a registered wake source (an active monitor or timer, a subagent, a reply watch) sits in Waiting with its chip naming the source ("Monitoring", "Waiting"), even when its branch has an open pull request; Working holds only running, hung or unhealthy sessions. An empty Waiting column reads "Nothing waiting on a wake". Every column header has a Collapse button (`aria-expanded="true"`); pressing it folds the column into a 44 px rail showing the column mark, its vertical name and its count, and moves focus to the rail (`aria-expanded="false"`, name "Expand <column>"). Activating the rail (click, or Enter/Space) expands the column and moves focus back to its Collapse button. A fold applies to that column in every Group-by swimlane and survives a reload of the same window; a new session window opens with every column expanded. `compozy session list --column waiting`, `GET /api/sessions?board_column=waiting` and `facets.board.waiting` agree with the column.
entry_points: SessionsBoardColumn (sessions-board-column-toggle-<column>, sessions-board-column-empty-waiting); GET /api/sessions?board_column=waiting; GET /api/sessions/facets (facets.board.waiting); compozy session list --column waiting; compozy__session_list board_column ["waiting"]
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-sessions-board-toggle, ET-web-sessions-board-paging-archive
---

Introduced by #1075 (collapsible columns and a dedicated Waiting column).

Owning automated coverage: `TestBoardColumnFor` (waiting with and without an open PR),
`TestManagerBoardCatalog` (a monitoring child with an open PR is counted and filtered as
`waiting`), the board-column SQL/Go equivalence suite in `global_db_session_index_test.go`,
`sessions-board.test.tsx` (rail, focus, empty state, one fold across lanes) and
`use-session-sidebar-state.test.ts` (folds persist per window across a reload).

Walk: in an isolated lab, start one session that registers a monitor (`compozy__monitor_create`
timer) and ends its turn, one running session, and one idle session with an open PR. Open the
board: the monitor session is in Waiting with "Monitoring"; the running one is in Working; the PR
one is in In review. Compare the counts with `GET /api/sessions/facets` → `facets.board`, and
`compozy session list --column waiting -o json` with the Waiting cards. Fold Done and Needs you;
switch Group by to Project and check both stay folded in every lane; reload and check the folds
remain; open a second session window and check it starts expanded. Repeat in light and dark.
