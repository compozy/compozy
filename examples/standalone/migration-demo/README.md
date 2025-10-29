# Migration Demo: Standalone → Distributed

Step-by-step guide to migrate from standalone (embedded cache + embedded Temporal) to distributed (external Redis + remote Temporal).

## Structure
- `phase1-standalone/` — initial standalone setup
- `phase2-distributed/` — final distributed config and docker-compose
- `migrate.sh` — helper that derives phase2 from phase1

## Run Phase 1
```bash
cd examples/standalone/migration-demo/phase1-standalone
docker compose up -d postgres
go run ../../../../main.go --config.from=env
```

## Migrate
```bash
cd ..
./migrate.sh
```

## Run Phase 2
```bash
cd phase2-distributed
docker compose up -d redis temporal postgres
go run ../../../../main.go --config.from=env
```

