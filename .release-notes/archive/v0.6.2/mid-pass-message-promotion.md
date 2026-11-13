---
title: Messages sent during a long turn are no longer delayed
type: fix
---

A message from another session that arrived while a long-running turn was already checking its queue could wait for the next supervisor tick before being delivered. Those requests are now merged into the pass that is running, so the message reaches the agent right away.
