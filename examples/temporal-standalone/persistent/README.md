# Temporal Standalone Persistent Example

## Purpose

Shows how to keep workflow history across restarts by storing Temporal state in a local SQLite database instead of memory.

## Key Concepts

- File-backed SQLite persistence configured through `temporal.standalone.database_file`
- Restarting the embedded server without losing workflow history
- Tracking WAL files (`.db-wal`/`.db-shm`) created by Temporal
- Maintenance tips for local development databases

## Prerequisites

- Go 1.25.2+
- Write permissions in `./data`
- Model provider API key (copy `.env.example`)

## Quick Start

```bash
cd examples/temporal-standalone/persistent
mkdir -p data
cp .env.example .env
compozy start
```

## Persist a Workflow Run

```bash
compozy workflow trigger counter --input='{"increment": 5}'
killall compozy
compozy start
compozy workflow describe counter
```

## Expected Output

- Database file `data/temporal.db` plus WAL companions created on first run
- Second start reuses existing history; the workflow remains visible in the UI
- Counter workflow response reflects the cumulative total across invocations

## Troubleshooting

- `database directory not accessible`: ensure the `data` directory exists and is writable before starting.
- `database is locked`: another Temporal process is still running; stop it before restarting.
- `history missing after restart`: verify you restarted using the same config and did not delete `data/temporal.db*`.

## What's Next

- Learn more about persistence strategies in `../../../docs/content/docs/deployment/temporal-modes.mdx`
- Combine persistence with debugging features from `../debugging/`
