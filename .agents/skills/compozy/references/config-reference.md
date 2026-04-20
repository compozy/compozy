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
| `model` | string | Model override. Per-IDE defaults: codex/droid=gpt-5.4, claude=opus, copilot=claude-sonnet-4.6, cursor-agent=composer-1, opencode/pi=anthropic/claude-opus-4-6, gemini=gemini-2.5-pro |
| `output_format` | string | Output format: `text`, `json`, `raw-json` |
| `reasoning_effort` | string | Reasoning effort level: `low`, `medium`, `high`, `xhigh` |
| `access_mode` | string | Access mode: `default`, `full` |
| `timeout` | string | Execution timeout in Go duration format (e.g., `30m`, `1h`) |
| `tail_lines` | int | Number of tail lines to display from agent output |
| `add_dirs` | string[] | Additional directories for ACP runtimes (claude and codex only) |
| `auto_commit` | bool | Include automatic commit instructions at task/batch completion |
| `max_retries` | int | Maximum number of retries on agent failure or inactivity timeout (`0` disables automatic retries) |
| `retry_backoff_multiplier` | float | Backoff multiplier between retries |

### `[start]`

Options specific to `compozy tasks run`.

| Field | Type | Description |
| --- | --- | --- |
| `include_completed` | bool | Include tasks already marked as completed |
| `task_runtime_rules` | `array<table>` | Type-scoped runtime overrides applied after `[defaults]` for `compozy tasks run` |

#### `[[start.task_runtime_rules]]`

Per-task runtime rules let `compozy tasks run` change the runtime for tasks that match a given task `type`. This v1 config surface is intentionally bulk-oriented: config supports `type` selectors only, while one-off task `id` overrides are available from the CLI and TUI for the current run.

| Field | Type | Description |
| --- | --- | --- |
| `type` | string | Task type selector such as `frontend`, `backend`, or any custom type from `[tasks].types` |
| `ide` | string | Runtime override for matching tasks |
| `model` | string | Model override for matching tasks |
| `reasoning_effort` | string | Reasoning effort override: `low`, `medium`, `high`, `xhigh` |

Rules are applied in declaration order within config, with later rules for the same `type` replacing earlier ones when workspace and global config are merged. At execution time, the effective precedence is:

1. Base runtime from `[defaults]` and `[start]`
2. Config `[[start.task_runtime_rules]]` matching the task `type`
3. CLI or TUI `type` rules for the current run
4. CLI or TUI `id` rules for the current run

Example:

```toml
[defaults]
ide = "codex"
model = "gpt-5.4"
reasoning_effort = "medium"

[[start.task_runtime_rules]]
type = "frontend"
model = "gpt-5.4"
reasoning_effort = "high"

[[start.task_runtime_rules]]
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

### `[exec]`

Options specific to `compozy exec`. Inherits all `[defaults]` fields plus:

| Field | Type | Description |
| --- | --- | --- |
| `verbose` | bool | Emit operational runtime logs to stderr |
| `tui` | bool | Open the interactive TUI |
| `persist` | bool | Save artifacts under `.compozy/runs/<run-id>/` |

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

[start]
include_completed = false

[tasks]
types = ["frontend", "backend", "docs", "test", "infra", "refactor", "chore", "bugfix"]

[fix_reviews]
concurrent = 2
batch_size = 3
include_resolved = false

[fetch_reviews]
provider = "coderabbit"
nitpicks = false

[exec]
verbose = false
tui = false
persist = false

[sound]
enabled = true
on_completed = "glass"
on_failed = "basso"
```
