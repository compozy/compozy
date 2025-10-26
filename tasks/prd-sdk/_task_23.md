## status: completed

<task_context>
<domain>sdk/compozy</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/infra/store, engine/agent, engine/tool, engine/schema</dependencies>
</task_context>

# Task 23.0: Registration: Agents/Tools/Schemas (S)

## Overview

Implement SDK-built agent, tool, and schema registration in the engine's resource store. Extends `sdk/compozy/integration.go` to register these resources programmatically alongside workflows.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Integration layer)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Agent/Tool/Schema builders)
</critical>

<requirements>
- Register all agents from project configuration
- Register all tools from project configuration
- Register all schemas with validation before registration
- Validate each resource before registration using context
- Use logger.FromContext(ctx) for logging
- Provide detailed error messages with resource IDs on failure
</requirements>

## Subtasks

- [x] 23.1 Implement RegisterAgent(ctx, *agent.Config) in integration.go
- [x] 23.2 Implement RegisterTool(ctx, *tool.Config) in integration.go
- [x] 23.3 Implement RegisterSchema(ctx, *schema.Schema) in integration.go
- [x] 23.4 Add validation for each resource type before registration
- [x] 23.5 Add unit and integration tests for all registration paths

## Implementation Details

High-level registration per 02-architecture.md:

```go
// Continuing loadProjectIntoEngine from Task 22

// 4. Register all agents
for _, agent := range proj.Agents {
    if err := resourceStore.RegisterAgent(ctx, agent); err != nil {
        return fmt.Errorf("failed to register agent %s: %w", agent.ID, err)
    }
}

// 5. Register all tools
for _, tool := range proj.Tools {
    if err := resourceStore.RegisterTool(ctx, tool); err != nil {
        return fmt.Errorf("failed to register tool %s: %w", tool.ID, err)
    }
}

// 6. Register schemas
for _, schema := range proj.Schemas {
    if err := resourceStore.RegisterSchema(ctx, schema); err != nil {
        return fmt.Errorf("failed to register schema %s: %w", GetID(schema), err)
    }
}
```

### Relevant Files

- `sdk/compozy/integration.go` (extend)
- `engine/infra/store/resource_store.go` (consumer)
- `engine/agent/config.go` (reference)
- `engine/tool/config.go` (reference)
- `engine/schema/schema.go` (reference)

### Dependent Files

- `sdk/agent/builder.go` (producer)
- `sdk/tool/builder.go` (producer)
- `sdk/schema/builder.go` (producer)

## Deliverables

- Extended sdk/compozy/integration.go with agent/tool/schema registration
- Unit tests for each registration type
- Integration tests with resource store
- Logging for each resource registration
- Error messages with resource IDs and types

## Tests

Integration tests from _tests.md:

- [x] Valid agent with actions registers successfully
- [x] Agent validation failure returns error with agent ID
- [x] Multiple agents register without conflicts
- [x] Valid tool with runtime registers successfully
- [x] Tool validation failure returns error with tool ID
- [x] Valid schema with properties registers successfully
- [x] Schema validation failure includes schema details
- [x] Duplicate agent IDs are detected and rejected
- [x] Duplicate tool IDs are detected and rejected
- [x] logger.FromContext(ctx) used for all logging
- [x] Registration order: workflows → agents → tools → schemas

## Success Criteria

- All integration tests pass with resource store
- make lint and make test pass
- Error messages are specific and actionable
- No resource leaks on registration failures
- Context-first pattern enforced throughout
