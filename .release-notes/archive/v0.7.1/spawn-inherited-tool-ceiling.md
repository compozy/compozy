---
title: Narrow inherited child tools to the parent agent's permissions
type: fix
---

Spawning a less restricted child agent with default inheritance now narrows any broader effective tool set to the parent's permissions. Explicit requests for tools outside the parent's permissions are still refused.
