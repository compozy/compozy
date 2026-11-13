---
id: ET-window-tab-deck-lifecycle
area: ET
title: Open, activate, reorder and close windows as tabs in the desktop tab bar
persona: Bruno
journey: J-organize-tabbed-work
expected: Each desktop shows one tab bar (a tablist named "Tabs") listing every tab; opening an app or pressing ⌘T adds a tab right after the active tab and activates it; a tab shows its lead window's icon and title, a split tab up to three icon tiles and a member count; clicking a tab activates it, middle-click closes it, the bar is one keyboard stop where Left/Right move between tabs and Home/End jump to the first and last, ⌃Tab/⌃⇧Tab cycle, ⌘1–⌘8 jump and ⌘9 goes to the last tab; dragging a tab along the bar or using the tab menu's Move ▸ Left/Right reorders it within its pinned or unpinned range; the pending-count pill and tooltip trail stay on the tab; a desktop always keeps at least one tab.
entry_points: desktop tab bar (pointer, keyboard, middle-click); ⌘T; ⌃Tab; ⌘1–9; tab context menu Move ▸ Left/Right; window.tab.move_left|move_right; compozy window open|activate|reorder
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/window-merge-preview.png; docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/window-three-tab-deck.png
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-window-manager-layout-gestures; ET-web-desktop-shell-lifecycle
---

> The notes and walks below up to the 2026-11-13 qa-impact note describe decks, floating windows, snapping or zoom and are history only; walk the 2026-11-13 note.

Derived from J-organize-tabbed-work steps 1-2. Covers functional grouping, interruption recovery,
pointer/keyboard consistency, reduced-motion presentation, and reload continuity. Safety hot spots:
TechSpec invariants 4 and 9.

qa-impact: 2026-07-31 drag-UX fix pass — solo frames now advertise their head as a
`window.stack.group` merge target while another frame drags over them (occluded heads never
advertise; the accent "Group as tabs" band owns the affordance and suppresses snap previews), and
the deck row aligns traffic lights, tab labels, and the new-tab control on the tab band's
centerline per the VC-01 reference. Fix-verification evidence: head-drop grouping smoke vs live
daemon in lab compozy-window-drag-ux-smoke-20260731-151337-798977-lab/qa-artifacts/qa/evidence.
Flag only; this cycle owns live retesting.

qa-impact: 2026-07-31 deck-drag semantics pass — only a direct tab drag acts on that tab in
isolation (reorder in the row, tear-out past the exit slop); dragging the deck bar's empty gutter
moves the whole frame and never detaches the active tab, and dropping a dragged window on another
window's body no longer folds it into the deck — the occupied center swaps the two units instead
(the deck row and the solo head stay the only merge zones). Flag only; this cycle owns live
retesting.

qa-impact: 2026-08-03 PR 291 remediation — canonical Command shortcuts now resolve to Command on
Apple platforms and Control elsewhere, and window-head drag automation uses the stable empty head
surface while route identity loads. Reset for a fresh keyboard, drag, reload, and sole-pinned-tab
walk from the current build.

qa-completion: 2026-08-03 isolated retest — Command-T opened the destination picker, selecting
Settings produced one Tasks + General tab deck, and a full reload retained that deck. A real pointer
drag moved the Home window through its stable head surface. The current Web E2E run independently
passed the grouping-preview, grouping-commit, persisted-deck, sole-pinned-survivor, and continuous
drag performance contracts, including every shell case that had failed on PR CI.

qa-impact: 2026-08-03 PR 291 CI remediation — the fixed primary-modifier navigation pop now uses
Command on Apple platforms and Control elsewhere. Reset for a fresh non-Apple Control-[ route-pop
walk from the current build; Command-T and reload remain adjacent deck canaries.

qa-completion: 2026-08-03 PR 291 CI remediation — Bruno created `Draft release notes`, opened its
detail, returned through the public window-stack Back action, and confirmed the list after reload.
Command-T created a Tasks + General deck that survived reload. The browser driver could not encode
BracketLeft with a modifier, so the exact non-Apple Control-[ input is proven by the Linux-platform
unit regression and E2E-032; the browser walk independently proved the same durable route-pop path.

qa-impact: 2026-08-06 review remediation — animation-frame callbacks and merge-target frame reads
now refresh during the layout phase so a pointer release cannot group against the previous committed
frame. Reset for a fresh drag-grouping walk and reload canary from the current build.

qa-completion: 2026-08-06 targeted retest — Bruno moved Settings, observed the live "Group as
tabs" affordance over the Tasks head, released into one Tasks + Settings frame, and confirmed the
same deck after reload. At 1280×800 with reduced motion, repeated Tasks dock activation cycled the
existing deck without creating another frame.

qa-impact: 2026-08-10 loop-convergence browser closeout — merge-target registration now stays hot
through an active gesture without clearing a deck-owned insertion target. Reset for a fresh
drag-to-deck walk, cancellation probe, commit, and reload canary from the current build.

qa-completion: 2026-08-10 isolated retest — Bruno grouped Home and Tasks, dragged Marketplace into
the live deck target, cancelled one preview with Escape, committed the next preview, and reloaded.
The ordered Tasks, Skills, and Home tabs and the active Skills member persisted without a stuck
merge target or duplicate frame.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). The deck became browser tabs on a recessed strip; the traffic lights moved to quiet trailing controls. Reset for a grouping, reorder, tear-out and reload walk in both themes.

qa-impact: 2026-09-30 shell-rail polish P6 — with the new default `new_window_policy = tab`, opening an app from the rail, palette, or menubar while a window is focused adds it to that window's deck as the active tab; a deck therefore also forms without any drag or Command-T. Walk one rail-opened tab alongside the grouping, reorder, tear-out and reload checks.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 window-tabs-split S10. Decks, "Group as tabs" and per-pane tab strips are gone; this scenario now owns tab bar basics. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Open Sessions, Tasks and Terminal from the dock: each appears as a tab right after the active one and becomes active. ⌘T opens a New Tab. Click each tab; Tab into the bar and use ←/→ to move focus, Enter to activate. ⌃Tab and ⌃⇧Tab cycle; ⌘2 jumps to the second tab; ⌘9 to the last. Middle-click a tab: it closes. Drag the last tab to first place; then right-click › Move right. Split two tabs (drag one into the content): the remaining tab shows two icon tiles and the member count "2". Start a session that waits for input: the tab shows its waiting count. `compozy window list` shows TAB, TAB INDEX and MEMBERS matching the bar.

qa-impact: 2026-11-13 window-tabs-split S10 (review fixes). The tab bar now uses roving focus. Walk: Tab into the bar lands on the active tab; Left, Right, Home and End move focus; Tab again leaves the bar.

qa-impact: 2026-11-13 issue #1659 (tab menu grouping). The tab menu now reads Pin tab · Move ▸ (Left, Right, To desktop) · Split with ▸ · Arrange ▸ · Close tab · Close ▸ (Other tabs, Tabs to the right, Tabs to the left, All tabs) · Reopen closed tab; the "right-click › Move right" step of the walk above is now right-click › Move › Right. Walk: right-click a middle tab: the top level lists exactly those items in that order. Move › Left and Move › Right each take it one place. On the first unpinned tab Move offers no Left; on a lone tab with one desktop the Move, Split with and Arrange submenus are absent, not empty. After choosing an item from a submenu by keyboard, focus is back on the tab the menu came from.
