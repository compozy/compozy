# Examples Plan: Standalone Mode - Redis Alternatives

## Conventions

- Folder prefix: `examples/standalone/*`
- Use `mode: standalone` in all examples to demonstrate standalone deployment
- Avoid secrets; use environment variable interpolation (`${VAR}`)
- Include README with prerequisites and commands
- Keep examples minimal and focused on specific use cases

## Example Matrix

### 1. examples/standalone/basic

- **Purpose**: Simplest possible standalone deployment for local development
- **Files**:
  - `compozy.yaml` - Minimal config with `mode: standalone`
  - `workflows/hello-world.yaml` - Basic workflow
  - `README.md` - Quick start instructions
- **Demonstrates**:
  - Global mode configuration (inheritance pattern)
  - Zero external dependencies (except PostgreSQL)
  - In-memory operation (no persistence)
- **Walkthrough**:
  ```bash
  cd examples/standalone/basic
  compozy start
  compozy workflow run hello-world
  ```

### 2. examples/standalone/with-persistence

- **Purpose**: Standalone with BadgerDB snapshots for data durability
- **Files**:
  - `compozy.yaml` - Standalone with persistence enabled
  - `workflows/stateful-workflow.yaml` - Workflow using memory store
  - `.gitignore` - Exclude `./data` directory
  - `README.md` - Persistence configuration guide
- **Demonstrates**:
  - Snapshot configuration (interval, on-shutdown, restore)
  - Data persistence across restarts
  - Memory store usage with conversation history
- **Walkthrough**:
  ```bash
  cd examples/standalone/with-persistence
  compozy start
  # Run workflow, stop server, restart
  compozy start  # Data restored from snapshot
  ```

### 3. examples/standalone/mixed-mode

- **Purpose**: Advanced - override specific components for hybrid deployment
- **Files**:
  - `compozy.yaml` - Standalone with Redis override to distributed
  - `workflows/hybrid-workflow.yaml` - Workflow example
  - `docker-compose.yml` - External Redis for testing
  - `README.md` - Mixed mode use case explanation
- **Demonstrates**:
  - Per-component mode overrides
  - Global mode with Redis using external instance
  - Temporal and MCPProxy still embedded
  - When to use mixed mode (dev + shared Redis)
- **Walkthrough**:
  ```bash
  cd examples/standalone/mixed-mode
  docker compose up -d redis  # Start external Redis
  compozy start
  ```

### 4. examples/standalone/edge-deployment

- **Purpose**: Minimal footprint for edge/IoT deployments
- **Files**:
  - `compozy.yaml` - Standalone with memory limits and persistence
  - `workflows/edge-workflow.yaml` - Lightweight workflow
  - `Dockerfile` - Optimized container image
  - `README.md` - Edge deployment guide
- **Demonstrates**:
  - Resource-constrained configuration
  - Compact snapshot intervals
  - Minimal logging and telemetry
  - Single-binary deployment
- **Walkthrough**:
  ```bash
  cd examples/standalone/edge-deployment
  docker build -t compozy-edge .
  docker run -p 8080:8080 compozy-edge
  ```

### 5. examples/standalone/migration-demo

- **Purpose**: Demonstrate migration from standalone to distributed
- **Files**:
  - `compozy-standalone.yaml` - Initial standalone config
  - `compozy-distributed.yaml` - Target distributed config
  - `workflows/sample-workflow.yaml` - Test workflow
  - `migrate.sh` - Migration script with steps
  - `docker-compose.yml` - External Redis and PostgreSQL
  - `README.md` - Complete migration walkthrough
- **Demonstrates**:
  - Configuration differences between modes
  - Data export (if applicable)
  - Service restart procedure
  - Validation steps
- **Walkthrough**:
  ```bash
  cd examples/standalone/migration-demo
  # Start with standalone
  compozy start --config compozy-standalone.yaml
  # Run migration script
  ./migrate.sh
  # Start with distributed
  docker compose up -d redis
  compozy start --config compozy-distributed.yaml
  ```

## Minimal YAML Shapes

### Basic Standalone (Full Inheritance)

```yaml
# compozy.yaml - Minimal standalone
mode: standalone

server:
  host: 0.0.0.0
  port: 8080

database:
  host: localhost
  port: 5432
  name: compozy

# All components (redis, temporal, mcpproxy) inherit "standalone" mode
```

### Standalone with Persistence

```yaml
# compozy.yaml - With persistence
mode: standalone

redis:
  standalone:
    persistence:
      enabled: true
      data_dir: ./data/redis
      snapshot_interval: 5m
      snapshot_on_shutdown: true
      restore_on_startup: true

server:
  host: 0.0.0.0
  port: 8080

database:
  host: localhost
  port: 5432
  name: compozy
```

### Mixed Mode (Advanced)

```yaml
# compozy.yaml - Mixed mode
mode: standalone  # Default for all components

# Override Redis to use external instance
redis:
  mode: distributed
  addr: localhost:6379
  password: ${REDIS_PASSWORD}

# Temporal and MCPProxy inherit "standalone"
server:
  host: 0.0.0.0
  port: 8080

database:
  host: localhost
  port: 5432
  name: compozy
```

