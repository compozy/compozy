## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>examples/standalone</domain>
<type>documentation</type>
<scope>examples_runbooks</scope>
<complexity>medium</complexity>
<dependencies>cache|config</dependencies>
</task_context>

# Task 13.0: Examples & Runbooks [Size: M - 1-2 days]

## Overview

Create comprehensive example projects and runbooks that demonstrate standalone mode usage in various scenarios. These examples should be runnable, well-documented, and cover common use cases from basic deployment to edge computing.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- All examples must be tested and working
- All configuration files must be valid
- All Docker Compose files must work correctly
- All README files must be complete and accurate
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Create 5 example projects covering different use cases
- Each example must have complete README with setup instructions
- Each example must be independently runnable
- Docker Compose files must be provided where needed
- Integration tests must verify examples work correctly
- Examples must follow project conventions and best practices
- All examples must be documented in main docs
</requirements>

## Subtasks

- [ ] 13.1 Create basic standalone example
- [ ] 13.2 Create standalone with persistence example
- [ ] 13.3 Create mixed mode example
- [ ] 13.4 Create edge deployment example
- [ ] 13.5 Create migration demo example
- [ ] 13.6 Create integration tests for all examples
- [ ] 13.7 Add examples index to documentation

## Implementation Details

### Example Project Structure

Each example should follow this structure:
```
examples/standalone/<example-name>/
├── README.md                  # Complete setup and usage guide
├── compozy.yaml              # Compozy configuration
├── docker-compose.yml        # Docker Compose (if needed)
├── .env.example              # Environment variables template
├── workflows/                # Sample workflows
│   └── example-workflow.yaml
└── test/                     # Integration tests
    └── example_test.go
```

### Relevant Files

**New Example Projects:**
- `examples/standalone/basic/` - Minimal standalone deployment
- `examples/standalone/with-persistence/` - Standalone with BadgerDB snapshots
- `examples/standalone/mixed-mode/` - Hybrid deployment example
- `examples/standalone/edge-deployment/` - Edge/IoT deployment
- `examples/standalone/migration-demo/` - Migration walkthrough

**Integration Tests:**
- `examples/standalone/test/examples_test.go` - Test all examples work

**Documentation:**
- `docs/content/docs/examples/standalone-examples.mdx` - Examples index

### Dependent Files

- Task 3.0 deliverables - Mode-aware cache factory
- Task 7.0 deliverables - Snapshot manager for persistence example
- Task 12.0 deliverables - Documentation to reference examples

## Deliverables

### Example 1: Basic Standalone (`examples/standalone/basic/`)

**Purpose**: Minimal standalone deployment for local development

**Contents**:
- `compozy.yaml` - Minimal config with `mode: standalone`
- `README.md` - Setup and usage instructions
- `workflows/hello-world.yaml` - Simple workflow example
- `.env.example` - Environment variables template

**Features Demonstrated**:
- Zero external dependencies (PostgreSQL in Docker Compose)
- Quick start for local development
- Basic agent and workflow execution

### Example 2: With Persistence (`examples/standalone/with-persistence/`)

**Purpose**: Standalone with BadgerDB snapshots for data persistence

**Contents**:
- `compozy.yaml` - Config with persistence enabled
- `README.md` - Setup including persistence configuration
- `workflows/stateful-workflow.yaml` - Workflow using agent memory
- `docker-compose.yml` - PostgreSQL only

**Features Demonstrated**:
- Snapshot configuration
- Data persistence across restarts
- Periodic snapshots and graceful shutdown
- State recovery after restart

### Example 3: Mixed Mode (`examples/standalone/mixed-mode/`)

**Purpose**: Hybrid deployment with standalone cache but external Temporal

**Contents**:
- `compozy.yaml` - Mixed mode configuration
- `docker-compose.yml` - External Temporal server
- `README.md` - Setup for hybrid deployment
- `workflows/distributed-workflow.yaml` - Workflow leveraging Temporal

**Features Demonstrated**:
- Global mode with component overrides
- Standalone Redis + External Temporal
- When to use mixed mode deployments
- Configuration flexibility

### Example 4: Edge Deployment (`examples/standalone/edge-deployment/`)

**Purpose**: Resource-constrained edge/IoT deployment

