---
id: ET-web-window-routing-lifecycle
area: ET
title: Operate window lifecycle with URL and semantic topology parity
persona: Bruno
journey: J-operate-desktop-shell
expected: Dock, palette, pointer, and keyboard activation open or focus one window instance; drag, structural resize, zoom, minimize, restore, and close preserve return anchors and successor focus; the focused window owns the URL with one history write per user cause; task/detail/search route intent survives reload and daemon restart, layout undo does not rewind it, and browser, CLI, native tool, and peer-browser changes converge by revision.
entry_points: web desktop dock and windows; browser history; compozy window; compozy__window_manager
qa_status: skipped
bug_ids: BUG-20260830-terminal-retarget-duplicate-window; BUG-20260902-background-window-stream-starvation
fix_status: fixed
retest_status:
fix_commits: pending-remediation-batch
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/theo-routing4-home-restored.png; docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-desktop-shell-lifecycle; ET-window-manager-public-parity; ET-window-manager-layout-gestures; ET-web-route-chrome-topbar
---

Open Tasks from the dock and navigate to a task detail. Race a palette activation with restoration of that deep link and verify one window, one route owner, and one history entry. Minimize, restore, resize, close, undo layout, reload, and restart the daemon; task route intent and focus must survive each supported transition.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
