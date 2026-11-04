---
title: Stopping the daemon no longer records running agents as crashed
type: fix
---

When the daemon was stopped from outside while a session was mid-turn, for example by logging out of the OS, shutting down or quitting the desktop app, the session was recorded as failed with "ACP subprocess exited unexpectedly". That also wrote a crash bundle, fired `agent.crashed`, tried to restart the agent and sent the parent a `failed` wake.

Now an agent that exits while the daemon is shutting down is recorded as stopped by the daemon. No crash bundle or `agent.crashed` follows, the daemon doesn't restart agents while it stops, and a parent gets a `stopped` wake. After a restart, the interrupted turn still continues. An agent that exits while the daemon keeps running is still a crash.
