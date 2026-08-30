---
id: ET-web-window-routing-lifecycle
area: ET
title: Operate window lifecycle with URL and semantic topology parity
persona: Bruno
journey: J-operate-desktop-shell
expected: Dock, palette, pointer, and keyboard activation open or focus one window instance; drag, structural resize, zoom, minimize, restore, and close preserve return anchors and successor focus; the focused window owns the URL with one history write per user cause; task/detail/search route intent survives reload and daemon restart, layout undo does not rewind it, and browser, CLI, native tool, and peer-browser changes converge by revision.
entry_points: web desktop dock and windows; browser history; compozy window; compozy__window_manager
qa_status: pass
bug_ids: BUG-20260830-terminal-retarget-duplicate-window
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-10-local-stream-auth-clean/software-factory-web.png; docs/qa/evidence/2026-08-10-local-stream-auth-clean/software-factory-desktop.png; /Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/evidence/settings-knowledge-background-route.png; /Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/evidence/grouped-tabs-knowledge-active.png; docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
last_report: docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
overlaps: ET-web-desktop-shell-lifecycle; ET-window-manager-public-parity; ET-window-manager-layout-gestures; ET-web-route-chrome-topbar
---

story: As a builder, I can arrange persistent desktops and trust every surface to observe one semantic topology while my focus remains client-local.

qa-impact: 2026-08-30 a pending in-place retarget now projects its route and semantic instance as one
identity. Reset for the packaged Terminal first-create path that previously allowed route
reconciliation to open a duplicate window.

2026-08-30 re-walk: passed. Packaged Desktop E2E-013 created the first terminal, retained one
Terminal window, and completed input, clipboard, refit, and IME checks in three consecutive runs.

qa-impact: 2026-08-30 concurrent deep-link reconciliation and Dock activation now share one
in-flight semantic open, preventing duplicate windows while the first daemon command settles.
Reset for a focused Network deep-link and Dock handoff walk.

2026-08-30 re-walk: passed. The disabled Network deep link and concurrent Dock activation produced
one authority-backed Network window in five focused repetitions, including a 14.4-second loaded
reconciliation, and the Settings handoff reached `/settings/network` every time.

qa-impact: 2026-07-22 window-management hard cut replaced independent window documents with semantic commands, structural return anchors, durable route intent, and revisioned convergence; 2026-07-24 explicit `window.focus` and minimized-window restore now activate the window's desktop for the issuing client (dock activation follows a cross-desktop window instead of failing silently). Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 semantic app/instance identity, per-tab route stacks, and active-member routing
replaced singleton window lookup. Reset for tabbed routing and reload continuity.

qa-impact: 2026-08-20 rehosted Settings sections now read route intent from their owning window,
and Knowledge subscriptions preserve stable external-store snapshots. Reset for focused-route and
background-window rendering verification.
