---
id: RT-session-delete-keeps-empty-tab
area: RT
title: Delete a focused session and keep the empty tab
persona: Théo
journey: J-15
expected: Deleting the focused session unmounts the transcript, removes the catalog row, keeps the same window and stack, retargets that tab to the empty session surface with the sidebar still available, and does not open the agent page.
entry_points: Web session window overflow delete; Sessions catalog single/bulk delete; agent session lists; session sidebar; DELETE /api/workspaces/:wid/sessions/:sid
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/screenshots/delete-keeps-empty-tab.png; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/session-delete-empty-tab.png; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/logs/session-delete-api.txt; docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
last_report: docs/qa/reports/2026-10-19-issue-812-session-delete.md
overlaps: RT-014
---

QA impact 2026-08-27: delete no longer destroys the session window. The operator lands on `/sessions`
inside the same frame so they can pick another row or create one.

QA execution 2026-08-27: overflow delete of Dock last created kept window
os-window-w-e5495f467601ddeb42549dcaed, moved the URL to /sessions, showed "No session selected"
with the sidebar, and did not open the agent page. Independent HTTP GET was 404. Verdict: pass.


## Issue #812 regression walk

1. Open one or more session windows, including a retained background tab.
2. From **Session → Sessions**, delete an open session through its row menu. The same window becomes **No session selected**, with no missing-session notice or toast.
3. Create/open sessions again, select all in the catalog, and delete the selection. Every deleted binding retires; surviving sessions and other focused windows retain their routes.
4. Repeat single and bulk deletion in **All profiles**. The outcome is the same as a single-profile view.
5. Refuse a deletion (for example, a failed request): keep its binding and confirmation available for retry. Successful members of a partial batch retire independently.
6. Open a valid foreign-profile deep link from a scoped profile. Preserve the owner banner and switch action. A network/owner lookup failure must remain an error rather than being treated as deletion.

2026-10-19: real isolated daemon + branch Vite walk passed all four list-deletion combinations. Public detail reads returned 404 and window-manager snapshots cleared the bindings. The foreign-profile canary retained its banner (scoped detail 404, aggregate detail 200). Regression suites cover failed deletion, remote-lookup errors, background focus, and the SSE/operator deletion race. Evidence and limits: [run report](../reports/2026-10-19-issue-812-session-delete.md).
