---
id: ET-window-manager-multi-client
area: ET
title: Keep topology shared while desktop and focus stay client-local
persona: Bruno
journey: J-administer-window-manager
expected: Two registered clients using the same profile in one workspace observe the same persistent desktops, tabs, splits, windows, revisions, routes, and durable events while each keeps its own active desktop, active tab and focus; activating or opening a tab in one client never changes the other's active tab, and a client whose active tab is closed falls back to the desktop default, then the first tab; clients using different profiles observe separate desktops and windows; a remote presentation command reaches exactly the selected client's fenced stream without advancing topology revision/history/hooks or leaking that ClientView to its peer; dismissing the upgrade notice in one client hides it in the other at the same revision; missing, foreign, and disconnected client IDs reject.
entry_points: two Web browser contexts in one profile; two Web browser contexts in different profiles; compozy desktop clients; compozy desktop switch; compozy window focus; compozy window activate --client; compozy layout migration ack|retry; profile-scoped window-manager reads and commands
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits: a1baedd3a
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-parity-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/21-peer-sees-zoom-lift.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-10-02-untested.md
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

qa-impact: 2026-09-01 `window.zoom` no longer requires a `client_id` and no longer changes topology
beyond the `zoomed` flag: two clients on one profile both see the zoomed frame, and a zoom from the CLI
without a client leaves every client's active desktop and focus untouched. Reset for the current build.

qa-impact: 2026-09-01 two browser clients: the zooming client follows a lifted zoom to its new desktop while the peer stays on its desktop and sees the pager grow; a clientless CLI unzoom returns the window and repairs only the lifted client's view. Walked P2.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). Presentation chrome changed (pager in the topbar tray, dock rail, flat tiles) and the theme is a per-browser preference: two clients may paint different themes. Reset to re-walk shared topology vs client-local desktop/focus/zoom and confirm a theme switch in one client never reaches the other.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 migration notice updates use the existing `snapshot` WebSocket frame.
With two clients in one workspace/profile, acknowledge a migration in one and verify
`snapshot.migration` disappears in both without changing topology revision, undo history,
or firing layout/window hooks. Retry a preserved layout that still fails and verify both
clients receive the updated error with `acknowledged: false` at the same revision. A successful
retry publishes the converted layout and its migration status at the next topology revision.
Clients in another workspace or profile must receive no notice update. TCP/Unix transport
coverage is written; live QA and test execution remain deferred to the integration owner.

qa-impact: 2026-11-13 window-tabs-split S10. Zoom is gone; the per-client state is now the active tab (`active_tab` replaces `stack_active`). The migration-notice paragraph above (2026-11-13) stays valid; in the UI a successful upgrade is the bell item "Your windows are now tabs", and reading it acknowledges for every client; only a failed one shows an inline notice, whose Dismiss acknowledges for every client. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Open the same workspace in two browsers A and B (same profile) with three tabs. In A activate tab 2, in B tab 3: each keeps its own. In A open a new tab: B's active tab does not change, B's bar gains the tab. In B close the tab A is showing: A falls back to the desktop default tab. Drag a split in A: B sees the same split at the same revision. `compozy window activate <id> --client <A-id>` changes only A. With a migrated workspace (see `ET-window-layout-upgrade-v06`), click the bell item in A: it leaves B's bell without reload and `compozy layout get` shows the same revision with no `migration`.