### Full Distributed (Comparison)

```yaml
# compozy.yaml - Distributed mode
mode: distributed

redis:
  addr: redis.prod.internal:6379
  password: ${REDIS_PASSWORD}

temporal:
  host_port: temporal.prod.internal:7233
  namespace: production

mcpproxy:
  mode: ""  # Uses external MCP proxy (or configure as needed)

server:
  host: 0.0.0.0
  port: 8080

database:
  host: postgres.prod.internal
  port: 5432
  name: compozy
```

## Test & CI Coverage

Add to `test/integration/examples/`:

- `standalone_basic_test.go` - Validate basic example runs and executes workflow
- `standalone_persistence_test.go` - Verify snapshot/restore cycle
- `mixed_mode_test.go` - Validate mode overrides work correctly

Integration test requirements:
- Use testcontainers for PostgreSQL and Redis (when needed)
- Test each example's workflow execution
- Verify mode configuration is respected
- Validate persistence (if applicable)

## Runbooks per Example

### basic
- **Prereqs**: PostgreSQL running locally (or via Docker)
- **Env vars**: None required
- **Commands**:
  ```bash
  compozy start
  compozy workflow list
  compozy workflow run hello-world
  ```
- **Expected**: Workflow executes successfully, server shows standalone mode logs

### with-persistence
- **Prereqs**: PostgreSQL, writable `./data` directory
- **Env vars**: None required
- **Commands**:
  ```bash
  compozy start
  compozy workflow run stateful-workflow
  # Stop server (Ctrl+C)
  compozy start
  # Verify data restored
  compozy workflow list
  ```
- **Expected**: Data persists across restarts, snapshot logs visible

### mixed-mode
- **Prereqs**: Docker, PostgreSQL
- **Env vars**: `REDIS_PASSWORD` (optional, if Redis auth enabled)
- **Commands**:
  ```bash
  docker compose up -d redis
  compozy start
  compozy config show  # Verify Redis in distributed, others standalone
  ```
- **Expected**: Redis uses external instance, Temporal/MCP embedded

### edge-deployment
- **Prereqs**: Docker
- **Env vars**: None required
- **Commands**:
  ```bash
  docker build -t compozy-edge .
  docker run -p 8080:8080 compozy-edge
  curl http://localhost:8080/health
  ```
- **Expected**: Container starts, memory footprint <512MB

### migration-demo
- **Prereqs**: Docker, Docker Compose
- **Env vars**: `REDIS_PASSWORD` for distributed mode
- **Commands**:
  ```bash
  # Standalone phase
  compozy start --config compozy-standalone.yaml
  compozy workflow run sample-workflow
  # Migration
  ./migrate.sh  # Starts Docker services
  compozy start --config compozy-distributed.yaml
  compozy workflow list  # Verify migration
  ```
- **Expected**: Successful migration, workflows accessible in distributed mode

## Example README Template

Each example should include:

```markdown
# [Example Name]

## Purpose
[What this example demonstrates]

## Prerequisites
- PostgreSQL [version]
- [Other requirements]

## Configuration Highlights
- `mode: standalone` - [explanation]
- [Other key config points]

## Quick Start
1. [Setup step]
2. `compozy start`
3. `compozy workflow run [workflow]`

## What to Observe
- [Log messages to look for]
- [Behavior to verify]

## Cleanup
```bash
[Cleanup commands]
```

## Next Steps
- Try [related example]
- Read [related docs]
```

## Acceptance Criteria

- [ ] All 5 examples exist and are runnable
- [ ] Each example has a comprehensive README with commands and expected outputs
- [ ] YAML configurations follow project standards and pass validation
- [ ] Workflows in examples are minimal but demonstrate key features
- [ ] Examples are tested in CI (integration tests)
- [ ] Each example builds successfully (if Dockerfile included)
- [ ] Mixed-mode example clearly shows mode override pattern
- [ ] Migration demo provides clear before/after comparison
- [ ] All examples use environment variable interpolation for sensitive data
- [ ] `.gitignore` excludes generated data directories
- [ ] Examples are referenced in documentation (cross-links)

## Additional Assets

### docker-compose.yml (for examples needing external services)

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: compozy
      POSTGRES_USER: compozy
      POSTGRES_PASSWORD: compozy
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --requirepass ${REDIS_PASSWORD:-compozy}

volumes:
  pgdata:
```

### Dockerfile (for edge-deployment example)

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o compozy

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/compozy /usr/local/bin/
COPY examples/standalone/edge-deployment/compozy.yaml /etc/compozy/
EXPOSE 8080
CMD ["compozy", "start", "--config", "/etc/compozy/compozy.yaml"]
```

## Documentation Links

Each example README should link to:
- Configuration reference: `docs/configuration/mode-configuration.mdx`
- Standalone deployment guide: `docs/deployment/standalone-mode.mdx`
- Migration guide: `docs/guides/migrate-standalone-to-distributed.mdx`

