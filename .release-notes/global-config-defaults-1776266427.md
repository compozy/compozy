---
title: Global config defaults
type: feature
---

Set personal defaults once in `~/.compozy/config.toml` and have them apply across every project. Project-level `.compozy/config.toml` always takes precedence, so teams keep control while individuals stop repeating themselves.

### Example

```toml
# ~/.compozy/config.toml  (global — applies to all projects)
[defaults]
ide = "claude"
model = "sonnet"
access_mode = "default"
auto_commit = true

[sound]
enabled = true
on_completed = "glass"
on_failed = "basso"

[exec]
model = "gpt-5.4"
verbose = true
```

```toml
# .compozy/config.toml  (project — overrides global)
[defaults]
model = "o4-mini"

[start]
include_completed = true
```

With both files above the effective config resolves to:

| Field                     | Value       | Source       |
| ------------------------- | ----------- | ------------ |
| `defaults.ide`            | `"claude"`  | global       |
| `defaults.model`          | `"o4-mini"` | project wins |
| `defaults.auto_commit`    | `true`      | global       |
| `sound.enabled`           | `true`      | global       |
| `exec.model`              | `"gpt-5.4"` | global       |
| `start.include_completed` | `true`      | project      |

All sections supported in project config (`[defaults]`, `[start]`, `[exec]`, `[fix_reviews]`, `[fetch_reviews]`, `[tasks]`, `[sound]`) work in the global file with the same schema.
