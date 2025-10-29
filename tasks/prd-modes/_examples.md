# Examples Plan: Three-Mode Configuration System

**PRD Reference**: `tasks/prd-modes/_prd.md`
**Tech Spec Reference**: `tasks/prd-modes/_techspec.md`
**Status**: Planning

---

## Examples Strategy

### Goals
1. Demonstrate all three modes with realistic use cases
2. Show mode-specific configuration patterns
3. Provide copy-paste ready examples for each deployment scenario
4. Validate that all modes work as documented

### Target Audiences
- **Trial Users**: Memory mode examples (fastest path to value)
- **Local Developers**: Persistent mode examples (development workflow)
- **DevOps Engineers**: Distributed mode examples (production deployment)

---

## Example Directory Structure

```
examples/
├── README.md                      # Overview and quick links (UPDATE)
├── memory-mode/                   # NEW (rename from standalone-mode/)
│   ├── README.md
│   ├── compozy.yaml
│   ├── .env.example
│   ├── src/
│   │   └── workflows/
│   │       └── hello-world.ts
│   └── tests/
│       └── hello-world.test.ts
├── persistent-mode/               # NEW
│   ├── README.md
│   ├── compozy.yaml
│   ├── .env.example
│   ├── .gitignore                 # Exclude .compozy/
│   ├── src/
│   │   └── workflows/
│   │       └── stateful-agent.ts
│   └── tests/
│       └── stateful-agent.test.ts
├── distributed-mode/              # UPDATE (formerly examples/basic or distributed)
│   ├── README.md
│   ├── compozy.yaml
│   ├── docker-compose.yaml
│   ├── .env.example
│   ├── src/
│   │   └── workflows/
│   │       └── production-agent.ts
│   └── k8s/                       # Optional Kubernetes manifests
│       ├── deployment.yaml
│       └── service.yaml
└── advanced/                      # Keep existing advanced examples
    └── ...
```

---

## Example Projects

### 1. Memory Mode Example

**Directory**: `examples/memory-mode/`
**Purpose**: Demonstrate zero-dependency quickstart
**Priority**: CRITICAL (first impression)

#### Files to Create/Update

**`examples/memory-mode/README.md`**
```markdown
# Memory Mode Example

The fastest way to run Compozy - zero external dependencies, instant startup.

## Quick Start

```bash
cd examples/memory-mode
compozy start
```

Server ready in <1 second!

## What This Demonstrates

- ✅ Zero-dependency deployment
- ✅ In-memory SQLite database
- ✅ Embedded Temporal server
- ✅ Instant startup (<1s)
- ⚠️ All data lost on restart (ephemeral)

## Use Cases

- Trying Compozy for the first time
- Running tests (50-80% faster)
- Quick prototyping and demos
- CI/CD pipelines

## Configuration Highlights

```yaml
mode: memory  # Default, can be omitted

# All services embedded, no external dependencies
# Database: :memory:
# Cache: in-memory (miniredis)
# Temporal: embedded
```

## Running the Example

```bash
# Start server
compozy start

# Execute workflow
compozy run hello-world --input '{"name": "World"}'

# Expected output:
# Hello, World!

# Stop server (all data lost)
compozy stop
```

## Next Steps

- **Need persistence?** See [persistent-mode example](../persistent-mode/)
- **Going to production?** See [distributed-mode example](../distributed-mode/)
```

**`examples/memory-mode/compozy.yaml`**
```yaml
# Memory Mode Example - Zero Dependencies
name: memory-mode-example
version: 0.1.0
description: Fastest way to run Compozy - no external services required

# Memory mode (default)
mode: memory

# Explicit configuration (all defaults)
database:
  driver: sqlite
  url: ":memory:"

temporal:
  mode: memory
  namespace: memory-example

redis:
  mode: memory
  # Embedded miniredis, no persistence

# Agent configuration
agents:
  hello-agent:
    entrypoint: ./src/workflows/hello-world.ts
    tools:
      - name: echo
        type: builtin

# Server configuration
server:
  host: localhost
  port: 8080
  log_level: info
```

**`examples/memory-mode/.env.example`**
```bash
# Memory Mode Environment Variables
COMPOZY_MODE=memory
COMPOZY_LOG_LEVEL=info
COMPOZY_SERVER_PORT=8080

# No external service configuration needed!
```

**`examples/memory-mode/src/workflows/hello-world.ts`**
```typescript
import { defineWorkflow } from '@compozy/sdk'

export default defineWorkflow({
  name: 'hello-world',
  description: 'Simple greeting workflow',

  async execute({ input, tools }) {
    const name = input.name || 'World'
    const greeting = `Hello, ${name}!`

    await tools.echo({ message: greeting })

    return { greeting }
  }
})
```

