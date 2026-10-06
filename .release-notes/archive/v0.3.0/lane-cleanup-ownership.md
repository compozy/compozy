---
title: Preserve terminal cleanup ownership when canceling a lane
type: fix
---

Canceling a run-agent lane now records terminal session cleanup before generic cancellation
cleanup, preserving the exact session and binding epoch throughout settlement.
