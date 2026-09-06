---
id: ET-terminal-window-native-flow
area: ET
title: Terminal windows are the only terminal tabs
persona: Marina
journey: J-operate-integrated-terminal
expected: The Terminal window shows exactly one terminal with no in-app tab strip; more terminals arrive as OS window tabs or windows; Journal and New terminal live in the window head; the first paint uses the full window width without a tab switch.
entry_points: Web dock Terminal app; window head New terminal and Journal; dock right-click Open in new window / Open as tab
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-rework-20260901-150952-749450-lab/qa-artifacts/qa; docs/qa/reports/2026-09-01-terminal-rework.md
last_report: docs/qa/reports/2026-09-01-terminal-rework.md
overlaps: ET-terminal-browser-lifecycle
---

Reset by ADR-019 (agent-opened interactive terminals auto-materialize managed Terminal windows) and re-walked on 2026-09-01 — see `last_report`.

Added 2026-08-31 by the window-native UX rework (stabilization prompt
`docs/prompts/20260831-1333_integrated-terminal-stabilization-ux-rework.md`).

Walk:

1. Open the Terminal app; confirm the window shows one terminal with no second tab strip under the window head, and the grid fills the settled window width on first paint.
2. From the head, open the Journal; confirm the head shows the Journal crumb with a back affordance, and back returns to the same terminal with its scroll intact.
3. Use the head's New terminal; confirm a second terminal joins the frame as an OS window tab and the deck is the only tab strip visible.
4. Use the dock's right-click Open in new window; confirm another terminal opens without touching the existing ones.
5. End a session from the head's overflow Close terminal; confirm the window stays put on the exit bar until you close it yourself.

QA re-walk 2026-09-06: the terminal grid now fills a flex column whose height follows the window. A targeted rendered probe measured the host at 395px inside a 417px container; hidden panes cast no minimum-size vote. All 12 terminal journeys passed, including live watcher-size agreement after reflow. Evidence: `.cache/sessions-terminal-artifacts-probe2b/` and `.cache/sessions-terminal-e2e002-fixed2-all2-results.json`; BUG-20260906-hidden-terminal-pane-minimum-vote.
