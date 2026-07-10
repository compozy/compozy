---
title: GPT-5.6, Claude Fable 5, and ACP session model negotiation
type: feature
---

Compozy now negotiates model, reasoning, and permission mode from the options advertised by ACP `session/new` / `session/load`, and applies the resolved configuration **before the first prompt** on both new and resumed sessions.

### New models and defaults

- **Codex / Droid** default to `gpt-5.6-sol`. Supported GPT-5.6 IDs include `gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna` when the installed adapter advertises them.
- **Claude Fable 5** accepts `--model fable`, `fable-5`, or `claude-fable-5`. Fable always uses Claude's `auto` permission mode (never `bypassPermissions`), even when `--access-mode full` was requested.
- **Cursor** model names resolve against its ACP catalog (for example `--model grok-4.5` → the advertised catalog ID). Cursor does not get a separate reasoning option when the session does not advertise one.

### Reasoning effort

`max` and `ultra` are now first-class reasoning levels in the CLI forms, setup wizard, and recovery flags. Claude's `max` is an advertised ACP effort value; if a requested effort is not advertised, Compozy stops before the prompt and lists the valid choices.

### Codex ACP adapter

The preferred Codex ACP package is now `@agentclientprotocol/codex-acp` (GPT-5.6 and `max`/`ultra` need `>= 1.1.2`). The legacy `@zed-industries/codex-acp` path remains only for older combinations such as GPT-5.5 with reasoning through `xhigh`.
