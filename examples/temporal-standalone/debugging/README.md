# Temporal Standalone Debugging Example

## Purpose

Highlights debugging techniques when running Temporal in standalone mode with verbose logging and full Web UI access.

## Key Concepts

- Enable Temporal server debug logs via `temporal.standalone.log_level: debug`
- Trigger an intentional workflow failure to inspect error details in the UI
- Retry with a fixed payload to watch recovery in real time

## Prerequisites

- Go 1.25.2+
- Node.js 20+ (required for the Bun runtime used by the validation tool)
- Installed Bun runtime (https://bun.sh)
- Model provider API key (see `.env.example`)

## Quick Start

```bash
cd examples/temporal-standalone/debugging
cp .env.example .env
bun install # installs dependencies declared by Bun if needed
compozy start
```

## Trigger a Failure

```bash
compozy workflow trigger debugging --input='{"value": -1}'
```

- Workflow fails because the validation tool rejects negative numbers
- Logs include stack traces thanks to debug log level
- UI (<http://localhost:8233>) shows the failed activity with full details

## Retry with a Valid Payload

```bash
compozy workflow trigger debugging --input='{"value": 42}'
```

- Workflow succeeds and produces a diagnostic summary of the run
- UI shows both failed and successful runs for comparison

## Expected Output

- Server logs include `log_level=debug`
- First run transitions to `Failed` with error `value must be positive`
- Second run completes with a message summarizing the validated value and run ID

## Troubleshooting

- Bun missing: install Bun (`curl -fsSL https://bun.sh/install | bash`) or adjust the runtime if you prefer Node.
- Validation keeps failing: ensure you pass a positive integer and the JSON payload is quoted correctly.
- UI inaccessible: port 8233 might be in use; adjust `ui_port` in `compozy.yaml` or stop the conflicting process.

## What's Next

- Learn more debugging strategies in `../../../docs/content/docs/troubleshooting/temporal.mdx`
- Combine with the custom ports example (`../custom-ports/`) for remote debugging setups
