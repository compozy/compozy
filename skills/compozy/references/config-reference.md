# Configuration Reference

Complete reference for `.compozy/config.toml` workspace configuration.

## File Location

Place the configuration file at `.compozy/config.toml` in the workspace root. CLI flags always override config values.

## Sections

### `[defaults]`

Runtime defaults applied to all commands unless overridden.

| Field | Type | Description |
| --- | --- | --- |
| `ide` | string | ACP runtime: `claude`, `codex`, `copilot`, `cursor-agent`, `droid`, `gemini`, `opencode`, `pi` |
| `model` | string | Model override. Per-IDE defaults: codex/droid=gpt-5.5, claude=opus, copilot=claude-sonnet-4.6, cursor-agent=composer-1, opencode/pi=anthropic/claude-opus-4-6, gemini=gemini-2.5-pro |
| `output_format` | string | Output format: `text`, `json`, `raw-json` |
| `reasoning_effort` | string | Reasoning effort level: `low`, `medium`, `high`, `xhigh` |
| `access_mode` | string | Access mode: `default`, `full` |
| `timeout` | string | Execution timeout in Go duration format (e.g., `30m`, `1h`) |
| `tail_lines` | int | Number of tail lines to display from agent output |
| `add_dirs` | string[] | Additional directories for ACP runtimes (claude and codex only) |
| `auto_commit` | bool | Include automatic commit instructions at task/batch completion |
| `max_retries` | int | Maximum number of retries on agent failure or inactivity timeout (`0` disables automatic retries) |
| `retry_backoff_multiplier` | float | Backoff multiplier between retries |

### `[tasks.run]`

Options specific to `compozy tasks run`.

| Field | Type | Description |
| --- | --- | --- |
| `include_completed` | bool | Include tasks already marked as completed |
| `task_runtime_rules` | `array<table>` | Type-scoped runtime overrides applied after `[defaults]` for `compozy tasks run` |

#### `[[tasks.run.task_runtime_rules]]`

Per-task runtime rules let `compozy tasks run` change the runtime for tasks that match a given task `type`. This v1 config surface is intentionally bulk-oriented: config supports `type` selectors only, while one-off task `id` overrides are available from the CLI and TUI for the current run.

| Field | Type | Description |
| --- | --- | --- |
| `type` | string | Task type selector such as `frontend`, `backend`, or any custom type from `[tasks].types` |
| `ide` | string | Runtime override for matching tasks |
| `model` | string | Model override for matching tasks |
| `reasoning_effort` | string | Reasoning effort override: `low`, `medium`, `high`, `xhigh` |

Rules are applied in declaration order within config, with later rules for the same `type` replacing earlier ones when workspace and global config are merged. At execution time, the effective precedence is:

1. Base runtime from `[defaults]` and `[tasks.run]`
2. Config `[[tasks.run.task_runtime_rules]]` matching the task `type`
3. CLI or TUI `type` rules for the current run
4. CLI or TUI `id` rules for the current run

Example:

```toml
[defaults]
ide = "codex"
model = "gpt-5.5"
reasoning_effort = "medium"

[[tasks.run.task_runtime_rules]]
type = "frontend"
model = "gpt-5.5"
reasoning_effort = "high"

[[tasks.run.task_runtime_rules]]
type = "docs"
ide = "claude"
model = "opus"
```

### `[tasks]`

Task type registry.

| Field | Type | Description |
| --- | --- | --- |
| `types` | string[] | Allowed task types. Default: `["frontend", "backend", "docs", "test", "infra", "refactor", "chore", "bugfix"]` |

### `[fix_reviews]`

Options specific to `compozy reviews fix`.

| Field | Type | Description |
| --- | --- | --- |
| `concurrent` | int | Number of batches to process in parallel (1-10) |
| `batch_size` | int | Number of file groups per batch (1-50) |
| `include_resolved` | bool | Include already-resolved review issues |

### `[fetch_reviews]`

Options specific to `compozy reviews fetch`.

| Field | Type | Description |
| --- | --- | --- |
| `provider` | string | Default review provider (e.g., `coderabbit`) |
| `nitpicks` | bool | Enable or disable CodeRabbit review-body comments (`nitpick`, `minor`, and `major`). Default is enabled when unset |

