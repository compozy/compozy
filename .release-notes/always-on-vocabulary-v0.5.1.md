---
title: Stays on is now Always-on in v0.5.1
type: highlight
---

The agent kind that keeps running on its own is now called **Always-on**, and its value is `always_on`. Every CompozyOS surface uses the new name: the app, the CLI, `config.toml`, HTTP/UDS, native tools, the SDKs, the official skill, and the docs. On-demand agents are unchanged. The app now also uses the labels other agent tools use for permission levels and agent setup:

- Permission levels: **Accept edits** (`approve-edits`, was "Edit on its own"), **Auto-review** (`auto`, was "Let a reviewer decide"), and **Full access** (`approve-all`, was "Never ask"). Wire values are unchanged.
- The agent page section is **Setup**: Instructions, Model, Heartbeat, Memory, Tools, and Permissions. A scheduled check now reads "heartbeat". Page URLs are unchanged.

### Migration notes

v0.5.1 upgrades in place: v0.5.0 files, config, request values, and stored rows keep working through v0.5.1. Readers of emitted `kind`, reason, and error values must update now (see below). Every alias below is removed in **v0.5.2**.

- **Update the daemon together with the CLI.** The CLI doesn't check the daemon's version. A v0.5.1 CLI always sends `always_on`, which a v0.5.0 daemon rejects.
- **AGENT.md `kind: stays_on` → `kind: always_on`.** The old value loads as `always_on` through v0.5.1, and the daemon logs one warning per file. On boot, the daemon rewrites the `kind` value of every AGENT.md under `$COMPOZY_HOME/agents` and `$COMPOZY_HOME/profiles/<profile>/agents`. It changes no other byte and logs each rewrite and each skip. AGENT.md files in a repository, a workspace profile, or an extension are never rewritten, so edit those yourself before v0.5.2.
- **`[agents.stays_on]` → `[agents.always_on]`.** The old table is read with one warning per process, and a key set in `[agents.always_on]` wins. `compozy config get|set|unset` and `compozy__config_*` accept `agents.stays_on.*` paths, warn, and operate on `agents.always_on.*`. `unset` through either spelling clears the key from both tables in the file it edits. Defaults are unchanged.
- **`--kind stays_on` → `--kind always_on`.** The CLI accepts the old value with a stderr warning.
- **Request enums vs. emitted values.** Request `kind` fields (create, update, duplicate, its `overrides.kind`, and `compozy__agent_create`) accept `on_demand`, `always_on`, or the deprecated `stays_on`. The generated TypeScript and Go request types widen to match. Responses, events, tool results, and CLI `-o json` emit `always_on` from this release on, so readers that compare `kind` to `stays_on` break now.
- **`definition_not_stays_on` → `definition_not_always_on`.** This changes at once in the instance API `pause_reason` field, the instance check `gate` (`compozy agent instance check` and `compozy__agent_instance_check`), the check-in history `reason`, and the `agent.instance.*` hook payload `reason`. Stored rows are migrated. Hook consumers that compare the old value break.
- **`agent_not_stays_on` → `agent_not_always_on`.** The error code changes at once. The experimental agent memory routes break with no compatibility window.
- **Startup section `stays_on_doctrine` → `always_on_doctrine`.** The tag is now `<compozy-always-on-doctrine>`. Startup-section diagnostics show the new name.
- **Permission labels.** Only the labels changed. `deny-all`, `approve-reads`, `approve-edits`, `auto`, and `approve-all` are unchanged in files, config, and the API.
- **v0.5.2 removes every shim.** That covers the `stays_on` kind alias, `[agents.stays_on]`, the `agents.stays_on.*` paths, the `stays_on` request enum value, and the boot rewrite. After that, `stays_on` fails with an error that names `always_on`.
