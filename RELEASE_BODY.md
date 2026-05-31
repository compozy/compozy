## 0.2.8 - 2026-05-31

### 🐛 Bug Fixes

- Keep multi-run task timers ticking (#179)- Treat model auto as runtime default (#181)

### Release Notes

#### Features

##### Run multiple task workflows from one command
`compozy tasks run` now accepts a `--multiple` flag that enqueues several task workflows through a single daemon-owned parent run, with one TUI tab per requested slug. The single-workflow path (`compozy tasks run <slug>`) is unchanged — `--multiple` is purely additive, so existing habits, scripts, and CI invocations keep working byte-for-byte.

### Starting a multi-run

Pass one comma-separated slug list. The same runtime flags (`--ide`, `--model`, `--stream`, `--detach`, `--attach`, etc.) apply to every workflow in the batch:

```bash
# Enqueue two workflows with shared runtime defaults
compozy tasks run --multiple alpha,beta

# Pick the runtime once for the whole queue
compozy tasks run --multiple alpha,beta --ide codex --model gpt-5.5

# Stream or detach the parent queue
compozy tasks run --multiple alpha,beta --stream
compozy tasks run --multiple alpha,beta --detach
```

Use `--multiple` when one command should drive an ordered batch with identical flags. Keep using `tasks run <slug>` for a single workflow or for scripts that expect exactly one run ID per invocation.

`--multiple` cannot be combined with a positional slug or with `--name`.

### Scheduling mode

Scheduling is controlled by `[tasks.run] run_multiple_mode` in `.compozy/config.toml` or `~/.compozy/config.toml`:

```toml
[tasks.run]
run_multiple_mode = "enqueued"
```

| Mode         | Behavior                                                                                                                       |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------ |
| `"enqueued"` | **Default.** Runs one child workflow at a time, in the requested order. Later slugs stay queued until the active one finishes. |
| `"parallel"` | Accepted for forward-compatible config, but V1 prints a fallback message and still runs the queue enqueued.                    |

When unset, the built-in default is `"enqueued"`. The value is validated at config load: an empty string is rejected, and anything other than `"enqueued"` or `"parallel"` fails with a clear error.

Real parallel multi-run waits on git worktree isolation so concurrent agents never edit the same checkout simultaneously — that lands in V2. Configuring `"parallel"` today is safe: Compozy explains the fallback and proceeds in enqueued order.

### Tabbed TUI

The task-run TUI shows one tab per requested slug:

- Queued tabs appear before their child run exists.
- The running tab shows the familiar single-task surface.
- Completed, failed, and canceled tabs stay available for inspection.

The quit dialog applies to the parent queue:

- **Close TUI** — detaches and leaves the queue running in the daemon.
- **Stop Run** — cancels the parent queue, cancels the active child, and marks queued workflows canceled.
- **Cancel** — returns to the TUI without changing execution.

### Notes

- The multi-run path is daemon-owned end to end: a new parent run orchestrates independent child task runs over the same snapshot-plus-stream transport used by single runs.
- `--dry-run` still previews prompts without executing.