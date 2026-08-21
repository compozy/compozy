---
title: The command palette runs CompozyOS
type: feature
---

⌘K (or ⌘⇧P) opens a palette the daemon owns. Every command — shell actions, window and desktop moves, domain views, settings, and extension contributions — is registered once in the runtime and projected to every surface, so the web app, the desktop shell, the CLI, HTTP, UDS, and native tools read the same catalog with the same availability truth. (#441)

- A row is available or it is not, and the daemon says why. A command that needs an attached shell reports `requires an attached shell` in the row itself instead of failing after you press Enter.
- Commands that need input collect it inline as typed arguments — `text`, `password`, `dropdown`, `checkbox` — before anything runs. A destructive command carries its own confirmation title and confirm verb.
- Execution is single-flight per command: a second invocation while one is still running returns `already_running` until the first reaches a terminal result, so a double Enter cannot run something twice.
- A destructive command goes through the existing tool-approval path and returns `approval_pending` with a stable approval ID. `compozy approvals show|cancel <id>` follows or ends it; approve runs exactly once, deny or cancel ends it with no effect.
- A command that acts on a shell targets one attached client. With a single attachment it auto-selects; with several it asks for an explicit client and lists every attachment ID instead of guessing.
- ⌘K on a selected row opens a filterable action panel anchored to that row: the runnable action plus Pin, Set alias, and Set shortcut. Unavailable rows expose only those meta-actions and the daemon's reason.
- The catalog is live. Installing an extension, changing a binding, or pinning from another window updates open palettes without a reload.

```bash
compozy cmd-palette list --available=false -o json   # every command with the daemon's own reason
compozy cmd-palette inspect session.new -o json      # action, arguments, execution policy, risk
compozy cmd-palette clients -o json                  # the targeting source of truth
```
