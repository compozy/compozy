---
title: Compatibility shims scheduled for v0.6.1 are removed
type: fix
breaking: true
---

v0.6.0 was the last release that migrated or silently ignored the retired declarations below (#1518). v0.6.1 removes those compatibility shims (#1593). Old declarations now fail the normal validation for their file, and CompozyOS no longer rewrites the file. Current declarations behave as before.

## Migration notes

**Upgrade through v0.6.0 first** if you skipped it. Start v0.6.0 once with every user, profile, workspace and workspace-profile configuration in place so it migrates old files. Resolve any deferred-migration warning, then upgrade. Then remove anything below that is still left over. Existing archived comments, SQL migration history, memory files, transcripts and retained extension registrations are not deleted.

| Removed in v0.6.1 | Use instead |
| --- | --- |
| `kind: stays_on` in a persisted `AGENT.md`, decoded and rewritten on load | `kind: always_on` |
| `[agents.stays_on]` tables in `config.toml`, migrated on load | `[agents.always_on]` |
| Archiving retired memory and compaction settings from `config.toml` on start: `[memory]` and its sub-tables, `[roles.dream]`, `[roles.checkpoint_summary]`, `[roles.memory_extractor]`, `[roles.memory_controller]`, `[session.compaction]`, and `[[automation.triggers]]` with `event = "memory.consolidated"` | Delete them. They now fail configuration validation. |
| Ignoring the hook matcher keys `compaction_reason` and `compaction_strategy` in `AGENT.md` and `SKILL.md` | `compaction_trigger` in config, extension or skill hooks. `AGENT.md` hooks cannot target context events. |
| Dropping `memory.backend`, the `memory/recall`, `memory/store` and `memory/forget` methods, and the `memory.read` / `memory.write` consents from extension manifests | Remove them from the extension source and publish a new version. Installed packages are never edited. |
| Skipping extension automation resources triggered by `memory.consolidated` | Remove the resource. |
| Ignoring `memory_policy` in `SOUL.md` | Delete the field. It now gets the normal unsupported-field diagnostic. |
| Dropping retired memory tool IDs and the `compozy__memory` / `compozy__memory_admin` toolsets from agent, profile and session tool policies | Remove them. Retired toolsets fail catalog validation. Exact retired `compozy__memory_*` IDs are still accepted but match no tool, and an explicit allow-list stays restricted. |
| The `permissions.provider_full_access` key, a no-op with a warning in v0.6.0 | `permissions.mode` (`approve-all` for **Full access**). The old key is now unknown. |
| ACP usage aliases `cacheReadTokens` / `cacheWriteTokens` | ACP adapters must send `cachedReadTokens` / `cachedWriteTokens`. The aliases no longer count toward cache usage. |
| Automatic transfer of bundled `dev-cycle` extension state to `spec-cycle` | Migrate it yourself. The old state stays in place, and `compozy doctor` still reports it. |
| Take-over reclassifying process failures from launches canceled before v0.4.8 | Recover those sessions with v0.6.0 before upgrading. |

Marketplace source management (`compozy marketplace sources`) is still experimental. Only the expired "through v0.6.0" date was removed from its help text. The full upgrade steps are in the migration guide.
