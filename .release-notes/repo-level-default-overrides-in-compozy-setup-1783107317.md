---
title: Repo-level default overrides in compozy setup
type: feature
---

`compozy setup` can now save repo-level runtime defaults for you, so you no longer have to hand-edit `.compozy/config.toml` to change the built-in defaults for a project.

### What changed

After skill installation, interactive setup offers an optional step to override the built-in runtime defaults and persist them to the workspace config:

```toml
[defaults]
ide = "..."
model = "..."
reasoning_effort = "..."
```

- The IDE is chosen from a dropdown of supported runtimes, and the recommended model comes from the runtime registry's per-IDE default.
- The override prompt defaults to **No** — opt out and you keep the built-in defaults with no config change.
- Defaults are only written after setup completes with zero failures, and existing config sections are preserved on write.

### Notes

- The write is workspace-scoped: it lands in the repo's `.compozy/config.toml`.
- `compozy setup --global` does not prompt for repo defaults and never modifies workspace config.
- If your repo already has a custom model, setup preserves it; if the current model still matches the previously recommended one, changing the IDE updates the recommendation to the new IDE's default.
