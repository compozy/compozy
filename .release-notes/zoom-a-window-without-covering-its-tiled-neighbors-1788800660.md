---
title: Zoom a window without covering its tiled neighbors
type: fix
---

Zoom now moves a window or its entire tab frame to a separate regular desktop when needed, preserving the other windows. Unzoom returns the unit through its saved layout anchors and removes an empty desktop created for zoom. Tiling another window ends zoom and gives the new neighbor its own space.

Persisted version 3 layouts migrate losslessly to the current layout shape, preserving tab order and the active tab. Layout streams now use heartbeats, stalled-stream recovery, and authoritative reconnect fences so the browser catches up after a disconnect.

Moving one window no longer forces unchanged windows to render again, reducing work during layout updates from another browser or the CLI. The existing 12-window drag, restore, and peer-convergence performance limits remain enforced.

PR: [#525](https://github.com/compozy/compozy/pull/525).
