---
id: ET-window-pane-scroll-keep
area: ET
title: A window's pane keeps its scroll position across tab and split moves
persona: Bruno
journey: J-administer-window-manager
expected: Switching tabs, dragging a tab into a split, and `compozy window move` (into a visible split or into a new tab) keep each pane mounted with its position. A transcript returns to the same message and the same text, also after a width change, and holds there while its rows re-measure; a turn that started while hidden shows the new turn. The Files tree and other lists return to their offset. A list that stays beside a detail route keeps its offset after a later move. An in-window navigation that resets a list to the top right after a show keeps it at the top.
entry_points: tab bar; tab drag onto window edges; compozy window move; compozy__window_move; sidebar and palette navigation
qa_status: untested
bug_ids: 1654
fix_status: fixed
retest_status:
fix_commits:
evidence: web/src/systems/os/hooks/__tests__/os-interaction-hooks.test.tsx; web/src/components/assistant-ui/__tests__/session-thread.test.tsx
last_report:
overlaps: ET-window-manager-layout-gestures; ET-hidden-window-live-resources
---

story: As a builder, I can rearrange tabs and splits without losing my place in a transcript, a file tree or a list.

scope: Include a long transcript read at 0.3, 0.5 and 0.7 of its length, Settings and Tasks as the other tab, a reply streaming while hidden, a wide code block scrolled right, the Files tree at a deep offset, a Settings section rail beside a section, and a CLI move of a shown window into a new tab.

qa-impact: 2026-11-13 #1654. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell with a long session transcript, a Files window and a Settings window in separate tabs. Scroll the transcript to about half; switch to Settings and back: the same row stays in view and does not move afterwards. Drag the transcript's tab onto the Files window's edge: the pane narrows and the same text stays at the top. Scroll the Files tree deep, drag its tab into a split: the tree keeps its offset. Run `compozy window move <files-window> --placement tab`: the tree and the open file keep their state and offset when that tab is shown. In Settings, scroll the section rail, open another section, then move the window into a split: the rail keeps its offset. Hide the transcript's tab, send a prompt from the CLI, wait for the reply to end, show the tab: the new turn is in view.
