---
title: Daemon-based architecture
type: breaking
---

Compozy now runs every task, review, and exec workflow through a long-lived, home-scoped daemon at `~/.compozy/`. The daemon owns runtime state in SQLite (`~/.compozy/db/global.db` plus per-run `run.db`), exposes a UDS + HTTP API, and supports re-attach and observe across separate CLI invocations. Existing scripts that called `compozy start` or `compozy fix-reviews` need updates — most legacy top-level commands are gone or moved under the new `tasks`, `reviews`, `runs`, and `daemon` groups. The CLI auto-starts the daemon on first invocation, so most users do not need to start it explicitly.

### What's new

- `compozy daemon start | status | stop` lifecycle commands, plus `--foreground` for attached runs and `--web-dev-proxy <url>` for proxying a frontend dev origin through the daemon HTTP transport.
- `compozy workspaces list | show | register | unregister | resolve` workspace registry — workspaces are registered lazily on first use, or explicitly via `register`.
- `compozy tasks run <slug>` daemon-backed workflow runner with `--attach auto|ui|stream|detach`, `--ui`, `--stream`, `--detach`, and `--task-runtime` overrides.
- `compozy reviews fetch | list | show | fix` review command family (replaces the old top-level `fix-reviews` / `fetch-reviews`).
- `compozy runs attach <run-id> | watch <run-id> | purge` for re-attaching to live runs and pruning terminal artifacts.
- `--format text|json` flag on operator/daemon commands for machine-readable output.
- New durable stores: `~/.compozy/db/global.db` (workspaces, runs index) and per-run `~/.compozy/runs/<run-id>/run.db`.
- `compozy migrate` is now **required** before daemon-backed commands run on legacy projects (it also infers task types — see the dedicated note).

### Breaking changes

| Area                    | Before                                   | After                                                                                           |
| ----------------------- | ---------------------------------------- | ----------------------------------------------------------------------------------------------- |
| Workflow run            | `compozy start --name <slug>`            | `compozy tasks run <slug>` (top-level `start` is removed)                                       |
| Review fix              | `compozy fix-reviews`                    | `compozy reviews fix` (top-level `fix-reviews` kept as alias)                                   |
| Review fetch            | `compozy fetch-reviews`                  | `compozy reviews fetch` (top-level `fetch-reviews` kept as alias)                               |
| Per-task runtime config | `[[start.task_runtime_rules]]`           | `[[tasks.run.task_runtime_rules]]` (TOML), or `--task-runtime` (CLI/TUI)                        |
| Runtime artifacts       | `<workspace>/.compozy/runs/<run-id>/`    | `~/.compozy/runs/<run-id>/` (now includes durable `run.db`)                                     |
| Sync semantics          | `compozy sync` regenerated `_meta.md`    | Reconciles workflow state into `global.db`; one-time cleanup of legacy `_meta.md` / `_tasks.md` |
| Preflight               | `compozy start` skill check              | `tasks run` and `reviews fix` block on missing skill installs                                   |
| Public Go API           | File-based readers in `pkg/compozy/runs` | Daemon-transport readers; signature changes in `Run`, `watch`, `tail`, `replay`                 |
| Migrate                 | Recommended                              | **Required** before any daemon-backed workflow command on legacy projects                       |

### New daemon workflow

```bash
# Lifecycle (most users do not need explicit start; tasks/reviews auto-start)
compozy daemon start                                     # detached, returns status
compozy daemon start --foreground                        # attached
compozy daemon start --foreground \
  --web-dev-proxy http://127.0.0.1:3000                  # for UI development
compozy daemon status --format json
compozy daemon stop --force                              # cancel runs, then stop

# Workspaces
compozy workspaces register .
compozy workspaces list --format json
compozy workspaces show <id-or-path>

# Run a workflow
compozy tasks run user-auth                              # auto-attach (TUI if interactive)
compozy tasks run user-auth --stream                     # textual stream
compozy tasks run user-auth --detach                     # fire-and-forget
compozy tasks run user-auth \
  --task-runtime type=frontend,ide=codex,model=gpt-5.5

# Reattach / observe / purge
compozy runs attach <run-id>
compozy runs watch  <run-id>
compozy runs purge

# Reviews
compozy reviews fetch user-auth --provider coderabbit --pr 42
compozy reviews list  user-auth
compozy reviews fix   user-auth --ide claude --concurrent 2 --batch-size 3
```

### Daemon lifecycle improvements

- `daemon start --foreground` runs the daemon attached to the current shell with structured logs.
- HTTP port defaults to OS-chosen (ephemeral) and is reported by `daemon status`. Pin it with `COMPOZY_DAEMON_HTTP_PORT=<n>`. Bind host is loopback-only (`127.0.0.1`) and non-loopback origins are rejected at the middleware layer.
- Attaching to a run that has already settled now falls back to streaming the persisted event log instead of erroring.
- `daemon stop` accepts `--force` to cancel owned runs before shutdown; otherwise it drains gracefully.

### Migration steps

1. Upgrade the binary. On first invocation the daemon creates `~/.compozy/{config.toml,agents,extensions,state,daemon,db,runs,logs,cache}`.
2. For legacy projects with XML-tagged artifacts: run `compozy migrate` once before any daemon-backed command.
3. Replace scripts:
   - `compozy start --name X` → `compozy tasks run X`
   - `compozy fix-reviews` → `compozy reviews fix` (alias still works)
   - `compozy fetch-reviews` → `compozy reviews fetch` (alias still works)
4. Update TOML: rename `[[start.task_runtime_rules]]` to `[[tasks.run.task_runtime_rules]]`. Move `id=` selectors to `--task-runtime` (TOML rejects `id=` rules).
5. Stop reading `<workspace>/.compozy/runs/` directly — runtime artifacts now live in `~/.compozy/runs/<run-id>/` and include a durable `run.db`. Use `pkg/compozy/runs` (daemon transport) or the daemon HTTP/UDS API.
6. Optional: set `COMPOZY_DAEMON_HTTP_PORT=<n>` to pin the HTTP port (`0` requests an ephemeral port). `COMPOZY_WEB_DEV_PROXY` mirrors `--web-dev-proxy`.
