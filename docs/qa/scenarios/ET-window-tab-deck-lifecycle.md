---
id: ET-window-tab-deck-lifecycle
area: ET
title: Group related work into one persistent tab deck
persona: Bruno
journey: J-organize-tabbed-work
expected: Drag, context-menu, and Command-T grouping each create one ordered frame; dragging a window over a solo window's head shows the accent "Group as tabs" affordance and release folds both into one deck; the deck mounts only at two or more members with traffic lights, tab labels, and the new-tab control on one shared centerline, keeps hidden bodies mounted, supports reorder and tear-out, and survives reload with the same active member and placement.
entry_points: web desktop window drag; tab and dock context menus; Command-T; web /new-tab
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-06-review-remediation/bruno-grouped-deck.png; docs/qa/evidence/2026-08-06-review-remediation/bruno-grouped-deck-reload.png
last_report: docs/qa/reports/2026-08-06-review-remediation.md
overlaps: ET-window-manager-layout-gestures; ET-web-desktop-shell-lifecycle
---

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
