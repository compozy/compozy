---
title: Agents can wait for other sessions to finish
type: feature
---

An agent can now hand work to several sessions, end its turn, and get one wake when they are done. Agents use it through `compozy__monitor_create` with `kind: session`, and the wake lists each session and its state. A session with an active monitor counts as having open work, so a quiet orchestrator is no longer stopped after 40 minutes.

- A monitor watches 1 to 32 sessions in the same project and profile, until `any` or `all` of them reach the states it asks for.
- A stopped, failed, archived or deleted session always counts, so it can't leave the agent waiting.
- A session waiting on its own background work, or with queued messages, doesn't count as idle yet.
- It reads each session as it is now. If the condition already holds, the monitor fires right away.
- The timeout is 2 hours by default and 6 at most, and a timeout still wakes the agent. Monitors survive a daemon restart.

This monitor has no CLI command, and it can't watch sessions in another project.
