---
title: The v0.5.0 stays_on spelling is no longer accepted
type: breaking
---

v0.5.1 renamed the agent kind `stays_on` to `always_on` and kept the old spelling working for one release. v0.5.2 removes that compatibility layer as announced. Every surface now accepts only `on_demand` and `always_on`, and the daemon no longer rewrites AGENT.md files at boot.

### Migration notes

If you ran v0.5.1, the daemon already rewrote the AGENT.md files under `$COMPOZY_HOME/agents` and `$COMPOZY_HOME/profiles/<profile>/agents`. What remains is yours to edit before you upgrade:

- **AGENT.md files in a repository, a workspace profile, an additional root, or an extension.** Change `kind: stays_on` to `kind: always_on`. A file that still says `stays_on` stops loading: the agent is left out of the workspace's agent list, and the workspace agent diagnostics name the file with "agent kind must be on_demand or always_on (stays_on was renamed to always_on in v0.5.1)".
- **`config.toml`.** Rename `[agents.stays_on]` to `[agents.always_on]`. The old table now fails config load, and the daemon does not start: `removed config key "agents.stays_on" in "<file>": use "agents.always_on"`.
- **Scripts and integrations.** `--kind stays_on` and request `kind: "stays_on"` (create, update, duplicate and its `overrides.kind`) fail with `agent_kind_invalid` and the same message. `compozy__agent_create` lists only `on_demand` and `always_on` in its input schema, so `kind: "stays_on"` fails input validation on `$.kind`. `compozy config get|set|unset` and `compozy__config_*` reject `agents.stays_on.*` paths with `config: removed config path "agents.stays_on.<key>": use "agents.always_on.<key>"`. The generated TypeScript and Go request types narrow to `"on_demand" | "always_on"`.
- **Upgrading straight from v0.5.0.** This release skips the v0.5.1 boot rewrite, so daemon-owned AGENT.md files are not rewritten either. Edit `kind: stays_on` to `kind: always_on` in `$COMPOZY_HOME/agents` and `$COMPOZY_HOME/profiles/<profile>/agents` too, or upgrade to v0.5.1 first and start the daemon once.

Stored rows need nothing: the v0.5.1 migration already moved `definition_not_stays_on` reasons and agent resource records to the new values.
