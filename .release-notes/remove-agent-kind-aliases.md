---
title: Retired agent kind inputs now report actionable errors
type: breaking
---

The release after v0.5.2 removes the agent kind input aliases retained since v0.5.1.
Use `always_on` in definitions, CLI flags, HTTP/UDS requests and native creation.

## Migration notes

Edit repository, workspace-profile, additional-root and extension AGENT.md files from
`kind: stays_on` to `kind: always_on`. Replace `[agents.stays_on]` with `[agents.always_on]`
in every config overlay and use `agents.always_on.*` in scripts. Boot no longer rewrites files.

- Old kinds fail with `agent_kind_invalid`: "agent kind must be on_demand or always_on (stays_on was renamed to always_on in v0.5.1)".
- Old tables fail with `removed config key "agents.stays_on": use "agents.always_on"`.
- Old config paths fail with `removed config key "agents.stays_on.max_proposals": use "agents.always_on.max_proposals"` (with the requested suffix).

No new database migration is needed: v0.5.1 already migrated persisted rows.
