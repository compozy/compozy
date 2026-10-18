---
id: ET-terminal-window-native-flow
area: ET
title: Terminal windows are the only terminal tabs
persona: Marina
journey: J-operate-integrated-terminal
expected: The Terminal window shows exactly one terminal with no in-app tab strip; more terminals arrive as OS window tabs or windows; Journal and New terminal live in the window head; the first paint uses the full window width without a tab switch.
entry_points: Web dock Terminal app; window head New terminal and Journal; dock right-click Open in new window / Open in new tab
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/Dev/qa-labs/compozy-reported-issues-20260929-20260930-025651-581547-lab/qa-artifacts/qa/final-terminal-cap-native.json; docs/qa/reports/2026-09-29-reported-issues.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-terminal-browser-lifecycle
---

Reset by ADR-019 (agent-opened interactive terminals auto-materialize managed Terminal windows) and re-walked on 2026-09-01 — see `last_report`.

Added 2026-08-31 by the window-native UX rework (stabilization prompt
`docs/prompts/20260831-1333_integrated-terminal-stabilization-ux-rework.md`).

Walk:

1. Open the Terminal app; confirm the window shows one terminal with no second tab strip under the window head, and the grid fills the settled window width on first paint.
2. From the head, open the Journal; confirm the head shows the Journal crumb with a back affordance, and back returns to the same terminal with its scroll intact.
3. Use the head's New terminal; confirm a second terminal joins the frame as an OS window tab and the deck is the only tab strip visible.
4. Use the dock's right-click Open in new window; confirm another terminal opens as a floating window without touching the existing ones.
5. Use Stop and confirm the window stays on the exit bar. Close it with the window's Close control without a running warning. A running terminal instead asks for confirmation; there is no redundant Close terminal header action.

2026-09-30 queued adoption acceptance: seed eight running terminals, open Terminal from the dock,
and immediately use New terminal while the adopted terminal's route is settling. The deliberate
new window must use the accepted revision of the preceding adoption; it must show the unchanged
workspace-cap dialog naming all eight terminals rather than losing the action to a layout revision
conflict. Ordinary identity-based opens must still preserve their semantic lookup revision fence
so another client's concurrent open cannot create a duplicate. Existing Web E2E-009 owns the public
cap journey; the window-manager runtime suite owns this serialized dispatch boundary.

QA re-walk 2026-09-06: the terminal grid now fills a flex column whose height follows the window. A targeted rendered probe measured the host at 395px inside a 417px container; hidden panes cast no minimum-size vote. All 12 terminal journeys passed, including live watcher-size agreement after reflow. Evidence: `.cache/sessions-terminal-artifacts-probe2b/` and `.cache/sessions-terminal-e2e002-fixed2-all2-results.json`; BUG-20260906-hidden-terminal-pane-minimum-vote.

QA re-walk 2026-09-06: the packaged macOS Terminal E2E-013 journey passes input, clipboard, accelerators, zoom/refit and IME in 14.5s. Its watcher compares the current RESIZED grid with the visible size vote, accounting for the viewers footer. Evidence: `.cache/sessions-final-desktop-terminal-e2e-run2.log`; web/dist restored byte-for-byte.

Final selection-layout re-walk 2026-09-06: the packaged macOS Terminal E2E-013 passes again in12.8s with selection actions over the grid. Clipboard selection, accelerators, zoom/refit and IME remain functional; `.cache/sessions-final-desktop-selection-e2e.log`, with web/dist restored byte-for-byte.

QA re-walk 2026-09-10: PASS for the changed close contract. Production-bundle E2Es cover running cancel/confirm, grouped reload/history, disconnect feedback, Stop, and exited close. Manual isolated-browser checks cover keyboard/window-menu close, mixed-app groups, close-other/right targeting, shared viewers, and unchanged native view-only close. Scope and evidence: `docs/qa/reports/2026-09-10-issue-594-terminal-close.md`. Unchanged steps retain their earlier evidence.

QA re-walk 2026-09-30: PASS for the queued-adoption capacity flow on actual isolated Electron final index71ea3236e585a54471d994843a07ea687a1797a34badc28b6af7086f13d18b57. Public CLI created eight named interactive terminals in the owned project. Actual Terminal dock adopted Capacity QA8; actual head New terminal displayed the unchanged project limit dialog, 8 of 8, naming all eight exact IDs. Independent public listing still contained eight terminals. No lost409 action was observed. The manual CUA click interval was approximately1.6s and does not claim reproduction of the66ms CI race; the canonical serialized-queue regression and unchanged E2E-009 own that causal boundary. Only these eight disposable terminals were subsequently killed through public CLI, all successful. Evidence: final-terminal-cap-public-seed.json, final-terminal-cap-after-list.json, final-terminal-cap-native.json and final-terminal-cap-network.json in the canonical report lab.

qa-impact: 2026-09-30 shell-rail polish P6 — the dock menu's Open in new window now opens a floating window and "Open as tab" became Open in new tab; a plain Terminal launch with no open terminal joins the focused window as a tab (daemon default `tab`). Agent-materialized terminals still tile right after their bound session. Reset for a re-walk of steps 1 and 4.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
