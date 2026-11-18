---
title: The daemon starts faster with always-on agents running
type: fix
---

Restarting CompozyOS with active always-on agents no longer stalls startup. Before, each agent's main session resumed before the daemon was listening, so its CompozyOS tools waited out a connection deadline and failed before a retry, adding that delay per agent before the desktop app was ready. Always-on agents now resume once the daemon is accepting connections, and connect on the first attempt.

`compozy app status` also no longer rejects the desktop app's state while it is still starting.

Migration notes: no action needed.
