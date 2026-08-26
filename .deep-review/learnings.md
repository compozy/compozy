## 15bf6af1f103d644 — Keep extension hooks distinct from observability events

- Scope: internal/hooks/**
- Rule: Do not expand the agent-calls extension hook family beyond the exact events accepted by the spec; observability events are a separate catalog.
- Why: The agent-comms spec binds extension hooks to exactly seven events while defining eleven observability events independently.
- Origin: session, 2026-08-26

## cf13849291102e09 — Never fold an applied migration into another migration

- Scope: internal/store/globaldb/schema/migrations/**
- Rule: Preserve every existing migration byte and version; add a later migration when schema evolution is required.
- Why: The repository's append-only migration identity rule forbids editing, folding, renumbering, or reordering existing migrations.
- Origin: session, 2026-08-26
