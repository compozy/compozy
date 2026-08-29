---
id: RT-session-delete-keeps-empty-tab
area: RT
title: Delete a focused session and keep the empty tab
persona: Théo
journey: J-15
expected: Deleting the focused session unmounts the transcript, removes the catalog row, keeps the same window and stack, retargets that tab to the empty session surface with the sidebar still available, and does not open the agent page.
entry_points: Web session window overflow delete; DELETE /api/workspaces/:wid/sessions/:sid
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/screenshots/delete-keeps-empty-tab.png; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/session-delete-empty-tab.png; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/logs/session-delete-api.txt; docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
last_report: docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
overlaps: RT-014
---

QA impact 2026-08-27: delete no longer destroys the session window. The operator lands on `/sessions`
inside the same frame so they can pick another row or create one.

QA execution 2026-08-27: overflow delete of Dock last created kept window
os-window-w-e5495f467601ddeb42549dcaed, moved the URL to /sessions, showed "No session selected"
with the sidebar, and did not open the agent page. Independent HTTP GET was 404. Verdict: pass.
