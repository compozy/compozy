---
id: RT-desktop-pager-overview
area: RT
title: Navigate persistent desktops through the minimal pager
persona: Théo
journey:
expected: A lower-left horizontal carousel-dot pager shares the Dock centerline and shows one ordered dot per persistent desktop without colliding with the Dock; current and focus desktops are distinguishable without decorative color semantics; click, keyboard, and swipe switch only the active client; 1, 2, and 7 desktops remain direct and 8+ use an accessible overflow treatment; the on-demand overview creates, renames, reorders, transfers, and deletes desktops with honest pending, conflict, empty, and disconnected states.
entry_points: web desktop pager; Desktops Overview; keyboard and touch navigation
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-multi-client; ET-web-desktop-shell-lifecycle
---

story: As an operator, I can move between many desktops from a quiet control and open full management only when I need it.

scope: Verify screen-reader naming and position, 44px target equivalence, visible focus, contrast, reduced motion, portrait/landscape placement, 1/2/7/8+ counts, and no application remount during a switch.

qa-impact: 2026-07-22 replaced the workspace-card Spaces overlay with a persistent-desktop dot pager and on-demand management; 2026-07-24 desktop switch transitions moved from CSS transitions to keyframes at `--duration-shell-base` (was shell-fast), covering reconciled switches as well. Flag only; the next QA cycle owns live retesting.
