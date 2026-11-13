---
id: ET-window-tab-multi-instance
area: ET
title: Cycle independent app instances without identity collisions
persona: Bruno
journey: J-organize-tabbed-work
expected: Opening multiple Tasks and Session instances assigns opaque identities; dock and context-menu destinations enumerate every live instance; repeated dock activation cycles in MRU order across desktops and activates the instance's tab; ⌥-click opens a new instance split beside the focused window, ⇧-click opens it on a new desktop, and the dock menu offers Open in new tab, Open in split, Open in new desktop and Go to tab; closing one instance never redirects or mutates another.
entry_points: dock click, ⌥-click, ⇧-click; dock context menu; Go to tab
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/keyboard-02-command-t-deck.png; docs/qa/evidence/2026-08-01-window-tabs/keyboard-03-palette-tabs.png
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-window-routing-lifecycle; ET-web-dock-default-window-size; ET-window-manager-multi-client
---

Derived from J-organize-tabbed-work step 1. Covers semantic identity, minimized and cross-desktop
edges, MRU continuity, and the adjacent dock/default-size regression surface.

qa-impact: 2026-09-30 shell-rail polish P6 — the rail gains explicit new-instance gestures and the
dock app menu is rebuilt. Walk, with Tasks already open and focused: plain click focuses the
existing Tasks window (repeat cycles MRU); Option (⌥)-click opens a second Tasks window split beside
the focused one; Shift (⇧)-click creates a new desktop, switches to it, and fills it with a third
Tasks window; right-click shows Open in new tab (joins the focused frame as a tab), Open in split
(⌥ click hint), Open in new window (a floating window at the default rect), Open in new desktop
(⇧ click hint), a separator, and Go to tab; Enter or Space on a focused rail icon acts like a plain
click even with Shift or Option held. Every new instance is independent: closing one never
redirects another, and reload keeps each placement.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 window-tabs-split S10. Minimized instances and "Open in new window" (floating) no longer exist. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. With Tasks open and focused, click Tasks in the dock: the existing window stays focused; repeat with two instances to cycle them in MRU order, switching desktop when needed. ⌥-click Tasks: a second Tasks window splits beside the focused one in the same tab. ⇧-click: a new desktop opens with a third Tasks window as its only tab. Right-click Tasks: Open in new tab, Open in split, Open in new desktop, then Go to tab listing the three instances; no Open in new window. Close one instance: the others keep their routes.