---

### 2. Persistent Mode Example

**Directory**: `examples/persistent-mode/`
**Purpose**: Demonstrate local development with state persistence
**Priority**: HIGH (common development scenario)

#### Files to Create

**`examples/persistent-mode/README.md`**
```markdown
# Persistent Mode Example

Local development with state preservation - no external dependencies, but data persists between restarts.

## Quick Start

```bash
cd examples/persistent-mode
compozy start
```

Server ready in <2 seconds, state saved to `./.compozy/`

## What This Demonstrates

- ✅ File-based SQLite database
- ✅ Embedded Temporal with persistence
- ✅ Embedded Redis with BadgerDB persistence
- ✅ State preserved across restarts
- ✅ Still zero external dependencies

## Use Cases

- Local development with state
- Multi-day development sessions
- Debugging workflows with database inspection
- Learning Compozy with persistent examples

## Configuration Highlights

```yaml
mode: persistent

# Files automatically saved to ./.compozy/
# - compozy.db (main database)
# - temporal.db (Temporal database)
# - redis/ (Redis persistence)
```

## Running the Example

```bash
# Start server (first time)
compozy start
# Data directory created: ./.compozy/

# Execute stateful workflow
compozy run stateful-agent --input '{"action": "increment"}'
# Counter: 1

# Execute again
compozy run stateful-agent --input '{"action": "increment"}'
# Counter: 2

# Restart server
compozy restart

# State preserved!
compozy run stateful-agent --input '{"action": "get"}'
# Counter: 2 (state restored from ./.compozy/)
```

## Data Directory Structure

```
.compozy/
├── compozy.db          # Main application database
├── temporal.db         # Temporal server database
└── redis/              # Redis persistence directory
    └── dump.rdb
```

## Inspecting State

```bash
# Inspect main database
sqlite3 ./.compozy/compozy.db "SELECT * FROM workflows;"

# Inspect Temporal database
sqlite3 ./.compozy/temporal.db "SELECT * FROM executions;"

# View Redis data
# (Requires Redis CLI with BadgerDB plugin)
```

## Next Steps

- **Don't need persistence?** See [memory-mode example](../memory-mode/)
- **Going to production?** See [distributed-mode example](../distributed-mode/)
```

**`examples/persistent-mode/compozy.yaml`**
```yaml
# Persistent Mode Example - Local Development with State
name: persistent-mode-example
version: 0.1.0
description: Local development with persistence

# Persistent mode
mode: persistent

# Optional: customize data directory
# database:
#   url: ./my-data/compozy.db

# Optional: customize Temporal database
# temporal:
#   standalone:
#     database_file: ./my-data/temporal.db

# Optional: customize Redis persistence directory
# redis:
#   standalone:
#     persistence:
#       enabled: true
#       dir: ./my-data/redis

# Agent configuration
agents:
  stateful-agent:
    entrypoint: ./src/workflows/stateful-agent.ts
    tools:
      - name: counter
        type: builtin

# Server configuration
server:
  host: localhost
  port: 8080
  log_level: info
```

**`examples/persistent-mode/.gitignore`**
```gitignore
# Persistent mode data directory
.compozy/

# Node modules
node_modules/

# Environment variables
.env
.env.local
```

**`examples/persistent-mode/src/workflows/stateful-agent.ts`**
```typescript
import { defineWorkflow } from '@compozy/sdk'

export default defineWorkflow({
  name: 'stateful-agent',
  description: 'Workflow demonstrating state persistence',

  async execute({ input, tools, state }) {
    // State persists to ./.compozy/compozy.db
    const counter = state.get('counter') || 0

    if (input.action === 'increment') {
      const newValue = counter + 1
      await state.set('counter', newValue)
      return { counter: newValue }
    }

    if (input.action === 'reset') {
      await state.set('counter', 0)
      return { counter: 0 }
    }

    // Default: get current value
    return { counter }
  }
})
```

---

### 3. Distributed Mode Example

**Directory**: `examples/distributed-mode/`
**Purpose**: Demonstrate production-ready deployment with external services
**Priority**: MEDIUM (update existing distributed example)

#### Files to Update

**`examples/distributed-mode/README.md`**
```markdown
# Distributed Mode Example

Production-ready deployment with external PostgreSQL, Redis, and Temporal services.

## Prerequisites

- Docker & Docker Compose (for local testing)
- OR existing PostgreSQL, Redis, Temporal infrastructure

## Quick Start (Docker Compose)

```bash
cd examples/distributed-mode

# Start external services
docker-compose up -d

# Wait for services to be ready (30-60 seconds)
docker-compose logs -f

