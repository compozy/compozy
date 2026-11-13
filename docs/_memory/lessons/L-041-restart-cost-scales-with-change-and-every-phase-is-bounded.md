# L-041 — Restart work scales with what changed, and every boot and shutdown phase has a bound

**Class:** Runtime lifecycle / Persistence / Process
**Date discovered:** 2026-11-10 to 2026-11-12 (hung drain, failed v0.6.0 upgrade, 168 s restart); recorded 2026-11-12
**Evidence sources:** issue #1614 §Evidence (22 recorded restarts 2026-10-19 → 2026-11-12 on a workstation with about 2,600 retained sessions); `internal/daemon/boot_components.go`, `boot_session_repair.go`, `boot_finalize.go`, `daemon_shutdown_resources.go`; `internal/store/sessiondb/schema/migrations/00011_schema.sql`; the closed restart fixes #559, #939/#942, #1066, #1243/#1245, #1251/#1252, #1557, #1580.

## Context

A CompozyOS restart had become slow and unreliable while each individual defect was being fixed. Signal-to-ready across 22 restarts was p50 50 s, p90 309 s, max 874 s; the median was 11 s before 2026-11-01 and 91 s after. Four separate facts sat behind that:

- **Boot work proportional to total history.** About 45 boot steps ran in series before readiness. `upgradeBootSessionDatabases` opened every retained `events.db` serially, repair ran serially, and `observer.Reconcile` re-parsed every `meta.json` and upserted every catalog row on every boot while reporting `indexed=0 orphaned=0`. Boot cost about 30–35 ms per retained session (about 3 s at 30 sessions, about 85 s at 2,550) and correlated with session count at r = 0.71. The cold floor with nothing to resume was 26–34 s.
- **Shutdown without a bounded close or a watchdog.** One 60 s context covered the drain, the stop ladder and the close. 6 of 11 shutdowns spent the exact 30 s drain timeout. When the deadline expired, `shutdownPersistentResources` returned early and skipped the checkpointed global DB close, leaving a 67 MB WAL. Nothing forced exit: on 2026-11-10 the daemon logged shutdown and then served for 103 min while desktop bootstrap failed with `runtime mutation in progress`.
- **A per-session migration on the boot critical path that was not safe to meet twice.** The v0.6.0 upgrade on 2026-11-11 spent about 28 min upgrading every session DB before readiness. The replacement daemon then died on session migration `00011` with `duplicate column name: reported_cost_amount` and was rolled back, leaving the user without a daemon for about 14.5 min. The migration opens with an unguarded `ALTER TABLE token_usage ADD COLUMN`.
- **Recurrence.** About 25 closed restart fixes share the same symptom classes (#559 → #1251/#1252 → 2026-11-11; #939/#942 and #1243/#1245 → #1557; #1066 → #1580), and `docs/_memory/lessons/` had no lesson on boot or shutdown.

## Root cause

1. **Boot steps were written as "for every session" and nothing tied their cost to pending work.** Each step was cheap when it shipped; none carried a budget or a measurement, so the growth from 11 s to 91 s was only visible to the user.
2. **Shutdown had one shared deadline and no owner for "the process must exit".** A slow early phase consumed the budget of the phase that protects state, and a hung phase had no backstop at all.
3. **The migration runner checks and applies in separate steps, and one open can fail the whole boot.** Goose lists applied versions before it opens the migration's deferred transaction, and the session family lease is released before migrations run (`internal/store/sessiondb/owner_open.go`), so a second opener of the same database can reach the `ADD COLUMN` after the first committed it. The declarative bootstrap also stamps whatever versions the binary embeds without proving `schema.sql` matches them (`internal/store/migration_bootstrap.go`), so a build whose schema is ahead of its migrations creates a column with no version. Which of these produced the 2026-11-11 database is unconfirmed; the code proves that the migration's own transaction cannot leave the column without the version.
4. **Every incident was fixed at its symptom.** Twenty-five fixes each closed one ordering or timeout bug without recording the class, so the next change reintroduced it.

## Rule

> A restart is a product path with a budget. **Boot does work in proportion to what changed since the last boot**, never to total history, and serves before it recovers. **Every shutdown phase has its own bound**, the state-protecting close always runs, and a watchdog guarantees exit. **Work on the boot critical path must survive being interrupted, repeated or raced** — a migration that cannot is a boot outage waiting for the one database that differs.

## Operationalization

- Before adding a boot step, state what it iterates over. A step that walks all sessions, workspaces or files needs a watermark or a change listing, a named duration in the boot log, and progress through the boot-window channel if it can exceed a second.
- Give each shutdown phase its own deadline. The checkpointed database close and the hard-exit watchdog are not allowed to share a budget with drain or session stop.
- Treat each per-session migration as running thousands of times on databases with different histories. A published migration stays immutable; when one meets a database it cannot replay, add a named reconciliation to the owning stream (`store.MigrationStream.Reconciliation`, first used for session migration 11) that detects the exact state, completes the migration idempotently and records the version in one transaction. Never repair schema ad hoc at boot.
- When a restart defect is fixed, name its class in the PR (ordering, unbounded phase, history-proportional step, non-repeatable step) and check this lesson before adding a new mechanism.

## Anti-pattern

- "It only takes a few milliseconds per session" as the justification for a boot loop over all sessions.
- One context deadline shared by drain, stop and close, so the phase that protects data runs last on whatever time is left.
- Returning early from shutdown on a timeout and skipping the WAL checkpoint.
- A migration whose first statement fails when its effect already exists, placed on the path every restart must cross.
- Closing a restart bug with a reordered step or a longer timeout and no measurement of the phase it belongs to.

## Source

- Issue #1614 — measurements, incident timeline and the approved direction (observability, serve before recovery, bounded parallel recovery, boot cost proportional to change, bounded shutdown, safe relaunch, desktop handoff).
- `internal/daemon/boot_session_repair.go` (`upgradeBootSessionDatabases`), `internal/daemon/boot_finalize.go` (`observer.Reconcile`), `internal/daemon/daemon_shutdown_resources.go` (`shutdownPersistentResources`) — the history-proportional steps and the skipped close as they stood on 2026-11-12.
- `internal/store/sessiondb/schema/migrations/00011_schema.sql` and `internal/store/sessiondb/usage_cost_reconciliation.go` — the unguarded `ADD COLUMN` and its reconciliation.
- Prior fixes of the same classes: #559, #939/#942, #1066, #1243/#1245, #1251/#1252, #1557/#1560, #1580/#1591, #1549/#1600, #1582.
- Related: [L-008](L-008-schema-migrations-mandatory.md) (migrations are mandatory), [L-040](L-040-real-users-end-zero-legacy-posture.md) (user state upgrades losslessly).
