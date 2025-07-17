# Global Flags Reference

Compozy CLI provides a consistent set of global flags that work across all commands, ensuring a unified experience whether you're running workflows, managing authentication, or developing locally.

## Server Connection Flags

### `--server-url` / `-s`

Specifies the Compozy server URL for API requests.

- **Environment**: `COMPOZY_SERVER_URL`
- **Config**: `cli.server_url`
- **Example**: `compozy workflow list --server-url https://api.compozy.dev`

### `--config` / `-c`

Path to the configuration file to use instead of the default `compozy.yaml`.

- **Environment**: `COMPOZY_CONFIG_FILE`
- **Config**: `cli.config_file`
- **Example**: `compozy dev --config ./staging.yaml`

## Output Control Flags

### `--format` / `-f` or `--output` / `-o`

Controls output format. Both flags are aliases for convenience.

- **Values**: `json`, `tui`, `auto`
- **Default**: `auto` (TUI for interactive, JSON for non-interactive)
- **Environment**: `COMPOZY_DEFAULT_FORMAT`
- **Config**: `cli.default_format`
- **Examples**:
  ```bash
  compozy workflow list --format json
  compozy workflow list -o tui
  compozy workflow status --output json | jq '.status'
  ```

### `--no-color`

Disables colored output for compatibility with scripts and CI environments.

- **Environment**: `COMPOZY_COLOR_MODE=off`
- **Config**: `cli.color_mode: off`
- **Example**: `compozy workflow list --no-color`

## Behavior Control Flags

### `--debug` / `-d`

Enables debug mode with verbose logging and detailed error information.

- **Environment**: `COMPOZY_DEBUG=true`
- **Config**: `cli.debug: true`
- **Effect**: Sets log level to debug, shows internal operations
- **Example**: `compozy dev --debug`

### `--quiet` / `-q`

Suppresses non-essential output for automation and scripting scenarios.

- **Environment**: `COMPOZY_QUIET=true`
- **Config**: `cli.quiet: true`
- **Effect**: Only shows critical errors and requested data
- **Example**: `compozy workflow status --quiet --format json`

### `--interactive`

Forces interactive mode even when CI or non-TTY environment is detected.

- **Environment**: `COMPOZY_INTERACTIVE=true`
- **Config**: `cli.interactive: true`
- **Example**: `compozy auth login --interactive`

## Flag Precedence

Configuration values are resolved in the following order (highest to lowest priority):

1. **CLI Flags** - Direct command-line arguments
2. **Config File** - Values from `compozy.yaml` or specified config file
3. **Environment Variables** - Environment variable values
4. **Defaults** - Built-in default values

## Common Usage Patterns

### Development with Debug Output

```bash
compozy dev --debug --format tui
```

### Automation Script

```bash
#!/bin/bash
export COMPOZY_QUIET=true
export COMPOZY_DEFAULT_FORMAT=json

STATUS=$(compozy workflow status my-workflow | jq -r '.status')
if [ "$STATUS" = "running" ]; then
  echo "Workflow is running"
fi
```

### CI/CD Pipeline

```bash
compozy workflow run --config ./production.yaml --quiet --no-color --format json
```

### Custom Server Configuration

```bash
compozy workflow list --server-url https://staging.compozy.dev --format json
```

## Flag Aliases Summary

| Long Form  | Short Form | Alternative      |
| ---------- | ---------- | ---------------- |
| `--config` | `-c`       |                  |
| `--format` | `-f`       | `--output`, `-o` |
| `--debug`  | `-d`       |                  |
| `--quiet`  | `-q`       |                  |

## Environment Variables

All global flags can be controlled via environment variables:

```bash
export COMPOZY_SERVER_URL="https://api.compozy.dev"
export COMPOZY_CONFIG_FILE="./my-config.yaml"
export COMPOZY_DEFAULT_FORMAT="json"
export COMPOZY_DEBUG="true"
export COMPOZY_QUIET="false"
export COMPOZY_INTERACTIVE="false"
export COMPOZY_COLOR_MODE="auto"
```

## Configuration File

Global settings can also be configured in your `compozy.yaml`:

```yaml
cli:
  server_url: "https://api.compozy.dev"
  default_format: "tui"
  debug: false
  quiet: false
  interactive: false
  color_mode: "auto"
```
