---
title: Panes keep their scroll position across more tab and split moves
type: fix
---

Moving a window between tabs and splits keeps your place more reliably. A session
transcript returns to the same text after a tab switch or a width change and no longer
drifts forward once shown; a turn that started while its tab was hidden is shown when you
come back. Lists such as the Files tree keep their offset after a tab drag into a split,
a list beside a detail keeps its offset after a later move, and a list that an in-window
navigation sends to the top right after a tab switch stays at the top. Moving a shown
window into a new tab with `compozy window move --placement tab` keeps its pane open
instead of reloading it.

Migration notes: no action needed.
