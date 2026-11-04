---
title: The v0.5.0 stays_on spelling is no longer accepted on input
type: breaking
---

v0.5.1 renamed the agent kind `stays_on` to `always_on` and kept the old spelling working for one release. That compatibility layer is now gone: requests, flags, native tools and config paths accept only `on_demand` and `always_on`. Files you already have are migrated in place, so an upgrade keeps your agents and settings.

### Migration notes

- **`config.toml`.** The global, profile and workspace config files migrate on load. Keys under `[agents.stays_on]` move into `[agents.always_on]`; a key already set there keeps its value, as in v0.5.1. The old table is removed and its original values stay at the end of the file as an inactive comment. The daemon logs `config.retired_keys_archived` with the moved keys.
- **AGENT.md files.** The daemon rewrites `kind: stays_on` to `kind: always_on` and changes no other byte. Files under `$COMPOZY_HOME/agents` and `$COMPOZY_HOME/profiles/<profile>/agents` migrate at boot. Repository, workspace-profile and additional-root files migrate when their workspace resolves. Extension AGENT.md files belong to the extension package and are never rewritten; extension authors must ship `always_on`.
- **Files the migration skips.** It leaves a file untouched and logs `agent kind rewrite skipped` with the reason and path when the file is:
  - read-only or a symbolic link
  - changed during the rewrite
  - unreadable
  - written in a form it cannot edit safely, such as `kind: &k stays_on`

  This applies even if you ran v0.5.1, because its rewrite had the same skips. Such a file stops loading, and the workspace agent diagnostics name it with "agent kind must be on_demand or always_on (stays_on was renamed to always_on in v0.5.1)". Edit it to `kind: always_on`. To find leftovers, search your agent roots for `stays_on`.

- **Scripts and integrations.** `--kind stays_on` and request `kind: "stays_on"` (create, update, duplicate and its `overrides.kind`) fail with `agent_kind_invalid` and the message above. `compozy__agent_create` lists only `on_demand` and `always_on` in its input schema, so `kind: "stays_on"` fails input validation on `$.kind`. `compozy config get|set|unset` and `compozy__config_*` reject `agents.stays_on.*` paths with `config: removed config path "agents.stays_on.<key>": use "agents.always_on.<key>"`. The generated TypeScript and Go request types narrow to `"on_demand" | "always_on"`.

Stored rows need nothing: the v0.5.1 migration already moved `definition_not_stays_on` reasons and agent resource records to the new values.
