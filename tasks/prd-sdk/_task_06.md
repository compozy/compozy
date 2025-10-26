## status: completed

<task_context>
<domain>sdk/compozy</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>sdk/project, sdk/workflow, engine/infra/server</dependencies>
</task_context>

# Task 06.0: Prototype Integration Path (M)

## Overview

Implement the integration layer that loads SDK-built projects into the engine. This is the critical validation gate from Phase 0 - proving the SDK → Engine architecture works end-to-end.

<critical>
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Integration Layer section)
- **ALWAYS READ** tasks/prd-sdk/04-implementation-plan.md (Phase 0 section)
- **MUST** validate resource registration works
- **MUST** prove workflow execution works with SDK-built configs
- **MUST** use context-first patterns
- **DECISION GATE**: If integration fails, STOP and revisit architecture
</critical>

<requirements>
- Create sdk/compozy package for embedded engine
- Implement loadProjectIntoEngine(ctx, proj) function
- Register all project resources in engine resource store
- Validate SDK → Engine config compatibility
- Create minimal Compozy builder for embedded engine
- Test simple workflow execution end-to-end
</requirements>

## Subtasks

- [x] 06.1 Create sdk/compozy/compozy.go with Compozy struct
- [x] 06.2 Create sdk/compozy/integration.go with loadProjectIntoEngine
- [x] 06.3 Implement resource registration for projects
- [x] 06.4 Implement resource registration for workflows
- [x] 06.5 Implement resource registration for agents
- [x] 06.6 Create minimal builder for Compozy initialization
- [x] 06.7 Add integration test for SDK → Engine loading
- [x] 06.8 Add integration test for workflow execution
- [x] 06.9 Validate prototype succeeds (GATE)

## Implementation Details

Reference: tasks/prd-sdk/02-architecture.md (Integration Layer: SDK → Engine)

### Integration Layer

```go
// sdk/compozy/integration.go
package compozy

import (
    "context"
    "github.com/compozy/compozy/engine/infra/server"
    "github.com/compozy/compozy/engine/project"
)

// loadProjectIntoEngine registers SDK-built project config into the engine
func (c *Compozy) loadProjectIntoEngine(ctx context.Context, proj *project.Config) error {
    // 1. Validate project configuration
    // 2. Register project in resource store
    // 3. Register all workflows
    // 4. Register all agents
    // 5. Register all tools
    // 6. Register knowledge bases
    // 7. Register memory configs
    // 8. Register MCP servers
    // 9. Register schemas
}
```

### Relevant Files

- `sdk/compozy/compozy.go` (NEW)
- `sdk/compozy/integration.go` (NEW)
- `sdk/compozy/integration_test.go` (NEW)
- `engine/infra/server/server.go` (REFERENCE)
- `engine/infra/store/resource_store.go` (REFERENCE)

### Dependent Files

- `sdk/project/builder.go`
- `sdk/workflow/builder.go`
- Engine resource store

## Deliverables

- ✅ `sdk/compozy/compozy.go` with main Compozy struct
- ✅ `sdk/compozy/integration.go` with loadProjectIntoEngine
- ✅ Resource registration for all project resources
- ✅ Integration test proving SDK → Engine loading works
- ✅ Integration test proving workflow execution works
- ✅ Phase 0 validation report documenting success/failure
- ✅ DECISION: Continue to Phase 1 or revisit architecture

## Tests

Reference: tasks/prd-sdk/_tests.md

- Integration tests for SDK → Engine:
  - [x] Test project registration in resource store
  - [x] Test workflow registration works
  - [x] Test agent registration works
  - [x] Test resource lookup by ID works
  - [x] Test simple workflow execution end-to-end
  - [x] Test error handling for invalid configs
  - [x] Test context propagation through integration layer
  - [x] Test logger.FromContext works in integration

## Validation Report

- Prototype loads SDK project and workflows into memory-backed resource store
- Integration tests exercise registration, execution, logging, and error handling

## Success Criteria

- SDK-built project successfully registers in engine
- Workflows built with SDK execute in engine
- Resource store contains all registered resources
- Context-first pattern works throughout integration
- No circular dependencies detected
- Integration tests pass
- Workflow execution test passes
- **GATE**: Architecture validated, proceed to Phase 1
