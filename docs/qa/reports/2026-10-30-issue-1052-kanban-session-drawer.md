# Issue #1052 — Kanban session drawer walk (2026-10-30)

Scenario: `ET-web-sessions-board-drawer` (plus the updated `ET-web-sessions-board-toggle`).

Lab: isolated daemon built from the branch (`go build ./cmd/compozy`), `web/dist` served by the
daemon, demo-seed Northstar workspaces under **All projects**, acpmock-driver `builder` agent with
`[hold]`, `[stream]` and a child session spawned with `--parent`. Headless Chromium
(playwright-core 1.63) pinned to the lab port, 1600×1000 and 860×900, light and dark.

| Step | Result |
| ---- | ------ |
| 1 Card opens the drawer, board head kept, card highlighted, `drawer=open` in the URL | pass |
| 2 Swap cards; draft kept per session with the card's "Draft" mark; empty-board click ignored with unsent text, closes once cleared; focus on the card | pass |
| 3 Ctrl+Shift+↓/↑ from the drawer head step cards in board order (and from a focused card) | pass |
| 4 Drag and ←/→ resize; width persisted per window (`drawerWidth`) | pass |
| 5 Escape closes and focuses the card; Back reopens; reload restores | pass |
| 6 `[stream]` session moves Working → Done while open; drawer stays, highlight follows | pass |
| 7 Child session head shows the parent crumb | pass |
| 8 Archive from the drawer … closes it and focuses the next card in the column; delete lands the same; Escape in the delete confirmation keeps the drawer | pass |
| 9 860 px window: drawer covers the window (`data-mode=overlay`), Escape returns to the board | pass |
| 10 All projects: cards from both projects open in place | pass; another-profile session not walked (single-profile lab) — it renders the same `SessionProfileOwnerNotice` path as List, covered by `session-window.test.tsx` |
| 11 Open full session → List, `drawer=open` dropped | pass |

Accessibility: axe-core 4.13 on the open drawer reported no violations; the drawer is a non-modal
`dialog` named by the session title. With `reducedMotion: reduce` the overlay mode runs no
animations; the inline pane never animates.

Evidence (R2):

| State | Dark | Light |
| ----- | ---- | ----- |
| Open | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/dark-1-open.png | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/light-1-open.png |
| Resized | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/dark-2-resized.png | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/light-2-resized.png |
| Swapped card (child session) | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/dark-3-swapped-child.png | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/light-3-swapped-child.png |
| Card in Working while open | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/dark-4-moving-working.png | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/light-4-moving-working.png |
| Card moved to Done, drawer still open | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/dark-5-moved-done.png | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/light-5-moved-done.png |
| Narrow window | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/dark-6-narrow.png | https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/kanban-session-drawer/7fa57a70/light-6-narrow.png |

Found and fixed during the walk: a click on an empty-board status line read as a control (role
selector too broad); arrow-key session chords were swallowed by the dialog's composite-key handling
(the drawer now answers them); focus after archive/delete was lost to an effect cleanup and to the
delete retiring the window first (landing is now chosen when the verb is requested).