**Contents**:
- `compozy.yaml` - Optimized for low resources
- `README.md` - Edge deployment guide
- `Dockerfile.edge` - Minimal Docker image
- `workflows/edge-workflow.yaml` - Lightweight workflow

**Features Demonstrated**:
- Memory limits and resource constraints
- Minimal configuration
- ARM64 and x86_64 support
- Configurable retention policies
- Running without Docker

### Example 5: Migration Demo (`examples/standalone/migration-demo/`)

**Purpose**: Step-by-step migration from standalone to distributed

**Contents**:
- `phase1-standalone/compozy.yaml` - Initial standalone config
- `phase2-distributed/compozy.yaml` - Final distributed config
- `phase2-distributed/docker-compose.yml` - Add Redis
- `README.md` - Complete migration walkthrough
- `migrate.sh` - Migration helper script

**Features Demonstrated**:
- Migration triggers (when to migrate)
- Configuration changes required
- Testing migration without downtime
- Rollback procedures
- Data considerations

### Integration Tests

**Test File**: `examples/standalone/test/examples_test.go`

Test each example project:
- Setup environment
- Start Compozy with example config
- Execute sample workflows
- Verify expected behavior
- Cleanup resources

### Examples Documentation

**Documentation**: `docs/content/docs/examples/standalone-examples.mdx`

Create index page linking to all examples with:
- Overview of each example
- Use cases and target audience
- Quick start links
- Prerequisites
- Learning objectives

## Tests

Integration tests mapped from `_tests.md` for this feature:

### Example 1: Basic Standalone Tests

- [ ] Should start Compozy with basic standalone config
- [ ] Should execute hello-world workflow successfully
- [ ] Should complete workflow without external dependencies
- [ ] Should use <256MB memory for basic workload
- [ ] Should start in <5 seconds

### Example 2: With Persistence Tests

- [ ] Should start with persistence enabled
- [ ] Should create snapshot directory
- [ ] Should execute stateful workflow with memory
- [ ] Should persist agent memory across restarts
- [ ] Should restore state after restart
- [ ] Should take periodic snapshots
- [ ] Should snapshot on graceful shutdown

### Example 3: Mixed Mode Tests

- [ ] Should start with mixed mode configuration
- [ ] Should use standalone cache and external Temporal
- [ ] Should execute distributed workflow correctly
- [ ] Should honor component mode overrides
- [ ] Should connect to external Temporal server

### Example 4: Edge Deployment Tests

- [ ] Should start with minimal resource configuration
- [ ] Should run with <512MB memory limit
- [ ] Should execute lightweight workflow
- [ ] Should work on ARM64 architecture (if available)
- [ ] Should start without Docker (binary only)
- [ ] Should enforce retention policies

### Example 5: Migration Demo Tests

- [ ] Should run phase 1 (standalone) successfully
- [ ] Should export configuration from phase 1
- [ ] Should run phase 2 (distributed) successfully
- [ ] Should execute same workflows in both phases
- [ ] Migration script should handle config transformation
- [ ] Should document required Redis setup

### Documentation Tests

- [ ] Examples index page should list all examples
- [ ] All example links should work
- [ ] All prerequisites should be accurate
- [ ] All setup instructions should be complete
- [ ] All examples should be discoverable via search

### Quality Checks

- [ ] All example configs are valid YAML
- [ ] All Docker Compose files work correctly
- [ ] All README files are complete and accurate
- [ ] All commands in README are tested and work
- [ ] All environment variables are documented
- [ ] All examples follow project conventions
- [ ] All examples use project-standard structure

### Cross-Platform Testing

- [ ] Examples work on Linux (amd64)
- [ ] Examples work on macOS (arm64/amd64)
- [ ] Edge example works on ARM64
- [ ] Docker examples work on all platforms

## Success Criteria

- All 5 example projects created and working
- Each example has complete, accurate README
- Each example is independently runnable
- Docker Compose files work correctly where provided
- Integration tests pass for all examples
- All examples follow project conventions
- Examples documentation created and published
- All code examples are tested and working
- All configuration files are valid
- Examples cover diverse use cases (dev, prod, edge, migration)
- Examples are discoverable in main documentation
- Cross-platform compatibility verified
- Tests pass with `go test -v ./examples/standalone/test/`
- Examples can be used as templates for real deployments
- Migration demo provides clear, actionable migration path
