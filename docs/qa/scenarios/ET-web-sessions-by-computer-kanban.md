---
id: ET-web-sessions-by-computer-kanban
area: ET
title: Follow Kanban counts and project lanes across computers
persona: Bruno
journey: J-operate-remote-computers
expected: The board shows bands, lanes, and exact column counts for the computers in scope.
entry_points: Session window Kanban and View options; computer picker; GET /c/{id}/api/sessions/facets
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-sessions-board-card-actions-group-sort; ET-web-sessions-board-paging-archive; RT-sessions-by-computer-states
---

Use a controller and two linked computers. Give one remote computer two projects with sessions in
different columns. Include more sessions in one column than the first page holds.

1. Open Kanban with **Separate** and **Group by → Project**. Confirm each computer has a band.
2. Confirm the remote band contains both project lanes. Open a card and check the drawer's computer.
3. Fold the remote band. Compare its five displayed counts with the remote `facets.board` values.
4. Confirm the folded band's Needs you signal remains visible when a session needs you.
5. Select **Group by → None**. Confirm each computer has its own set of columns.
6. Fail one remote facets read. Confirm the affected band shows an error and the other bands remain usable.
7. Resolve the failure. Select **Retry** and confirm the correct counts return.
8. Choose **Together** and **Group by → Project**. Confirm each lane displays its project and computer name.
9. Scope the window to a remote computer. Confirm the board shows only that computer and hides its band header.
10. Open **New session**. Confirm **Run on** uses the scoped computer.
