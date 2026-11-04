---
title: Provider usage for extra accounts and verified native CLI sign-ins
type: fix
---

Two fixes for running more than one native CLI account.

- **Usage for custom providers.** A custom provider that points at a built-in, such as a second Claude account with `runtime_provider = "claude"` and `auth_mode = "native_cli"`, used to show `unsupported` / `no_collector` in Provider usage. It now uses the matching collector and shows as its own row. Usage collection also applies an `env NAME=value … cmd` launch prefix, so each account reads its own config folder, such as `CLAUDE_CONFIG_DIR=~/.claude-mm1`, instead of mixing with your main account.
- **"Sign-in unverified" no longer sticks.** Settings › Providers and `compozy provider list` showed "Sign-in unverified" for every `native_cli` provider, even right after a sign-in check passed. The daemon now remembers the last result from `compozy provider auth status`, from **Check sign-in**, and from session starts, and shows when it checked. **Check sign-in** now checks in place instead of opening the editor. Changing the provider's config, or restarting the daemon, clears the remembered result.
