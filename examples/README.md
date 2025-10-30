# Compozy Examples

Browse runnable examples demonstrating Compozy features and integrations. Each folder includes a README with setup instructions.

## Mode Configurations

Use the ready-to-run configs under `examples/configs` to bootstrap different infrastructure profiles:

- `memory-mode.yaml` — zero-dependency setup for demos and CI smoke tests.
- `persistent-mode.yaml` — embedded services with on-disk durability for daily development.
- `distributed-mode.yaml` — production wiring that connects to managed PostgreSQL, Temporal, and Redis clusters.

## Database Examples

### SQLite Quickstart

**Location:** `database/sqlite-quickstart/`

Minimal example demonstrating SQLite backend with a filesystem vector DB. Perfect for local development and testing.

**Highlights:**

- No external database dependencies
- Single-file SQLite datastore
- Filesystem vector embeddings

[View Example →](./database/sqlite-quickstart/)
