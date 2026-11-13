---
id: RT-desktop-pager-overview
area: RT
title: Navigate and manage persistent desktops from the top-bar indicators and the overview
persona: Théo
journey: J-administer-window-manager
expected: The desktop indicators sit in the topbar tray before All desktops: one 6px dot per persistent desktop in order, the active desktop drawn as an 18px foreground pill, and an amber (`attn`) dot on an off-screen desktop that needs you, each named "Desktop N of T: {name}" ("— needs you" appended when it does); click and Left/Right/Home/End switch only the active client and `desktop.switch.1–9`, next and previous keep working; 1, 2, and 7 desktops stay direct and 8+ window ±2 behind "Show N earlier/later desktops" controls that open the overview at the hidden desktops; right-click or Shift+F10/the menu key on a dot opens Rename, Move left, Move right, Close desktop, New desktop, with Move left absent on the first desktop, Move right on the last and Close desktop on the last remaining one; Rename and Close desktop open the overview on that card (rename field focused, or the close confirm open with "Move N tabs to" and the check disabled until a destination is chosen); while a tab is dragged every dot grows into a dashed target "Move tab to {name}" (the source desktop dimmed, the one under the pointer solid volt) and releasing moves the whole tab, split included, to the end of that desktop's bar; resting the dragged tab on an overflow control (shown as "+N") opens the overview as a drop surface whose cards take the drop; All desktops opens the Desktops overview over the desk — the title "Desktops", one card per desktop with its layout thumbnail, name, tab count, switch chord and first four tabs with "+N more", the active desktop ringed, New desktop and Done/Esc — where a tab row jumps to its tab, a card drags to reorder, a tab row dragged onto another card moves it, the card menu offers Move left, Move right and Move a tab ▸, and the bin closes a desktop in place after choosing a destination for its tabs; a desktop whose last tab leaves shows a New Tab window; pending, conflict, empty and disconnected states stay honest.
entry_points: web topbar desktop indicators (click, keyboard, context menu); tab drag onto an indicator or overflow control; All desktops button; Window › All desktops; Desktops overview
qa_status: skipped
bug_ids: BUG-20260724-stale-return-anchor-on-desktop-transfer
fix_status: pending
retest_status: pass
fix_commits: a1baedd3a
evidence: /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-window-manager-zoom-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/test-cases/walk-parity-results.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/04-zoom-lifted-over-neighbor.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/screenshots/05-unzoom-returned-home.png; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-window-manager-multi-client; ET-web-desktop-shell-lifecycle
---

story: As a person running agent work, I can move between many desktops from a quiet control and open full management only when I need it.

scope: Verify screen-reader naming and position, 44px target equivalence, visible focus, contrast, reduced motion, portrait/landscape placement, 1/2/7/8+ counts, and no application remount during a switch.

qa-impact: 2026-07-22 replaced the workspace-card Spaces overlay with a persistent-desktop dot pager and on-demand management; 2026-07-24 desktop switch transitions moved from CSS transitions to keyframes at `--duration-shell-base` (was shell-fast), covering reconciled switches as well. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 cross-desktop tab activation and multi-instance cycling now switch the active
client through the pager projection. Reset as the adjacent desktop canary.

qa-impact: 2026-09-01 focus desktops no longer exist as a kind: a zoom whose desktop shows another
window adds a regular desktop right after the current one (the pager gains a dot and the zooming
client moves to it), a zoom on a desktop that shows nothing else adds none, and unzoom or closing the
zoomed unit removes the desktop it created when it is empty; "current and focus desktops are
distinguishable" is dropped from the expectation and the overview no longer shows a Focus pill. Reset
for the current build.

qa-impact: 2026-09-01 correction to the earlier note: a zoom whose desktop shows another window adds a regular desktop right after the current one (the pager gains a dot and the zooming client moves to it); unzoom or closing the zoomed unit removes that desktop when it is empty. Walked through the zoom walk pager counts and P2.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). The pager moved from the lower-left carousel (Dock centerline) into the topbar tray, and a dedicated All desktops button opens the overview. The overview expectation follows the shell-rail prototype (VC-09 overview, VC-10 New desktop card); shell C4 is redesigning the production overview, so confirm its final anatomy with the shell owner before walking. Retest the open bug BUG-20260724-stale-return-anchor-on-desktop-transfer in the same walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-08 brand identity v1.2 (spec `.compozy/tasks/brand-site/`). The needs-you dot moved from the retired orange brand hue to its own amber `--color-attn` (`StatusDot tone="attn"`, `#f5b14c` dark / `#9a5b00` light). Expectation updated; status unchanged.

qa-impact: 2026-11-13 window-tabs-split S8/S9. The pager became the desktop indicators (context
menu, drop targets, overflow drop surface) and the overview lists tabs per card with card drag
reorder, tab drops, "Move a tab ▸" and in-place close with a destination. Walk: with four
desktops, right-click each end and a middle dot and confirm the item set; Shift+F10 on a focused dot
opens the same menu; Move right reorders; Rename and Close desktop open the overview on that card;
New desktop adds a dot. Drag a split tab onto another dot and confirm both bars and that the split
stays whole; drag the last tab of a desktop away and confirm a New Tab window remains. With nine
desktops, drag a tab, rest it on "+N", confirm the overview opens without the drag ending, drop on a
hidden desktop's card, and confirm the tab moved and the overview is still usable from the
keyboard afterwards (focus, Tab order, Escape). Also release over the overview off every card and
press Escape mid-drag: neither moves the tab nor closes the overview. Reset for the current build.
