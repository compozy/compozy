# Standalone With Persistence

Demonstrates embedded Redis snapshots (BadgerDB) for data persistence in standalone mode.

## Highlights
- Snapshot on shutdown and periodic snapshots
- Restore on startup from last snapshot

## Prerequisites
- Docker (PostgreSQL)
- Go 1.25+

## Quickstart
```bash
cd examples/standalone/with-persistence
docker compose up -d postgres
export COMPOZY_REDIS_MODE=standalone
export COMPOZY_REDIS_STANDALONE_PERSISTENCE_ENABLED=true
export COMPOZY_REDIS_STANDALONE_PERSISTENCE_DATA_DIR="$(pwd)/data"
export COMPOZY_REDIS_STANDALONE_PERSISTENCE_SNAPSHOT_INTERVAL=30s
export COMPOZY_REDIS_STANDALONE_PERSISTENCE_SNAPSHOT_ON_SHUTDOWN=true
export COMPOZY_REDIS_STANDALONE_PERSISTENCE_RESTORE_ON_STARTUP=true
go run ../../../main.go --config.from=env
```

Execute the stateful workflow:
```bash
curl -sS -X POST 'http://localhost:5001/api/v0/workflows/stateful-demo/executions' \
  -H 'Content-Type: application/json' \
  -d '{"input":{"session":"demo","value":"hello"}}'
```

## Files
- `compozy.yaml` — project config with workflow
- `workflows/stateful-workflow.yaml` — writes/reads from agent memory
- `agents/stateful-agent.yaml` — uses memory slot `session_store`
- `docker-compose.yml` — PostgreSQL

