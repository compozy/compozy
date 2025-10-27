# Temporal Standalone Migration Guide

## Purpose

Helps teams switch between remote Temporal clusters and the embedded standalone mode without altering workflow logic.

## Key Concepts

- Compare `compozy.remote.yaml` and `compozy.standalone.yaml`
- Use identical workflows and task queues in both modes
- Provide a migration checklist and rollback plan

## Prerequisites

- Docker Desktop (only required when running the remote stack)
- Go 1.25.2+
- Model API key (copy `.env.example`)

## Quick Start

```bash
cd examples/temporal-standalone/migration-from-remote
cp .env.example .env
```

### Option A: Standalone

```bash
compozy start --config=compozy.standalone.yaml
compozy workflow trigger demo --input='{"mode": "standalone"}'
```

### Option B: Remote via Docker

```bash
docker-compose up -d
compozy start --config=compozy.remote.yaml
compozy workflow trigger demo --input='{"mode": "remote"}'
```

## Migration Checklist

1. Align namespaces and task queues across both configs
2. Export environment overrides (see `README.md`) for CI pipelines
3. Run smoke tests in standalone mode before decommissioning remote infrastructure
4. Document rollback instructions (`docker-compose down` and restore remote config)

## Expected Output

- Workflows succeed in both modes with identical results
- UI port differs: 8233 for standalone, 8088 exposed by Docker for remote (configurable)
- Logs note which mode is active via the `mode` workflow input

## Troubleshooting

- Remote mode fails to connect: ensure Docker containers are running and `host_port` matches the Docker compose service.
- Ports busy: adjust `frontend_port`/`ui_port` in `compozy.standalone.yaml` or mapped ports in `docker-compose.yml`.
- Switching modes in CI: store both configs and select via `compozy start --config=...`.

## What's Next

- Dive deeper into deployment trade-offs in `../../../docs/content/docs/deployment/temporal-modes.mdx`
- Use the persistence example (`../persistent/`) once you settle on standalone mode
