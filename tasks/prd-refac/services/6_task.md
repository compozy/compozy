---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>interfaces,shared</dependencies>
</task_context>

# Task 6.0: Router Task Orchestrator

## Overview

Implement the router task orchestrator that conditionally routes execution to different tasks based on dynamic conditions. Router tasks evaluate expressions and direct workflow execution accordingly.

## Subtasks

- [ ] 6.1 Create RouterOrchestrator struct with routing logic
- [ ] 6.2 Implement CreateState with route configuration validation
- [ ] 6.3 Create RouteEvaluator for condition evaluation
- [ ] 6.4 Implement route selection logic based on conditions
- [ ] 6.5 Handle default route when no conditions match
- [ ] 6.6 Implement HandleResponse with routing decision output
- [ ] 6.7 Write unit tests for various routing scenarios
- [ ] 6.8 Write integration tests with downstream tasks

## Implementation Details

### Router Orchestrator (engine/task2/router/orchestrator.go)

```go
type Orchestrator struct {
    *shared.BaseOrchestrator
    routeEvaluator *RouteEvaluator
}

func (o *Orchestrator) HandleResponse(ctx context.Context, input interfaces.HandleResponseInput) (*task.Response, error) {
    routerConfig := o.getRouterConfig(input.State)

    // Evaluate routes in order
    selectedRoute, err := o.routeEvaluator.SelectRoute(ctx, routerConfig.Routes, input.Output)
    if err != nil {
        return nil, fmt.Errorf("route evaluation failed: %w", err)
    }

    // Use default if no route matched
    if selectedRoute == nil && routerConfig.DefaultRoute != nil {
        selectedRoute = routerConfig.DefaultRoute
    }

    if selectedRoute == nil {
        return nil, fmt.Errorf("no route matched and no default route configured")
    }

    response := &task.Response{
        Status: core.StatusSuccess,
        Output: input.Output,
    }
    (*response.Output)["selected_route"] = selectedRoute.Name
    (*response.Output)["next_task"] = selectedRoute.TaskRef

    return response, nil
}
```

### Key Features

- Condition evaluation using expression engine
- Ordered route evaluation
- Default route handling
- Route metadata in output
- Expression validation
- Multiple condition support per route

## Success Criteria

- Router correctly evaluates conditions and selects routes
- Expression evaluation handles all supported operators
- Default route works when no conditions match
- Route selection is deterministic and testable
- Output includes routing decision metadata
- Integration with workflow engine functions correctly
- Comprehensive test coverage for routing logic

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
