---
title: Full access runs the provider unrestricted
type: fix
---

**Full access** (`approve-all`) now runs the provider at its unrestricted level, the level you picked. New Codex sessions start in `agent-full-access`, Claude Code in `bypassPermissions`, Google Antigravity in `yolo`, and GitHub Copilot CLI with `allow_all = on`. Before, Codex under Full access still started in its own Auto review mode unless `permissions.provider_full_access = true` was also set, a key the Settings app never showed. Codex then stopped work such as `git push` or `gh issue create` and asked you to approve each one.

**Auto-review** (`auto`) is unchanged and still selects the provider's reviewer mode. An agent that chooses its own narrower provider mode keeps it, and a session started at a narrower level never inherits the unrestricted mode.

Migration notes: this widens what Codex and Copilot sessions can do when they run at Full access, which is the default level. Unrestricted provider commands run with your user's access inside the CompozyOS workspace and operating-system boundaries. To keep the provider's reviewer, set **Settings → General → Permissions** to **Auto-review**, or set `permissions.mode = "auto"` or an agent's `permissions: auto`. The new mode applies when a session starts or resumes. `permissions.provider_full_access` is deprecated and has no effect: CompozyOS still loads it in v0.5.6 and logs a warning naming `permissions.mode`. Remove it from `config.toml`; v0.5.7 deletes the key and rejects it as unknown.
