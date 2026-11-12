---
id: ET-cli-agent-kind-removal
title: Create always-on agents after removing retired kind inputs
area: ET
qa_status: untested
last_report: reports/2026-11-04-agent-kind-removal.md
---

As a CLI operator upgrading beyond v0.5.2:

1. Create `triage` with `compozy agent create triage --kind always_on --prompt ...`, create its instance, then inspect `compozy agent info triage -o json`. Confirm `kind: always_on` using a fresh public API read.
2. Run create/update/duplicate with the retired kind. Confirm exit 1 and the actionable `agent_kind_invalid` error.
3. Upgrade from v0.6.0 (#1618): seed an AGENT.md with the retired kind in each discovery root (global, profile, workspace, workspace profile, additional). Start the runtime and confirm it becomes ready, each definition loads as `always_on`, and a writable file changes only its kind value. Repeat with a read-only file and with a layout the rewrite does not support (a YAML anchor on the kind value): the definition still loads as `always_on`, the file is byte-identical, and the log carries the deferred-migration warning. A packaged definition loads as `always_on` and is never edited.
4. Upgrade from v0.6.0 (#1618): load the retired agent-kind config table from each config scope. Confirm the runtime starts, the values apply under `agents.always_on` with canonical keys winning conflicts, the original values stay in the file as inactive comments, and a second load changes nothing. Try retired get/set/unset paths and confirm errors naming `agents.always_on`. Canonical definitions and config still load.

The v0.5.1 database migration remains. v0.6.1 rejected the persisted files in steps 3 and 4, which could stop the runtime from starting; the persisted-file migration is restored as one shim generation and is removed in v0.7.0. Current-head CI owns automated verification (`TestAgentKindAliasLoad`, `TestPersistedAgentKindDeferredMigration`, `TestPersistedAgentKindConfigMigration`, `TestAgentKindAliasBootIntegration`); the live upgrade walk for steps 3 and 4 is unexecuted.

Walked 2026-11-04: canonical creation/instance/info and retired create/table/config paths passed.
Workspace load failures are visible in the public agent catalog diagnostics; the ordinary list omits invalid definitions.
The HTTP/UDS owning suite covers update and both duplicate kind locations.
