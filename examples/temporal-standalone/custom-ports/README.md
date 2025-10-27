# Temporal Standalone Custom Ports Example

## Purpose

Illustrates how to reconfigure the embedded Temporal services and UI to avoid local port conflicts.

## Key Concepts

- Override `frontend_port` and derived service ports (history, matching, worker)
- Expose the UI on a custom HTTP port
- Run multiple standalone instances side by side on the same machine

## Prerequisites

- Go 1.25.2+
- Free TCP ports 8233-8236 and 9233 (or update the config before starting)
- Model provider API key (see `.env.example`)

## Quick Start

```bash
cd examples/temporal-standalone/custom-ports
cp .env.example .env
compozy start
```

## Verify the Ports

- Temporal frontend listens on `127.0.0.1:8233`
- Temporal Web UI is at <http://localhost:9233>
- History, matching, and worker services run on 8234-8236 respectively

## Expected Output

- Logs show `frontend_addr=127.0.0.1:8233` and `ui_addr=http://127.0.0.1:9233`
- Workflow execution succeeds against the custom task queue
- Web UI accessible only on the configured port

## Troubleshooting

- `address already in use`: choose a different base port and update both `frontend_port` and `ui_port`.
- `workflow cannot connect`: confirm `temporal.host_port` matches the frontend port (8233 in this example).
- Browser cannot reach UI: ensure you updated any bookmarks or CLI scripts to the new port.

## What's Next

- Combine custom ports with the persistence example (`../persistent/`) to isolate multiple environments
- Read the configuration reference for every Temporal option: `../../../docs/content/docs/configuration/temporal.mdx`
