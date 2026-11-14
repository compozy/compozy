---
title: Back returns to the previous window, and the keyboard follows your focus
type: fix
---

Opening or focusing a window from the dock, the palette or a menu now adds exactly one browser history entry, so one Back returns to the window you came from. Before, dock **Files** could overwrite the previous entry and Back stayed on Files.

When you move focus yourself (directional focus, next or previous tab, **Focus last window**, the dock, the palette, a menu, Back or Forward), the keyboard now moves into the window you focused: to the field you last typed in there, or to the window itself. Focus sent from the CLI or another client still moves only the focused window and leaves the keyboard where you are typing.

After a daemon restart, an open browser or desktop client comes back on the desktop it was showing, with the same window focused, if both still exist.

Migration notes: no action needed.
