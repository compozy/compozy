---
id: RT-desktop-pager-overview
area: RT
title: Navigate persistent desktops through the minimal pager
persona: Théo
journey: J-administer-window-manager
expected: The desktop pager sits in the topbar tray before All desktops: one 6px dot per persistent desktop in order, the active desktop drawn as an 18px foreground pill, and an orange dot on an off-screen desktop that needs you, each named "Desktop N of T: {name}" ("— needs you" appended when it does); click and Left/Right/Home/End switch only the active client; 1, 2, and 7 desktops stay direct and 8+ window ±2 behind "Show N earlier/later desktops" controls; All desktops opens the Desktops overview over the desk — the title "Desktops", one live layout thumbnail per desktop with its name, window count and position key, the active desktop ringed, a New desktop card, and Done/Esc to close — which creates, renames, reorders, transfers, and deletes desktops with honest pending, conflict, empty, and disconnected states.
entry_points: web topbar tray pager; All desktops button; Desktops overview; keyboard navigation
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
