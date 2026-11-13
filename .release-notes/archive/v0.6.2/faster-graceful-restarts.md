---
title: Restarts are faster and no longer look like a hang
type: fix
---

Restarting CompozyOS, from the desktop app, `compozy daemon relaunch` or an update, used to take longer the more sessions you had ever created, and could stall for a minute or more with no sign of progress.

- **The app opens sooner.** The daemon serves your sessions, transcripts and status as soon as it is listening, then resumes interrupted sessions in the background, several at a time. The boot window names the step in progress, and sessions that are still coming back show as resuming.
- **Startup cost follows what changed.** Sessions that have not changed since the last start are no longer reopened, repaired and re-indexed on every boot.
- **Shutdown is bounded.** A restart no longer waits up to 30 seconds for turns that are saved and resumed anyway, no new provider process starts once shutdown begins, the database is always checkpointed, and the process is guaranteed to exit.
- **A bad update no longer leaves you without a daemon.** Before the running daemon stops, the replacement is checked against your config and pending database migrations. If it cannot start, the running daemon stays up and the restart reports why.
- **Interrupted sessions continue on their own more often.** A leftover infrastructure error from an earlier start, such as a tool connection timeout, no longer pauses a session with "restart continuation requires operator input". Real agent failures still wait for you.
- **An interrupted upgrade no longer blocks startup.** A session database that already had the `reported_cost_amount` column without its migration recorded now finishes that migration instead of failing with `duplicate column name`.

The desktop app also waits for a restart or update that is already running instead of failing with "runtime mutation in progress", and no longer gives up on a daemon that is still starting after 45 seconds.

Migration notes: no action needed. `compozy daemon status` and `daemon.json` gain a readiness phase (`starting`, `serving`, `ready`, `stopping`); `status: running` still means fully ready, so existing scripts keep working.
