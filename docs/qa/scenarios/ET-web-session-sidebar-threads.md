---
id: ET-web-session-sidebar-threads
area: ET
title: In-window sessions sidebar with provenance threads and in-place switch
persona: Bruno
journey: J-14
expected: A session window's topbar shows a List-icon sessions toggle before the goal action; the sidebar starts closed (transcript full-bleed) and opens as a 264px left rail hosting the shared sessions list (filter, Recent ⇄ All panes, agent groups). Sessions whose lineage.parent_session_id is loaded nest under their root behind a hairline connector; the parent row carries a count toggle that folds the thread, and a collapsed thread with a failed/waiting/running child shows a danger/warning/accent signal dot. The current session row carries an accent left bar. Clicking another session switches this window to it in place (URL follows, one history entry); if that session already has its own window, that window is focused instead and no duplicate opens. The footer New session action opens the create flow. Open preference and per-thread collapse persist across reloads (localStorage compozy:session:sidebar:v1).
entry_points: web session window topbar (session-sidebar-toggle, List icon); SessionSidebar; sessions modal (shared threads); localStorage key compozy:session:sidebar:v1
qa_status: blocked-verify
bug_ids: compozy/compozy#416
fix_status: pending
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-08-16-loop-goal-origin-session-lineage.md
overlaps: ET-web-sessions-catalog-modal; ET-web-session-thread-full-bleed; ET-web-session-inspector-toggle
---

Added by the session sidebar + parent provenance feature (2026-08-06), implementing
docs/design/opendesign/session/session-sidebar.html. Flag only — walk in the next QA cycle.

2026-08-06 walked live in lab compozy-session-sidebar-parent-20260806-212647: closed default, PanelLeft toggle, 264px rail, provenance thread (chip=5) nested behind connector, aria-current accent row, chip collapse/expand, in-place switch (URL changed, one window), dedup focus onto the session's existing window (no duplicate), persistence across reload, New session footer, modal shares the same threads. Evidence: journey-log.jsonl + session-sidebar-open-threads.png. Verdict: pass.

2026-08-06 post-walk adjustment: the sidebar toggle icon changed from PanelLeft to the dock's sessions icon (SquareTerminal) so the topbar doesn't show two near-identical panel glyphs. Toggle behavior, testid, and aria are unchanged (component suite re-run green); icon rendering re-verified in the live shell.

2026-08-06 PR 327 CodeRabbit impact: retargeting an already-open session now reconciles its requested route; cyclic lineage remains visible; filtered descendants retain their full ancestor path; collapsed thread and agent bodies are inert. Reset for a targeted live re-walk.

2026-08-06 PR 327 completion: a live root → child → grandchild thread retained its full ancestry under a grandchild-only filter. The thread toggle stayed keyboard-reachable while collapsed children left the accessibility tree, open/collapse preferences survived reload, and selecting an already-open root focused it with two windows still present. The exact differing-route branch and malformed multi-session cycle are public-state edge cases owned by the canonical regression suites. Verdict: pass.

2026-08-06 operator adjustment: the window-level sidebar toggle now uses Lucide List. The dock keeps its own Sessions icon; the topbar control now describes the list it reveals without implying a terminal. Re-verified live with the sidebar open: the List glyph rendered while `aria-pressed` and the accessible label remained truthful.

2026-08-16 issue #416 impact: Loop Goal system sessions now receive informational lineage from the session that started the Loop. Reset for an isolated re-walk that proves the Goal nests under a loaded origin, becomes a visible root when the origin is absent, and keeps neutral child-session copy without changing safe-spawn governance.
