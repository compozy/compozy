# Basic Standalone Example

Minimal, runnable project showing Compozy in standalone mode for local development.

## Prerequisites
- Docker (for PostgreSQL via Compose)
- Go 1.25+

## Quickstart
```bash
cd examples/standalone/basic
docker compose up -d postgres
COMPOZY_DEBUG=false go run ../../../main.go --config.from=env
```

In a second terminal, execute the workflow:
```bash
curl -sS -X POST 'http://localhost:5001/api/v0/workflows/hello-world/executions' \
  -H 'Content-Type: application/json' \
  -d '{"input":{"name":"Developer"}}'
```

## Files
- `compozy.yaml` — project config with `workflows/hello-world.yaml`
- `docker-compose.yml` — PostgreSQL
- `agents/hello-agent.yaml` — minimal agent
- `tasks/hello-task.yaml` — basic task using the agent
- `workflows/hello-world.yaml` — simple workflow used by tests
- `.env.example` — environment template

## Notes
- Uses mock provider through tests; when running manually, set a real `OPENAI_API_KEY` or switch to mock via docs.

