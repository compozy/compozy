---
title: Terminal windows no longer hang on "Connecting…"
type: fix
---

A **Terminal** window in the Web UI could stay on "Connecting…" forever, with no prompt and no error, most often with the daemon in WSL2 and the browser on Windows. Each connection attempt now has a 10-second deadline. A stalled attempt is retried automatically and the window shows "Reconnecting…". After three stalled attempts in a row, the window says it couldn't connect to the terminal and offers **Reconnect**.

Migration notes: no action needed.
