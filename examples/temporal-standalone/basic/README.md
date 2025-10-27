# Temporal Standalone Basic Example

## Purpose

Demonstrates the quickest path to running Compozy with the embedded Temporal server. Everything runs locally, including the Web UI, using the default in-memory configuration.

## Key Concepts

- Start Temporal in standalone mode without Docker or external services
- Default ports (7233-7236) for the Temporal services and 8233 for the UI
- In-memory persistence for fast restarts during development
- Simple workflow execution and inspection through the UI

## Prerequisites

- Go 1.25.2 or newer installed
- Node.js 20+ if you plan to run additional tooling
- An API key for the model configured in `compozy.yaml` (see `.env.example`)

## Quick Start

```bash
cd examples/temporal-standalone/basic
cp .env.example .env
compozy start
```

## Trigger the Workflow

```bash
compozy workflow trigger hello --input='{"name": "Temporal developer"}'
```

## Inspect in the UI

1. Open <http://localhost:8233> in your browser
2. Locate the `hello` workflow run in the Workflows list
3. Expand the history to view task execution details

## Expected Output

- CLI shows `Embedded Temporal server started successfully` logs
- Workflow result includes a greeting that echoes the provided name
- Web UI shows the workflow in the `Completed` state with a single task

## Troubleshooting

- `address already in use`: another process is using port 7233 or 8233. Stop the other process or change the ports in `compozy.yaml`.
- `missing API key`: ensure `.env` contains a valid key for the configured provider. Run `compozy config diagnostics` to confirm environment variables are detected.
- Workflow stuck in `Running`: use the UI to inspect the history and confirm the agent completed. Retry after resolving any model issues.

## What's Next

- Read the standalone architecture overview: `../../../docs/content/docs/architecture/embedded-temporal.mdx`
- Explore other configurations in this directory for persistence, custom ports, and debugging techniques