### `[watch_reviews]`

Options specific to `compozy reviews watch`. Watch-specific loop values come from this section; child review
fetch/fix defaults continue to come from `[fetch_reviews]`, `[fix_reviews]`, and `[defaults]`.

| Field | Type | Description |
| --- | --- | --- |
| `max_rounds` | int | Maximum watch rounds. Must be greater than zero when `until_clean = true`. |
| `poll_interval` | string | Positive Go duration between provider status checks (e.g., `30s`). |
| `review_timeout` | string | Positive Go duration to wait for the provider to review a PR head (e.g., `30m`). |
| `quiet_period` | string | Positive Go duration to wait after the latest provider review/status signal before fetching issues. |
| `auto_push` | bool | Push committed fixes after each successful round. Requires `defaults.auto_commit = true` when enabled from config. |
| `until_clean` | bool | Continue until the provider has reviewed the current PR head and no actionable issues are fetched. |
| `push_remote` | string | Optional push remote. Must be set together with `push_branch`; omit both to resolve upstream later. |
| `push_branch` | string | Optional push branch. Must be set together with `push_remote`; omit both to resolve upstream later. |

CLI usage:

```bash
compozy reviews watch --provider coderabbit --pr 123
compozy reviews watch my-workflow --provider coderabbit --pr 123 --max-rounds 4
compozy reviews watch my-workflow --provider coderabbit --pr 123 --auto-push --push-remote origin --push-branch feature/reviews
```

Watch runs are daemon-owned parent runs. They wait until the provider reports a completed, settled review for the
current PR head, fetch actionable issues, launch child `reviews fix` runs, optionally push the child commit, then
repeat until the provider-current fetch is clean or `max_rounds` is reached. For CodeRabbit, settled means the latest
CodeRabbit commit status for the PR head is complete; transient "review in progress" statuses keep the watch waiting.

Auto-push safety:

- `auto_push = true` requires committed fixes. The watch parent forces child `auto_commit = true`; an explicit
  `runtime_overrides.auto_commit = false` is rejected.
- The daemon never restores, resets, cleans, or manually stages unrelated work. It inspects git state and runs only
  `git push <remote> HEAD:<branch>` for committed branch state. It pushes after a child fix run completed, all fetched
  issues are resolved, and `HEAD` advanced; when `auto_push` starts with existing unpushed commits, it first pushes
  that committed `HEAD` so the provider can review the same head the watch is waiting on.
- `push_remote` and `push_branch` can be supplied by config/CLI or resolved from the current upstream. Missing push
  target information fails before pushing.

Lifecycle events emitted by the parent run:

| Event | Purpose |
| --- | --- |
| `review.watch_started` | Initial provider/PR/workflow/git state, including `head_sha`, `remote`, `branch`, `dirty`, and `unpushed_commits`. |
| `review.watch_waiting` | Provider has not yet reviewed the current PR head; includes `status`, `review_id`, and `review_state` when available. |
| `review.watch_round_fetched` | A provider-current round produced actionable issues; includes `round`, `total`, `resolved`, and `unresolved`. |
| `review.watch_fix_started` | Child `reviews fix` run started; includes `child_run_id`. |
| `review.watch_fix_completed` | Child run reached a terminal state; includes `status` and `error` when failed. |
| `review.watch_push_started` / `review.watch_push_completed` / `review.watch_push_failed` | Auto-push lifecycle with `head_sha`, `remote`, `branch`, and `error` on failure. Startup reconciliation uses `round = 0`. |
| `review.watch_clean` | Provider-current fetch returned no actionable issues. |
| `review.watch_max_rounds` | The parent stopped because `max_rounds` was reached before a clean result. |

Extension hooks exposed to Go and TypeScript SDKs:

