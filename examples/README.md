# Compozy Examples

Browse runnable examples demonstrating Compozy features and integrations. Each
folder includes a README with setup instructions.

## Mode Profiles

These directories provide end-to-end environments for each deployment mode:

- `memory-mode/` — zero-dependency setup that starts instantly with fully
  ephemeral services.
- `persistent-mode/` — embedded services that persist data to `.compozy/` for
  stateful local development.
- `distributed-mode/` — connects to external PostgreSQL, Temporal, and Redis
  services via the bundled `docker-compose.yml`.

## Config Packs

Use the ready-to-run configs under `examples/configs` to bootstrap additional
projects or CI environments:

- `memory-mode.yaml` — minimal memory profile for demos and smoke tests.
- `persistent-mode.yaml` — embedded services with on-disk durability.
- `distributed-mode.yaml` — production wiring targeting managed services.

## Additional Examples

Explore the rest of the folders for domain-specific workflows (GitHub, memory,
weather, etc.). Each README describes prerequisites and execution steps.
