---
id: ET-web-session-sidebar-threads
area: ET
title: In-window sessions sidebar with provenance threads and in-place switch
persona: Bruno
journey: J-14
expected: A session window's topbar shows a sessions toggle (the dock's sessions icon, SquareTerminal) before the goal action; the sidebar starts closed (transcript full-bleed) and opens as a 264px left rail hosting the shared sessions list (filter, Recent ⇄ All panes, agent groups). Sessions whose lineage.parent_session_id is loaded nest under their root behind a hairline connector; the parent row carries a count toggle that folds the thread, and a collapsed thread with a failed/waiting/running child shows a danger/warning/accent signal dot. The current session row carries an accent left bar. Clicking another session switches this window to it in place (URL follows, one history entry); if that session already has its own window, that window is focused instead and no duplicate opens. The footer New session action opens the create flow. Open preference and per-thread collapse persist across reloads (localStorage compozy:session:sidebar:v1).
entry_points: web session window topbar (session-sidebar-toggle, dock sessions icon); SessionSidebar; sessions modal (shared threads); localStorage key compozy:session:sidebar:v1
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-sidebar-parent-20260806-212647-734931-lab/qa-artifacts/qa/journey-log.jsonl
last_report:
overlaps: ET-web-sessions-catalog-modal; ET-web-session-thread-full-bleed; ET-web-session-inspector-toggle
---

Added by the session sidebar + parent provenance feature (2026-08-06), implementing
docs/design/opendesign/session/session-sidebar.html. Flag only — walk in the next QA cycle.

2026-08-06 walked live in lab compozy-session-sidebar-parent-20260806-212647: closed default, PanelLeft toggle, 264px rail, provenance thread (chip=5) nested behind connector, aria-current accent row, chip collapse/expand, in-place switch (URL changed, one window), dedup focus onto the session's existing window (no duplicate), persistence across reload, New session footer, modal shares the same threads. Evidence: journey-log.jsonl + session-sidebar-open-threads.png. Verdict: pass.

2026-08-06 post-walk adjustment: the sidebar toggle icon changed from PanelLeft to the dock's sessions icon (SquareTerminal) so the topbar doesn't show two near-identical panel glyphs. Toggle behavior, testid, and aria are unchanged (component suite re-run green); icon rendering re-verified in the live shell.
