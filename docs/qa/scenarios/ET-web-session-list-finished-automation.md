---
id: ET-web-session-list-finished-automation
area: ET
title: Automation runs stay out of the session list and the board but stay reachable
persona: Bruno
journey:
expected: A job or trigger session never appears on the list, the board, the dock launcher or the counts, in one project and in All projects, from the moment it is created through running, waiting, failing and finishing, and it never takes the selection. Pagination stays complete. It is not archived. The Automations run view, search, the session URL, `type=system`, `compozy session list` and `compozy__session_list` still reach it, and a run that needs the operator still raises its attention toast. Turning on "Show automation runs" in a window's View options lists every run with its counts, persists for that window across a reload and does not reach a new window. An operator-prompted run and its family stay visible. Loop and Task sessions never leave. Runs created before the upgrade follow the same rule with no backfill. `exclude_finished_automation` keeps hiding only settled runs.
entry_points: GET /api/sessions?exclude_automation=true; GET /api/sessions/facets?exclude_automation=true; GET /api/sessions?exclude_finished_automation=true; SessionList; SessionsBoard; SessionViewOptionsMenu; dock Sessions launcher; Automations run view; compozy session list
qa_status: passed
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/hide-automation-sessions-1355/214246d1f8a1/
last_report: 2026-11-07 #1355 lab walk
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

Walk 2026-11-07 (#1355): isolated lab daemon and `web/dist` built from the branch, acpmock-backed
`general` agent, one ordinary session and an `every:1m` workspace job. The scheduled run fired and
worked for 45 seconds. Polling every 2 seconds, `/api/sessions` and `/api/sessions/facets` with
`exclude_automation=true` never listed the run and kept `working=0, idle=1`; without the flag they
listed it with `working=1`. In the browser, the dock's Sessions launcher opened the ordinary session
while the run was the newest session, the list and footer showed only "My own work · 1 idle", "Show
automation runs" in View options added the run with "1 working · 1 idle" and marked the trigger, and
the finished run stayed hidden. The job's run history linked the run's session, and its URL opened
it without adding a row. `compozy session list` still returned both sessions. Dark theme only; the
board, All projects with several projects, a prompted run and a reload were not walked here and rest
on their automated suites.

QA impact 2026-11-08 (#1388): a run shown with "Show automation runs" is titled by its job (for
example `issue-triage`), not "New session", even when the job and its agent share a name; an
always-on agent's untitled main and an untitled child another agent spawned show their agent; an
operator's untitled session still reads "New session". Each row's compact time keeps counting while the list stays open (it used to freeze at
the value it first rendered). Owners: `session-list.test.tsx` ("SessionList row title") and the
`Time` tick tests in `packages/ui`, which now run under the React Compiler. Verified in the
compiled `SessionSidebar/Origins` story with a Playwright clock advanced two hours: before,
`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1388/102cc19b3/00-before-t2h.png` (three "New session" rows, times stuck at 3m/18m/42m); after,
`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1388/102cc19b3/02-origins-t2h.png` (`issue-triage`, `log-triage`, "New session", all at 2h). Not walked against
a lab daemon.
