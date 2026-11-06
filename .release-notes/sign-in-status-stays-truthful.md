---
title: Provider sign-in status stays accurate
type: fix
---

The sign-in badge for CLI providers could be wrong in a few cases. It now reports what is actually true.

- **After a restart.** Sign-in used to show as unverified until a session started or you clicked **Check sign-in**. The daemon now checks each installed CLI provider once in the background after it starts.
- **When the CLI is broken.** A saved **Ready** no longer hides a CLI that can't be found or run.
- **With a different account folder.** A session that signs in with another `HOME`, `CLAUDE_CONFIG_DIR` or `CODEX_HOME` no longer marks the default account as Ready.
- **After a timeout.** A check that timed out is no longer recorded as **Needs login**.
- **Ready looks ready.** The Ready dot in Settings now uses the success color.
