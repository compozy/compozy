---
title: CompozyOS starts again with settings v0.6.0 still accepted
type: fix
---

Updating to v0.6.1 could leave the desktop app on "CompozyOS could not start". v0.6.1 treated several settings as errors that v0.6.0 still accepted, and the only way out was to edit files by hand.

CompozyOS now starts with those settings present and handles each one for you:

- **`permissions.provider_full_access` in `config.toml`.** The key stopped doing anything in v0.6.0. CompozyOS removes that line, changes nothing else, and keeps the original next to the file as `config.toml.retired-keys.bak`.
- **Retired memory and compaction settings in `config.toml`.** `[memory]`, the Dream, checkpoint summary, memory extractor and memory controller roles, `[session.compaction]`, `memory.consolidated` automation triggers and the `compaction_reason` / `compaction_strategy` hook matchers move to a commented block at the end of the same file. The values are kept and no longer apply.
- **The earlier always-on agent settings.** A config table or an `AGENT.md` that still uses the kind name from before v0.5.1 is updated to `always_on` in place.
- **`compaction_reason` and `compaction_strategy` in `AGENT.md` hooks.** The two keys are ignored and the file is left as it is.

This covers the global, profile and workspace files, including a workspace config you pull after updating. If a file cannot be rewritten, for example in a read-only folder, CompozyOS ignores the retired settings, starts anyway, and tries again on the next start.

An installed extension whose packaged resources no longer load is now skipped with a warning in the log. Your other extensions and CompozyOS itself start normally.

Migration notes: no action needed. `permissions.mode` still selects the provider access level, and `compaction_trigger` replaces the two retired hook matchers. CompozyOS stops accepting these settings in v0.7.0, so let it rewrite your files, or update the ones it could not rewrite, before then.