| Hook | Mutable | Payload highlights | Allowed patch fields |
| --- | --- | --- | --- |
| `review.watch_pre_round` | yes | `run_id`, `provider`, `pr`, `workflow`, `round`, `head_sha`, `review_id`, `review_state`, `status`, `nitpicks`, `runtime_overrides`, `batching`, `continue` | `nitpicks`, `runtime_overrides`, `batching`, `continue`, `stop_reason` |
| `review.watch_post_round` | no | `run_id`, `provider`, `pr`, `workflow`, `round`, `child_run_id`, `status`, `total`, `resolved`, `unresolved`, `pushed`, `stop_reason`, `error` | none |
| `review.watch_pre_push` | yes | `run_id`, `provider`, `pr`, `workflow`, `round`, `head_sha`, `remote`, `branch`, `push` (`round = 0` for startup reconciliation) | `remote`, `branch`, `push`, `stop_reason` |
| `review.watch_finished` | no | `run_id`, `child_run_id`, `provider`, `pr`, `workflow`, `round`, `head_sha`, `status`, `terminal_reason`, `stopped`, `clean`, `max_rounds`, `error` | none |

Hooks may veto a round or push only by returning `continue = false` or `push = false` with a non-empty
`stop_reason`. Hooks cannot mark a PR clean, skip provider-current detection, or mutate immutable provider/head/status
fields.

### `[exec]`

Options specific to `compozy exec`. Inherits all `[defaults]` fields plus:

| Field | Type | Description |
| --- | --- | --- |
| `verbose` | bool | Emit operational runtime logs to stderr |
| `tui` | bool | Open the interactive TUI |
| `persist` | bool | Save artifacts under `~/.compozy/runs/<run-id>/` |

### `[sound]`

Optional audio notifications that play when a run reaches a terminal state. Applies to both `compozy tasks run` and `compozy exec`. Disabled by default — setting any field without `enabled = true` is a no-op.

| Field | Type | Description |
| --- | --- | --- |
| `enabled` | bool | Master switch. Default `false`. |
| `on_completed` | string | Preset name or absolute path played on `run.completed`. Default `glass` when `enabled = true`. |
| `on_failed` | string | Preset name or absolute path played on `run.failed` and `run.cancelled`. Default `basso` when `enabled = true`. |

**Presets** (resolve to platform-native files at play time):

| Preset | macOS | Linux (freedesktop) | Windows |
| --- | --- | --- | --- |
| `glass` | `/System/Library/Sounds/Glass.aiff` | `complete.oga` | `tada.wav` |
| `basso` | `/System/Library/Sounds/Basso.aiff` | `dialog-error.oga` | `chord.wav` |
| `ping` | `Ping.aiff` | `message.oga` | `ding.wav` |
| `hero` | `Hero.aiff` | `complete.oga` | `tada.wav` |
| `funk` | `Funk.aiff` | `bell.oga` | `notify.wav` |
| `tink` | `Tink.aiff` | `message.oga` | `chimes.wav` |
| `submarine` | `Submarine.aiff` | `bell.oga` | `Ring01.wav` |

**Absolute paths** bypass preset lookup, so any local sound file works:

```toml
[sound]
enabled = true
on_completed = "/System/Library/Sounds/Hero.aiff"
on_failed = "/Users/me/sounds/custom-fail.wav"
```

**Platform requirements**: `afplay` (bundled with macOS), `paplay` (Linux, from `pulseaudio-utils`), or `powershell` + `System.Media.SoundPlayer` (Windows). On unix variants without one of these tools the feature silently falls back to no-op; playback errors never break a run.

## Complete Example

```toml
[defaults]
ide = "claude"
model = "opus"
reasoning_effort = "high"
auto_commit = true
add_dirs = ["../shared-lib", "../docs"]
timeout = "45m"
max_retries = 2
retry_backoff_multiplier = 1.5

[tasks]
types = ["frontend", "backend", "docs", "test", "infra", "refactor", "chore", "bugfix"]

[tasks.run]
include_completed = false

[fix_reviews]
concurrent = 2
batch_size = 3
include_resolved = false

[fetch_reviews]
provider = "coderabbit"
nitpicks = false

[watch_reviews]
until_clean = true
max_rounds = 6
poll_interval = "30s"
review_timeout = "30m"
quiet_period = "20s"
auto_push = false

[exec]
verbose = false
tui = false
persist = false

[sound]
enabled = true
on_completed = "glass"
on_failed = "basso"
```
