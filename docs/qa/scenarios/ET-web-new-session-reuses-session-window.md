---
id: ET-web-new-session-reuses-session-window
area: ET
title: A new session lands in the operator's existing Session window instead of opening another
persona: Bruno
journey: J-17
expected: Creating a session from any launcher (dock Sessions, menubar Session > New session, ⌘K New session, its shortcut, a session list or folder "+", a Session window's sidebar or empty state, an agent page, the empty-desktop composer, the palette prompt fallback) shows it in a Session window that already exists on the landing desktop. When the focused window is a Session window, the new session replaces its conversation in place and the window keeps focus. Otherwise an empty-state Session window (/sessions) is preferred, then the most recently focused Session window; its tab is activated and the window focused, even when it sits in a background tab or inside a split. The window keeps its tab and its place in a split. Pinned Session windows and Session windows on other desktops are never reused. With no eligible Session window, a new window opens as before. Split and "open in new window" requests still open a new window. A session that already has a window focuses that window. The replaced conversation keeps running, keeps its unsent draft, and is one click away in the window's session sidebar.
entry_points: web dock Sessions; menubar Session > New session; command palette New session; New session shortcut; session list and folder "+"; Session window sidebar "+" and empty state; agents page Start session; empty-desktop composer; palette prompt fallback
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case1-focused-after.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case1b-draft-kept.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case2-unfocused-after.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case2b-minimized-after.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case3-several-after.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case5-other-desktop-after.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case6-pinned-after.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case4-none-after.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-825/case7-desk-composer-after.png
last_report: docs/qa/reports/2026-10-19-issue-825-session-window-reuse.md
overlaps: ET-web-session-prompt-runtime-and-create-navigation; ET-web-empty-desktop-card; ET-palette-agent-fallback; ET-web-session-sidebar-threads; ET-window-tab-pinning
---

Added 2026-10-19 for issue #825. Walk, on a single desktop unless noted:

1. Open one session and keep its window focused, type an unsent draft, then create a session from the dock. The same window (same tab and frame) now shows the new session and stays focused. Choose the previous session in the window's sidebar: its draft is still there.
2. Focus another app window (Tasks) and create a session from the menubar. The Session window comes forward with the new session; no second Session window appears.
3. Activate another tab so the Session window sits in a background tab, then create a session from ⌘K. Its tab is activated and the window shows the new session.
4. With two Session windows, one of them retired to the empty state, create a session: the empty-state window receives it. Without an empty one, the most recently focused Session window receives it.
5. Pin the only Session window, or move it to another desktop, then create a session: a new Session window opens, and the pinned or other-desktop window is left as it was.
6. Close every Session window and create a session (also from the empty-desktop composer once the desktop has no tabs): a new window opens (today's behavior).
7. Continue a session with "open in new window": it still opens beside the current window. (Fork has no
   placement choice since #1024; it always replaces the conversation in the current window.)
8. Split the Session window with another app in one tab, activate a different tab, then create a session from ⌘K: the split tab is activated, the Session member is focused and shows the new session, and the split is unchanged.

QA 2026-10-19: steps 1–8 walked in an isolated Playwright lab against the branch daemon; see the dated report. Dialog steps used Session > New session; the dock, ⌘K, list "+", sidebar and agent-page launchers open the same dialog and share its submit path. The palette prompt fallback was not walked (it shares the empty-desktop composer's created-session jump, step 8).

qa-impact: 2026-11-13 window-tabs-split S10. Minimize and window frames are retired; a window is open in a tab (alone or in a split) or closed. Expected and steps 3 and 8 rewritten: the reused Session window is brought forward by activating its tab (also from inside a split) instead of being restored from minimized. Step 8 now covers the split case; a desktop holding a Session window is not empty, so the empty-desktop composer moved to step 6. Walk steps 1–8. Minimized-window evidence above is history. Reset for the current build.
