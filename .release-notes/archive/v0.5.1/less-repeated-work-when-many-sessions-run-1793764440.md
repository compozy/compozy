---
title: Less repeated work when many sessions run
type: fix
---

A round of fixes from a performance review of many sessions running at once.

- **Clicks no longer stall in the desktop app.** Clicks on sessions and window tabs could hang for tens of seconds and then apply all at once, with "Can't save window layout — retrying". Three long-lived streams held half of the browser's connections; they now use WebSocket. A window command still waiting after 8 seconds is dropped instead of replayed.
- **Skills are reparsed only when they change.** A failed extension refresh no longer leaves skill discovery stale, and healthy profiles and projects stay usable when one fails.
- **Fewer catalog reads and writes.** Window transcripts follow the 5-second refresh window, cached catalogs are written only when they change, and pending-answer views refresh from the live stream instead of polling every 5 or 15 seconds.
- **Model discovery stops retrying sources that can't work,** such as a missing CLI. **Refresh** still tries them, and temporary failures still retry, backing off to at most 2 hours.
- **The desktop app caps its HTTP cache at 256 MiB** and trims a larger one at launch. Your data and sign-ins are kept.
- **Cancelled requests return 499** and no longer log as server errors.