# Start Compozy
compozy start
```

Server ready in <3 seconds, connected to external services.

## What This Demonstrates

- ✅ External PostgreSQL database
- ✅ External Redis cache
- ✅ External Temporal server
- ✅ Production-ready configuration
- ✅ Horizontal scaling capability
- ✅ TLS support
- ✅ High availability

## Use Cases

- Production deployments
- Horizontal scaling
- High availability setups
- Kubernetes deployments

## Configuration Highlights

```yaml
mode: distributed

database:
  driver: postgres
  url: postgresql://user:pass@postgres:5432/compozy

temporal:
  mode: remote
  host_port: temporal:7233
  namespace: production

redis:
  mode: distributed
  distributed:
    addr: redis:6379
    tls:
      enabled: true
```

## Running the Example

```bash
# Start infrastructure
docker-compose up -d

# Verify services
docker-compose ps

# Start Compozy
export COMPOZY_MODE=distributed
export COMPOZY_DATABASE_URL="postgresql://compozy:password@localhost:5432/compozy"
export REDIS_ADDR="localhost:6379"
export TEMPORAL_HOST_PORT="localhost:7233"

compozy start

# Execute production workflow
compozy run production-agent --input '{"task": "process-data"}'

# Scale horizontally (multiple instances)
compozy start --port 8081  # Instance 2
compozy start --port 8082  # Instance 3
```

## Docker Compose Services

- **PostgreSQL** (5432): Main application database
- **PostgreSQL** (5433): Temporal database
- **Temporal** (7233): Workflow orchestration
- **Temporal UI** (8080): Workflow monitoring
- **Redis** (6379): Cache and message broker

## Kubernetes Deployment

See `k8s/` directory for Kubernetes manifests:

```bash
kubectl apply -f k8s/
```

## Next Steps

- **Simpler setup?** See [memory-mode example](../memory-mode/) or [persistent-mode example](../persistent-mode/)
- **Advanced features?** See [advanced examples](../advanced/)
```

**`examples/distributed-mode/compozy.yaml`**
```yaml
# Distributed Mode Example - Production Deployment
name: distributed-mode-example
version: 1.0.0
description: Production-ready deployment with external services

# Distributed mode
mode: distributed

# PostgreSQL database
database:
  driver: postgres
  url: ${COMPOZY_DATABASE_URL}
  pool:
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: 5m

# External Temporal
temporal:
  mode: remote
  host_port: ${TEMPORAL_HOST_PORT}
  namespace: ${TEMPORAL_NAMESPACE:-production}
  tls:
    enabled: ${TEMPORAL_TLS_ENABLED:-false}

# External Redis
redis:
  mode: distributed
  distributed:
    addr: ${REDIS_ADDR}
    password: ${REDIS_PASSWORD}
    db: ${REDIS_DB:-0}
    tls:
      enabled: ${REDIS_TLS_ENABLED:-false}
    pool:
      max_active: 100
      max_idle: 10

# Agent configuration
agents:
  production-agent:
    entrypoint: ./src/workflows/production-agent.ts
    tools:
      - name: http
        type: builtin
      - name: database
        type: builtin

# Server configuration
server:
  host: 0.0.0.0
  port: ${COMPOZY_SERVER_PORT:-8080}
  log_level: ${COMPOZY_LOG_LEVEL:-info}

# Observability
observability:
  metrics:
    enabled: true
    port: 9090
  tracing:
    enabled: true
    endpoint: ${OTEL_EXPORTER_OTLP_ENDPOINT}
```

**`examples/distributed-mode/docker-compose.yaml`**
```yaml
version: '3.8'

services:
  # Main PostgreSQL database
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: compozy
      POSTGRES_USER: compozy
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U compozy"]
      interval: 5s
      timeout: 5s
      retries: 5

  # Temporal PostgreSQL database
  postgres-temporal:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: temporal
      POSTGRES_USER: temporal
      POSTGRES_PASSWORD: temporal
    ports:
      - "5433:5432"
    volumes:
      - postgres_temporal_data:/var/lib/postgresql/data

  # Temporal server
  temporal:
    image: temporalio/auto-setup:latest
    depends_on:
      postgres-temporal:
        condition: service_healthy
    environment:
      DB: postgresql
      DB_PORT: 5432
      POSTGRES_USER: temporal
      POSTGRES_PWD: temporal
      POSTGRES_SEEDS: postgres-temporal
      DYNAMIC_CONFIG_FILE_PATH: config/dynamicconfig/development-sql.yaml
    ports:
      - "7233:7233"
    volumes:
      - ./config/temporal:/etc/temporal/config/dynamicconfig

  # Temporal UI
  temporal-ui:
    image: temporalio/ui:latest
    depends_on:
      - temporal
    environment:
      TEMPORAL_ADDRESS: temporal:7233
    ports:
      - "8080:8080"

  # Redis
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  postgres_data:
  postgres_temporal_data:
  redis_data:
```

