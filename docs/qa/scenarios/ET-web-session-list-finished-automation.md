---
id: ET-web-session-list-finished-automation
area: ET
title: Automation runs stay out of the session list and the board but stay reachable
persona: Bruno
journey:
expected: A job or trigger session never appears on the list, the board, the dock launcher or the counts, in one project and in All projects, from the moment it is created through running, waiting, failing and finishing, and it never takes the selection. Pagination stays complete. It is not archived. The Automations run view, search, the session URL, `type=system`, `compozy session list` and `compozy__session_list` still reach it, and a run that needs the operator still raises its attention toast. Turning on "Show automation runs" in a window's View options lists every run with its counts, persists for that window across a reload and does not reach a new window. An operator-prompted run and its family stay visible. Loop and Task sessions never leave. Runs created before the upgrade follow the same rule with no backfill. `exclude_finished_automation` keeps hiding only settled runs.
entry_points: GET /api/sessions?exclude_automation=true; GET /api/sessions/facets?exclude_automation=true; GET /api/sessions?exclude_finished_automation=true; SessionList; SessionsBoard; SessionViewOptionsMenu; dock Sessions launcher; Automations run view; compozy session list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-list-aggregate-parity, ET-web-sessions-board-paging-archive
---

Introduced by #1162 for settled runs; widened to every unprompted run by #1355.

Owning automated coverage: `TestPageSessionsVisibilityExclusion` in
`global_db_session_index_test.go` owns both populations, totals, facets, All projects, paging and
the live-row lookup against real SQLite. `TestManagerListPageOverlaysActiveAndBindsCursor` owns the
live overlay. `TestNormalizeListQueryFinishedAutomationScope` owns which reads ignore the flags.
`always-on-signals.test.tsx` and `use-session-list-groups.test.tsx` own the filters each shelf and
the window toggle send; `session-list.test.tsx` and `use-session-sidebar-state.test.ts` own the
toggle and its persistence.

Walk: in an isolated lab with a mock agent, create a scheduled job that runs every minute and an
ordinary session. Watch the list while the run starts, works and finishes: no row appears, the
ordinary session keeps the selection, and the working count does not move. Compare the list, the
board and `/api/sessions/facets` with and without `exclude_automation`. Open the run from its job's
run history. Turn on "Show automation runs": the runs appear with their counts; reload and open a
new window. Prompt a run and check it stays. Click the dock's Sessions launcher right after a run
starts: it opens the ordinary session. Repeat under All projects, in light and dark.
