## status: pending

<task_context>
<domain>sdk/compozy</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/infra/store, engine/project, engine/workflow</dependencies>
</task_context>

# Task 22.0: Registration: Projects/Workflows (S)

## Overview

Implement SDK-built project and workflow registration in the engine's resource store. This task extends `sdk/compozy/integration.go` to register projects and workflows programmatically (no YAML intermediate).

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Integration layer section)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Project/Workflow builders)
</critical>

<requirements>
- Register project configuration in engine resource store
- Register all workflows within project
- Validate before registration using context
- Use logger.FromContext(ctx) for all logging
- Support hybrid SDK+YAML projects (SDK resources registered first)
- Return detailed error messages on registration failures
</requirements>

## Subtasks

- [ ] 22.1 Implement RegisterProject(ctx, *project.Config) in integration.go
- [ ] 22.2 Implement RegisterWorkflow(ctx, *workflow.Config) for each workflow
- [ ] 22.3 Add validation before registration with ctx
- [ ] 22.4 Add logging for registration events using logger.FromContext(ctx)
- [ ] 22.5 Add unit and integration tests for registration flows

## Implementation Details

High-level registration flow per 02-architecture.md Integration Layer section:

```go
func (c *Compozy) loadProjectIntoEngine(ctx context.Context, proj *project.Config) error {
    log := logger.FromContext(ctx)

    // 1. Validate project
    if err := proj.Validate(ctx); err != nil {
        return fmt.Errorf("project validation failed: %w", err)
    }

    // 2. Register project in resource store
    resourceStore := c.server.ResourceStore()
    if err := resourceStore.RegisterProject(ctx, proj); err != nil {
        return fmt.Errorf("failed to register project: %w", err)
    }

    // 3. Register all workflows
    for _, wf := range proj.Workflows {
        if err := resourceStore.RegisterWorkflow(ctx, wf); err != nil {
            return fmt.Errorf("failed to register workflow %s: %w", wf.ID, err)
        }
    }

    log.Info("SDK project loaded successfully", "workflows", len(proj.Workflows))
    return nil
}
```

### Relevant Files

- `sdk/compozy/integration.go` (extend)
- `engine/infra/store/resource_store.go` (consumer)
- `engine/project/config.go` (reference)
- `engine/workflow/config.go` (reference)

### Dependent Files

- `sdk/project/builder.go` (producer)
- `sdk/workflow/builder.go` (producer)

## Deliverables

- Extended sdk/compozy/integration.go with project/workflow registration
- Unit tests for validation failures
- Integration tests for successful registration
- Logging at INFO level for registration events
- Error messages with workflow IDs on failure

## Tests

Integration tests from _tests.md (SDK→Engine section):

- [ ] Valid project with workflows registers successfully
- [ ] Project validation failure prevents registration
- [ ] Workflow validation failure returns specific error with ID
- [ ] Multiple workflows register in correct order
- [ ] Empty workflows array returns validation error
- [ ] Duplicate workflow IDs are detected and rejected
- [ ] Hybrid project: SDK workflows coexist with AutoLoad YAML
- [ ] logger.FromContext(ctx) is used for all log statements
- [ ] Validation errors include project name in message

## Success Criteria

- Integration tests pass with real resource store
- make lint and make test pass
- Logging provides actionable information
- Error messages include resource IDs and context
- No global singletons used (logger/config from context)
