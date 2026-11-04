---
title: Many sessions no longer wait on one database lock
type: fix
---

With around 30 active sessions, transcript updates, session health, model refreshes, process checks and a large event-summary log all waited on the same SQLite resources, so the app and agents slowed down together. CompozyOS now keeps that work apart.

- Interactive writes, such as sending a prompt or changing a session, go ahead of background work, which waits in a bounded queue.
- Global, profile and session reads have their own read pools, and transcript work is prepared before it takes the write lock.
- Session health is written in batches every 500 ms and flushed on a clean shutdown.
- Event summaries move to their own database, `$COMPOZY_HOME/compozy.db.telemetry.db`, which has its own write lock.

CLI commands, HTTP/UDS routes, native tools, Observe filters and cursors, hooks and config keys are unchanged.

### Migration notes

- On the first start after the upgrade, the daemon moves existing event summaries into `compozy.db.telemetry.db` in small batches and checks every row before it removes the old copy. On a large installation this first start takes longer and needs disk space for the new file. An interrupted move resumes at the next start.
- Back up and restore `compozy.db`, `compozy.db.telemetry.db` and their `-wal` and `-shm` files together. The two databases carry a matching identity. If the telemetry file is missing, empty or comes from another installation, the daemon refuses to start instead of replacing it; restore both files from the same backup. See **Operations › Database** for a live backup recipe.
