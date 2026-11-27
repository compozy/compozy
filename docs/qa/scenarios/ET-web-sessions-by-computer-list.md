---
id: ET-web-sessions-by-computer-list
area: ET
title: Browse and scope the Sessions List across computers
persona: Bruno
journey: J-operate-remote-computers
expected: The list shows each computer's named projects and exact counts in the selected layout and scope.
entry_points: Session window List and View options; computer picker; GET /c/{id}/api/sessions; GET /c/{id}/api/sessions/facets
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-sidebar-per-window; ET-web-session-folders-manual-order; RT-sessions-by-computer-states
---

Use this computer and two enabled remote computers. Register the same Git repository on two computers
with different project IDs. Give one remote computer a second project with a folder and two profiles.
Start with one empty remote computer and a new window.

1. Open List under **All computers**. Confirm **Separate** puts this computer first.
2. Under **Group by → Project**, open the remote section. Confirm project names appear instead of raw IDs.
3. Confirm the project with sessions in two profiles splits by profile.
4. Compare counts with the per-computer facets. Include sessions on a page you have yet to open.
5. Fold a computer section. Confirm the header keeps its count and gives the Needs you signal priority.
6. Confirm the empty connected computer shows "No sessions on {computer} yet." and an eligible **+**.
7. Choose **Together**. Confirm the shared repository shows distinct project groups with computer names.
8. Select **Group by → None**. Confirm the shared list shows a computer label on each remote row.
9. Focus the shared repository in the topbar. Confirm the list matches the remote project by repository.
10. Scope the window to the remote computer. Confirm the computer header and **Computers** choice disappear.
11. Open **New session**. Confirm **Run on** uses the scoped computer.
12. Reload. Confirm the scope persists. Confirm a new window starts at **All computers**.
13. Fail one remote catalog read. Confirm other computers remain usable and **Not counted** names the failed computer.
14. Search for a loaded offline row. Confirm the list identifies matches from before the computer went offline.
15. Select a session in the scoped window. Switch to Kanban, then to List.
16. Confirm the window keeps its computer scope and accepts changes to **Group by**.

Walk remote folder ownership in `ET-web-session-folders-manual-order`.
Walk disconnect and reconnect in `RT-sessions-by-computer-states`.