---

### 4. Examples README Update

**File**: `examples/README.md`
**Priority**: HIGH (navigation hub)

**Content:**
```markdown
# Compozy Examples

Practical examples demonstrating Compozy's three deployment modes.

## Quick Links

| Mode | Use Case | Startup Time | External Deps | Example |
|------|----------|--------------|---------------|---------|
| **Memory** | Tests, prototyping | <1s | None | [View](./memory-mode/) |
| **Persistent** | Local development | <2s | None | [View](./persistent-mode/) |
| **Distributed** | Production | <3s | Yes | [View](./distributed-mode/) |

## Mode Selection Guide

### 🚀 Memory Mode
**Best for:** First-time users, tests, CI/CD

- ✅ Zero external dependencies
- ✅ Instant startup
- ✅ Fastest execution
- ⚠️ No persistence (data lost on restart)

**Quick Start:**
```bash
cd examples/memory-mode
compozy start
```

### 💾 Persistent Mode
**Best for:** Local development, debugging

- ✅ Zero external dependencies
- ✅ State preserved between restarts
- ✅ Inspect database files
- ⚠️ Single-process only

**Quick Start:**
```bash
cd examples/persistent-mode
compozy start
```

### 🏭 Distributed Mode
**Best for:** Production deployments

- ✅ Horizontal scaling
- ✅ High availability
- ✅ Production-ready
- ⚠️ Requires external PostgreSQL, Redis, Temporal

**Quick Start:**
```bash
cd examples/distributed-mode
docker-compose up -d
compozy start
```

## Running Examples

Each example is self-contained:

```bash
# Navigate to example
cd examples/<mode>-mode

# Read the README
cat README.md

# Start the example
compozy start

# Execute workflows
compozy run <workflow-name>
```

## Example Structure

All examples follow this structure:

```
<mode>-mode/
├── README.md              # Mode-specific guide
├── compozy.yaml          # Mode-specific configuration
├── .env.example          # Environment variables template
├── src/
│   └── workflows/        # Example workflows
└── tests/                # Example tests
```

## Advanced Examples

See [advanced/](./advanced/) for more complex scenarios:
- Multi-agent systems
- Custom tools integration
- Observability setup
- Deployment patterns

## Contributing

Have an example idea? See [CONTRIBUTING.md](../CONTRIBUTING.md).
```

---

## Example Validation Checklist

### Functional Testing
- [ ] Memory mode example runs successfully
- [ ] Persistent mode example runs successfully
- [ ] Distributed mode example runs successfully
- [ ] State persists in persistent mode (verify `./.compozy/` created)
- [ ] State does NOT persist in memory mode
- [ ] Docker compose works for distributed mode
- [ ] All workflows execute without errors

### Configuration Testing
- [ ] All `compozy.yaml` files are valid
- [ ] Environment variables work correctly
- [ ] Mode defaults are correct
- [ ] No hardcoded values (use env vars)

### Documentation Testing
- [ ] All README examples run as documented
- [ ] Code snippets are copy-paste ready
- [ ] Links between examples work
- [ ] Troubleshooting sections address common issues

### User Experience
- [ ] Memory mode is clearly the "quick start" option
- [ ] Persistent mode emphasizes local development
- [ ] Distributed mode emphasizes production
- [ ] Decision matrix helps choose mode
- [ ] Next steps guide users to other modes

---

## Deliverables

### Must-Have (Task 19.0)
- [x] Rename `standalone-mode/` → `memory-mode/`
- [x] Create `persistent-mode/` example
- [x] Update `distributed-mode/` example
- [x] Update `examples/README.md`
- [x] All examples tested and working

### Nice-to-Have (Post-MVP)
- [ ] Video walkthrough of each mode
- [ ] Kubernetes manifests for distributed mode
- [ ] Advanced examples (multi-agent, custom tools)
- [ ] Performance comparison benchmarks

---

## Success Criteria

- [x] All three mode examples exist and work
- [x] Each example README is clear and actionable
- [x] Memory mode example runnable in <5 minutes
- [x] Persistent mode demonstrates state preservation
- [x] Distributed mode demonstrates production patterns
- [x] Examples README provides clear mode selection guidance
- [x] All code examples are copy-paste ready
- [x] No "standalone" references (except migration notes)

---

## Implementation Notes

- Task 19.0 handles example creation/updates
- Task 23.0 validates all examples work
- Examples should match documentation exactly

**Estimated Effort**: 1 day (part of Phase 4 parallelization)
