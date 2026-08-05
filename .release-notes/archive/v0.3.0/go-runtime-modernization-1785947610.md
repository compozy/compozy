---
title: Runtime hardening and secret-safe provider login
type: breaking
---

A broad modernization pass across the Go runtime tightened lifecycle and cleanup ownership, ID allocation, task settlement, filesystem confinement, and streaming framing. Most of it is invisible, but it lands several deliberate cuts that change what operators, scripts, and agents see. (#293)

- `providers.<id>.auth_login_command` is now write-only. You can still set it through `config.toml`, `compozy config set`, or `compozy__config_set`, but no read surface returns it. `compozy config show|list|get|diff`, provider status, doctor, Settings, HTTP, and UDS return a safe `login` descriptor instead: whether it is configured, its source, the executable basename, whether that executable is present, and a recommended action.
- Every per-session event database carries an immutable owner and physical identity. A database that was copied between sessions or workspaces is refused before any migration or mutation — no adoption, no rebinding, no automatic repair. The operator recovery path is documented under "Session event store ownership".
- Markdown files under `<workspace>/knowledge/` are injected as a bounded workspace knowledge snapshot before each accepted turn, including task, task-creator, and Heartbeat wakes. It is prompt context for the turn, not durable memory.
- Installing an extension from Git now requires Git 2.37 or newer and reports `extension_git_version_unsupported` when it is older. Git sources must be HTTPS and resolve to public addresses.

Migration notes: `compozy provider auth login --print-command` is removed. The config key `memory.recall.signals.metrics_enabled` is removed with no alias. The task-notification native tools take `workspace_id` instead of `workspace`, and the old input is not an alias. Notification cursor identity and `delivery_id` are now opaque values that must be echoed back byte for byte.
