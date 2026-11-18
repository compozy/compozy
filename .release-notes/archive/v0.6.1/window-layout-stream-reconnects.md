---
title: Window layout saving recovers without a reload
type: fix
---

If the app hit an error while syncing the window layout, it now reconnects on its own. Before, the menubar kept showing "Can't save window layout — retrying" with no detail, every dock item stayed disabled, and window commands did nothing until you reloaded the app. The message now shows the underlying error, the app reconnects with increasing delays, and the dock works again as soon as the layout syncs.

Migration notes: no action needed.
