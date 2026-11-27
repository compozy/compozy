---
id: RT-session-delete-keeps-empty-tab
area: RT
title: Delete a focused session and keep the empty tab
persona: Théo
journey: J-15
expected: Deleting the focused session removes its catalog row and keeps the same Sessions tab, split, and view. List shows No session selected and keeps the sidebar available. Kanban closes the deleted session's drawer and follows the same neighboring-card focus order as Archive.
entry_points: Web session window overflow delete; Sessions catalog single/bulk delete; agent session lists; session sidebar; DELETE /api/workspaces/:wid/sessions/:sid
qa_status: pending
bug_ids:
fix_status:
retest_status: pending
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

2026-10-21 User scope coverage:

Open one User session in `user`, `global`, and project desktop partitions across two profiles, reload the layouts, delete the session, and verify every window retires the session resource while unrelated windows remain intact. This extends the project-only deletion walk.

QA impact (#2009): Delete the selected session from List while another row is visible, then
repeat with the last session. The URL is `/sessions` in the same tab, with no session selected.
Repeat from a Kanban drawer with neighboring cards, then with no remaining cards. The board
stays open and focuses the same surviving card that Archive would, when one is available.
`session-hardening.spec.ts` covers the List landing and `session-board-window.test.tsx` covers
the drawer's focus behavior. A focused live re-walk remains pending.
