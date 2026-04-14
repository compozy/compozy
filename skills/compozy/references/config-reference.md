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
| `max_retries` | int | Maximum number of retries on agent failure |
| `retry_backoff_multiplier` | float | Backoff multiplier between retries |

### `[start]`

Options specific to `compozy start`.

| Field | Type | Description |
| --- | --- | --- |
| `include_completed` | bool | Include tasks already marked as completed |

### `[tasks]`

Task type registry.

| Field | Type | Description |
| --- | --- | --- |
| `types` | string[] | Allowed task types. Default: `["frontend", "backend", "docs", "test", "infra", "refactor", "chore", "bugfix"]` |

### `[fix_reviews]`

Options specific to `compozy fix-reviews`.

| Field | Type | Description |
| --- | --- | --- |
| `concurrent` | int | Number of batches to process in parallel (1-10) |
| `batch_size` | int | Number of file groups per batch (1-50) |
| `include_resolved` | bool | Include already-resolved review issues |

### `[fetch_reviews]`

Options specific to `compozy fetch-reviews`.

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
```
