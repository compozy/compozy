---
id: RT-sessions-by-computer-states
area: RT
title: Recover the Sessions window through remote computer state changes
persona: Iris
journey: J-operate-remote-computers
expected: Each remote computer's session controls and counts reflect its connection, plan, and protocol state.
entry_points: Settings → Computers; Session window List and Kanban; controller computer status and sessions APIs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-computers-operator; ET-web-sessions-by-computer-list; ET-web-sessions-by-computer-kanban
---

Use `eng-qa-bootstrap` for the controller and real remote daemon. Use a unique home, port, and socket
for each daemon. Use test account and plan fixtures for the plan change. Use a remote fixture with a
non-overlapping protocol range for the incompatible case.

1. Connect the remote computer. Confirm **Connecting…** shows without a count while the read is pending.
2. Confirm its section shows sessions and exact counts once connected. Load at least one row.
3. Disconnect it. Confirm loaded rows are dimmed and read-only. Confirm the section offers **Reconnect**.
4. Scope the window to the offline computer. Confirm the scope stays selected and the reconnect notice remains.
5. Reopen the app while the remote is offline. Confirm it shows the offline notice without cached rows or a count.
6. Restore the connection. Confirm **Reconnect** restores the rows, counts, and permitted actions.
7. Change the test plan so one connected computer needs Pro. Confirm it shows **Needs Pro** and **Upgrade**.
8. Connect to the incompatible fixture. Confirm the section shows **Update needed** and **Details**.
9. Confirm both ineligible computers omit rows and creation controls. Confirm the footer names uncounted computers.
10. Remove or disable the scoped computer. Confirm the window returns to **All computers** and shows the notice once.
11. Sign out with a remote computer in scope. Confirm the remote data and persisted scope clear.
