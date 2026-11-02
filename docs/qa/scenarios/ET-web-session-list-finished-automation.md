---
id: ET-web-session-list-finished-automation
area: ET
title: Finished automation runs leave the session list and the board but stay reachable
persona: Bruno
journey:
expected: A job or trigger session appears on the list and the board while it runs or needs the operator. After a clean finish it leaves the list, every board column and the counts, in one project and in All projects, and pagination stays complete. It is not archived. Search, the Automations run view, the session URL, `type=system`, `compozy session list` and `compozy__session_list` still reach it. A failed run, a pending interaction, an operator-prompted run and a family with a failed, waiting or running child stay visible with their nesting. Opening a run does not keep it. Loop and Task sessions never leave. Runs created before the upgrade follow the same rule with no backfill.
entry_points: GET /api/sessions?exclude_finished_automation=true; GET /api/sessions/facets?exclude_finished_automation=true; SessionList; SessionsBoard; Automations run view; compozy session list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-list-aggregate-parity, ET-web-sessions-board-paging-archive
---

Introduced by #1162.

Owning automated coverage: `TestPageSessionsVisibilityExclusion` in
`global_db_session_index_test.go` owns the population, totals, facets, board, All projects and
paging against real SQLite. `TestNormalizeListQueryFinishedAutomationScope` owns which reads ignore
the flag. `stays-on-signals.test.tsx` owns the shelves that send it.

Walk: in an isolated lab with a mock agent, trigger a job built on the previous release, upgrade the
binary on the same home, then trigger one succeeding and one failing job. Compare the list, the
board and `/api/sessions/facets` with and without the flag: finished runs are gone, the failure
stays. Search the job name and open the session from its run. Prompt a finished session, stop it,
and check it stays. Repeat under All projects, in light and dark.
