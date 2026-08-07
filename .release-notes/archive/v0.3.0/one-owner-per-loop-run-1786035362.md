---
title: One owner per Loop run, and cancellation that sticks
type: fix
---

Loop action runs now have exactly one daemon-owned worker, cancellation survives a restart, and a session that needs Compozy tools fails before the provider starts instead of running without them. Fresh Compozy homes also start with the bundled `dev-cycle` extension already enabled, while a home that has been booted before keeps whatever you chose. (#321, #322, #326)

- Coordinators and ordinary task-role sessions can no longer activate or bootstrap a run that the dedicated `loop-action` executor already owns.
- When the effective agent or lineage policy requires concrete tools and hosted MCP cannot provide them, session startup fails closed with `ErrHostedMCPUnavailable` before the provider process is launched.
- Loop cancellation is durable: delivery state is persisted, delivery is idempotent, the run advances to draining once acknowledged, and anything still pending is retried from daemon boot and from scheduler sweeps — no restart required to converge.
- Resuming a stopped session discards the stopped ledger projection first and restores it if provider startup or the clear rolls back, so forensic projections stop conflicting and the full history is rematerialized on the next stop.
- Enablement of a bundled extension is a fresh-home default, not an override: generic local and marketplace installs stay disabled by default, and stored state survives restart and update.
