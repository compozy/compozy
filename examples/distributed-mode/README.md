# Distributed Mode Demo

Connect Compozy to production-style infrastructure by running PostgreSQL,
Temporal, and Redis outside of the application process. This example uses
`docker compose` to provision the dependencies locally while keeping the Compozy
server running on your host machine.

## Prerequisites

- Docker and docker compose plugin
- Go 1.25.2+
- OpenAI API key (or update the model configuration)

## Start Dependencies

```bash
cd examples/distributed-mode
docker compose up -d
```

The services expose:

- PostgreSQL: `postgres://compozy:compozy@localhost:55432/compozy`
- Redis: `redis://localhost:56379`
- Temporal: gRPC at `localhost:7233` and UI at <http://localhost:8233>

## Run Compozy

```bash
cp .env.example .env
export $(grep -v '^#' .env | xargs)
../../bin/compozy start
```

Trigger the workflow to verify connectivity:

```bash
../../bin/compozy workflow trigger support-router --input '{"ticket":"Customer cannot access billing invoice"}'
```

Expected behaviour:

- Compozy connects to the external PostgreSQL, Redis, and Temporal services
- Workflow classification and reply tasks complete using the configured model
- Temporal UI displays executions within the `compozy-distributed` namespace

## Shutdown

```bash
../../bin/compozy stop
docker compose down -v
```

## Troubleshooting

- `dial tcp 127.0.0.1:55432: connect: connection refused`: Ensure docker
  services are running and reachable.
- `missing OPENAI_API_KEY`: Export a valid key or adjust `compozy.yaml` to use a
  different provider.
- `namespace not found`: The Temporal container bootstraps the namespace; wait a
  few seconds after `docker compose up -d` before starting Compozy.
