---
title: Retired agent kind inputs now report actionable errors
type: breaking
---

The release after v0.5.2 removes the agent kind input aliases retained since v0.5.1.
Use `always_on` in definitions, CLI flags, HTTP/UDS requests and native creation.

## Migration notes

Persisted `[agents.stays_on]` tables migrate on load to `[agents.always_on]`. Canonical keys
win conflicts; original values and source text are retained as inactive comments. Persisted
AGENT.md files migrate when loaded from any discovery root, including extension roots.
Writable user-owned scalar layouts update only the kind value. Extension packages are never edited. Read-only files and unsupported edit layouts
still decode as `always_on` and log guidance to update the source before v0.6.0.
This single persisted-file shim generation is scheduled for removal in v0.6.0.
CLI/native `agents.stays_on.*` paths still fail with replacement guidance; public creation,
update and duplicate inputs still reject the retired kind. Database rows already migrated in v0.5.1.

- Old kinds fail with `agent_kind_invalid`: "agent kind must be on_demand or always_on (stays_on was renamed to always_on in v0.5.1)".
- Old tables in persisted config files migrate on load; API/config mutation paths reject the retired key.
- Old config paths fail with `removed config key "agents.stays_on.max_proposals": use "agents.always_on.max_proposals"` (with the requested suffix).

No new database migration is needed: v0.5.1 already migrated persisted rows.
