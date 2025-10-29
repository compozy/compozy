# Mixed Mode Example

Hybrid deployment using standalone cache (embedded Redis) with an external Temporal server.

## When to use
- You want easy local caching but need to connect to a shared Temporal cluster.

## Prerequisites
- Docker (Temporal and Postgres)
- Go 1.25+

## Start services
```bash
cd examples/standalone/mixed-mode
docker compose up -d temporal postgres
```

## Run Compozy
```bash
export COMPOZY_MODE=standalone
export COMPOZY_REDIS_MODE=standalone
export COMPOZY_TEMPORAL_MODE=remote
export COMPOZY_TEMPORAL_HOST_PORT=localhost:7233
export COMPOZY_TEMPORAL_NAMESPACE=default
go run ../../../main.go --config.from=env
```

## Execute the workflow
```bash
curl -sS -X POST 'http://localhost:5001/api/v0/workflows/distributed-demo/executions' \
  -H 'Content-Type: application/json' \
  -d '{"input":{"name":"Hybrid"}}'
```

