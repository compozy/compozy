---
id: ET-window-manager-multi-client
area: ET
title: Keep topology shared while desktop and focus stay client-local
persona: Bruno
journey: J-administer-window-manager
expected: Two registered clients using the same profile in one workspace observe the same persistent desktops, groups, windows, revisions, routes, and durable events while independently switching desktops and focusing or zooming windows; clients using different profiles observe separate desktops and windows, and switching one never changes the other's active view; a remote presentation command reaches exactly the selected client's fenced stream without advancing topology revision/history/hooks or leaking that ClientView to its peer; missing, foreign, and disconnected client IDs reject.
entry_points: two Web browser contexts in one profile; two Web browser contexts in different profiles; compozy desktop clients; compozy desktop switch; compozy window focus; compozy window zoom; profile-scoped window-manager reads and commands
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/supervisor-01-desktop-pager.png; docs/qa/evidence/2026-08-01-window-tabs/supervisor-02-recovery-desktop.png; /Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/evidence/two-client-conflict-recovery.txt; /Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/evidence/theo-triggers-after-conflict.png
last_report: docs/qa/reports/2026-08-20-window-management-regressions.md
overlaps: ET-profile-desktop-restoration; ET-window-manager-public-parity; ET-web-desktop-shell-lifecycle; RT-desktop-pager-overview
---

story: As a person running agent work using two screens, I can navigate and focus independently without either client fighting the other's presentation.

qa-impact: 2026-07-22 split daemon-owned topology from explicit `(workspace_id, client_id)` presentation state and added client-bound presentation stream fencing for remote control. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 added client-local `stack_active` over durable last-active tab state. Reset to
verify independent tab selection with shared topology.

qa-impact: 2026-08-20 focus and tab-activation commands gained window-identity rebase guards so a
stale client can converge after a competing topology write. Reset for a real two-client conflict
and recovery walk.
qa-impact: 2026-08-22 window arrangements moved from one document per workspace to one per
(workspace, profile), and every window-manager read and write now names the profile it acts as.
Reset to verify isolation, restoration on switch, and that a workspace still purges every profile's
desks when it is removed.

Profile boundary walk: repeat the shared-topology checks with two clients on one profile, then use
two different profiles in the same workspace and prove their desktops, windows, active views, and
revisions stay separate. The profile-desktop scenario owns switch restoration and workspace purge;
this row owns the client fencing and presentation-only command assertions.

2026-08-23 qa-impact (Profiles): multi-client topology is now scoped per profile — two clients share
topology only when they are on the same profile, and a switch retires and reclaims the client
registration atomically so overlapping switches serialize. Already `untested`, so no reset was
needed. Add one pair of clients on different profiles and confirm neither sees the other's windows
and neither is force-switched when its peer switches. Per-profile restoration itself is owned by
`ET-profile-desktop-restoration`.
