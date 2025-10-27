# Temporal Standalone Without UI Example

## Purpose

Demonstrates running the embedded Temporal server with the Web UI disabled for headless environments such as CI pipelines or lightweight containers.

## Key Concepts

- Disable the Web UI using `enable_ui: false`
- Reduce resource usage and startup time
- Rely on CLI diagnostics instead of browser tooling

## Prerequisites

- Go 1.25.2+
- Model API key (copy `.env.example`)

## Quick Start

```bash
cd examples/temporal-standalone/no-ui
cp .env.example .env
compozy start
```

## Verify Headless Operation

- UI port log entry is absent and no HTTP listener starts
- Use `compozy workflow trigger minimal` to run the sample workflow
- Inspect state through `compozy workflow list` or `compozy workflow describe`

## Expected Output

- Startup logs include `ui_enabled=false`
- Workflow finishes in a few seconds even without the UI running
- `compozy config diagnostics` shows the standalone configuration is active

## Troubleshooting

- Need UI temporarily? Set `TEMPORAL_STANDALONE_ENABLE_UI=true` and restart.
- Want quieter logs? Adjust `temporal.standalone.log_level` (default is `warn` here).
- Workflows not appearing? Remember there is no Web UI; use the CLI inspection commands.

## What's Next

- Pair this configuration with the integration testing example (`../integration-testing/`) for CI builds
- Review logger guidance for headless runs: `../../../docs/content/docs/troubleshooting/temporal.mdx`
