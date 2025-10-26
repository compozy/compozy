## status: pending

<task_context>
<domain>v2/compozy</domain>
<type>implementation|integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server|database|temporal</dependencies>
</task_context>

# Task 12.0: Compozy Lifecycle (M)

## Overview

Implement the main Compozy embedded engine package for hosting projects programmatically. Provides builder for configuration and lifecycle management (Start/Stop/Wait) with direct workflow execution capabilities.

<critical>
- **MANDATORY** implement full SDK → Engine integration layer
- **MANDATORY** use `context.Context` throughout
- **MANDATORY** use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- **NEVER** use `context.Background()` in runtime code
- **MANDATORY** register all SDK resources in engine resource store
</critical>

<requirements>
- Compozy struct with embedded server instance
- Builder for server/infrastructure configuration
- Integration layer: SDK → Engine resource registration
- Lifecycle methods: Start(), Stop(ctx), Wait()
- Direct workflow execution: ExecuteWorkflow(ctx)
- Access to internals: Server(), Router(), Config()
- Full resource registration (workflows, agents, tools, knowledge, memory, MCP, schemas)
</requirements>

## Subtasks

- [ ] 12.1 Create v2/compozy package structure
- [ ] 12.2 Implement Compozy struct and Builder
- [ ] 12.3 Implement server/infrastructure configuration methods
- [ ] 12.4 Implement integration.go (SDK → Engine registration)
- [ ] 12.5 Implement lifecycle methods (Start, Stop, Wait)
- [ ] 12.6 Implement ExecuteWorkflow for direct execution
- [ ] 12.7 Add accessor methods (Server, Router, Config)
- [ ] 12.8 Write unit tests for Builder
- [ ] 12.9 Write integration tests for full lifecycle

## Implementation Details

Reference:
- `tasks/prd-modules/02-architecture.md` (Integration Layer: SDK → Engine)
- `tasks/prd-modules/03-sdk-entities.md` (Section 14: Compozy Embedded Engine)

### Key Components

```go
// v2/compozy/compozy.go
type Compozy struct {
    server  *server.Server
    config  *config.Config
    project *project.Config
    ctx     context.Context
}

// v2/compozy/builder.go
func New(proj *project.Config) *Builder
func (b *Builder) WithServerHost(host string) *Builder
func (b *Builder) WithServerPort(port int) *Builder
func (b *Builder) WithDatabase(connString string) *Builder
func (b *Builder) WithTemporal(hostPort, namespace string) *Builder
func (b *Builder) WithRedis(url string) *Builder
func (b *Builder) Build(ctx context.Context) (*Compozy, error)

// v2/compozy/integration.go
func (c *Compozy) loadProjectIntoEngine(ctx context.Context, proj *project.Config) error

// Lifecycle
func (c *Compozy) Start() error
func (c *Compozy) Stop(ctx context.Context) error
func (c *Compozy) Wait() error
func (c *Compozy) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) (*ExecutionResult, error)
```

### Relevant Files

- `v2/compozy/compozy.go` - Main struct and types
- `v2/compozy/builder.go` - Configuration builder
- `v2/compozy/integration.go` - SDK → Engine integration layer
- `v2/compozy/execution.go` - Direct workflow execution
- `v2/compozy/types.go` - Result types

### Dependent Files

- `engine/infra/server/server.go` - Server instance
- `engine/infra/store/resource_store.go` - Resource registration
- `engine/project/config.go` - Project configuration
- `v2/project/builder.go` - Project builder
- `pkg/config/config.go` - Config from context
- `pkg/logger/logger.go` - Logger from context

## Deliverables

- ✅ `v2/compozy/` package with full lifecycle
- ✅ Builder with server and infrastructure configuration
- ✅ Integration layer registering all SDK resources
- ✅ Start/Stop/Wait lifecycle methods
- ✅ ExecuteWorkflow for programmatic execution
- ✅ Context-first patterns throughout
- ✅ Unit tests for Builder and lifecycle
- ✅ Integration test with embedded server

## Tests

Unit and integration tests from `_tests.md`:
- [ ] Builder validates required infrastructure (DB, Temporal, Redis)
- [ ] Build(ctx) creates Compozy with server instance
- [ ] loadProjectIntoEngine registers all resource types
- [ ] Resource IDs are unique across project
- [ ] Start() launches server successfully
- [ ] Stop(ctx) shuts down gracefully
- [ ] ExecuteWorkflow executes SDK-built workflow
- [ ] Context cancellation propagates to server
- [ ] Logger and config retrieved from context
- [ ] Error aggregation in Builder
- [ ] Integration: build project → embed → execute → verify

## Success Criteria

- Builder configures server, DB, Temporal, Redis
- Integration layer registers workflows, agents, tools, knowledge, memory, MCP, schemas
- Lifecycle methods work (Start/Stop/Wait)
- ExecuteWorkflow runs SDK-built workflows
- Context-first pattern enforced
- Test coverage ≥90% for integration layer
- `make lint && make test` pass
- Integration test demonstrates full embedded usage
