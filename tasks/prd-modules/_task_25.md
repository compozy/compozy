## status: pending

<task_context>
<domain>v2/compozy</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>v2/project, v2/workflow, v2/agent, engine/infra</dependencies>
</task_context>

# Task 25.0: Validation & Linking Orchestration (S)

## Overview

Implement orchestration logic to validate and link SDK resources before engine registration. This ensures $ref resolution, dependency ordering, and cross-resource validation work correctly for SDK-built projects.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-modules/02-architecture.md (Resource resolution)
- **ALWAYS READ** tasks/prd-modules/03-sdk-entities.md (Project builder)
</critical>

<requirements>
- Validate all $ref references resolve to valid resource IDs
- Ensure registration order respects dependencies
- Cross-validate agent→tool, agent→knowledge, agent→memory references
- Use logger.FromContext(ctx) for validation logging
- Provide detailed error messages for missing references
- Support hybrid SDK+YAML reference resolution
</requirements>

## Subtasks

- [ ] 25.1 Implement ValidateReferences(ctx, *project.Config) in integration.go
- [ ] 25.2 Implement dependency graph construction for registration order
- [ ] 25.3 Add cross-resource validation (agent→tool, workflow→agent, etc.)
- [ ] 25.4 Add logging for validation warnings and errors
- [ ] 25.5 Add unit tests for reference validation and dependency ordering

## Implementation Details

High-level validation orchestration:

```go
func (c *Compozy) validateAndLink(ctx context.Context, proj *project.Config) error {
    log := logger.FromContext(ctx)

    // 1. Build resource ID index
    resourceIndex := buildResourceIndex(proj)

    // 2. Validate all $ref references
    if err := validateReferences(ctx, proj, resourceIndex); err != nil {
        return fmt.Errorf("reference validation failed: %w", err)
    }

    // 3. Validate cross-resource dependencies
    if err := validateDependencies(ctx, proj); err != nil {
        return fmt.Errorf("dependency validation failed: %w", err)
    }

    // 4. Check for circular dependencies
    if err := detectCircularDependencies(ctx, proj); err != nil {
        return fmt.Errorf("circular dependency detected: %w", err)
    }

    log.Info("validation and linking complete", "resources", len(resourceIndex))
    return nil
}
```

Key validations:
- Workflow tasks reference valid agent/tool IDs
- Agent actions reference valid tool IDs
- Agent knowledge bindings reference valid knowledge base IDs
- Agent memory references reference valid memory IDs
- Knowledge bases reference valid embedder and vectorDB IDs
- No circular workflow dependencies

### Relevant Files

- `v2/compozy/integration.go` (extend)
- `v2/compozy/validation.go` (new)
- `engine/project/config.go` (reference)

### Dependent Files

- All builder packages (producers)
- `engine/infra/store/resource_store.go` (consumer)

## Deliverables

- v2/compozy/validation.go with reference validation logic
- Extended integration.go to call validation before registration
- Unit tests for all validation scenarios
- Detailed error messages with resource IDs and reference paths
- Logging for validation warnings (e.g., unused resources)

## Tests

Unit tests from _tests.md (Validation section):

- [ ] Valid project with all references passes validation
- [ ] Workflow referencing non-existent agent returns error
- [ ] Agent referencing non-existent tool returns error
- [ ] Agent referencing non-existent knowledge base returns error
- [ ] Agent referencing non-existent memory returns error
- [ ] Knowledge base referencing non-existent embedder returns error
- [ ] Knowledge base referencing non-existent vectorDB returns error
- [ ] Circular workflow dependency is detected and rejected
- [ ] Hybrid project: SDK references resolve YAML resources
- [ ] Hybrid project: YAML references resolve SDK resources
- [ ] logger.FromContext(ctx) used for logging
- [ ] Error messages include full reference path (e.g., workflow.task.agent.tool)

## Success Criteria

- All validation tests pass
- Error messages clearly identify missing resources
- Registration order respects dependencies
- Hybrid SDK+YAML reference resolution works
- make lint and make test pass
- Validation completes <10ms for typical projects
