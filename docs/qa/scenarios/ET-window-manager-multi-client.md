---
id: ET-window-manager-multi-client
area: ET
title: Keep topology shared while desktop and focus stay client-local
persona: Bruno
journey:
expected: Two registered clients in one workspace observe the same persistent desktops, groups, windows, revisions, routes, and durable events while independently switching desktops and focusing or zooming windows; a remote presentation command reaches exactly the selected client's fenced stream without advancing topology revision/history/hooks or leaking that ClientView to its peer; missing, foreign, and disconnected client IDs reject.
entry_points: two Web browser contexts; agh desktop clients; agh desktop switch; agh window focus; agh window zoom
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-public-parity; ET-web-desktop-shell-lifecycle; RT-desktop-pager-overview
---

story: As an operator using two screens, I can navigate and focus independently without either client fighting the other's presentation.

qa-impact: 2026-07-22 split daemon-owned topology from explicit `(workspace_id, client_id)` presentation state and added client-bound presentation stream fencing for remote control. Flag only; the next QA cycle owns live retesting.
