## status: completed

<task_context>
<domain>sdk/compozy</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>engine/knowledge, engine/memory, engine/mcp</dependencies>
</task_context>

# Task 24.0: Registration: Knowledge/Memory/MCP (S)

## Overview

Implement SDK-built knowledge base, memory config, and MCP server registration in the engine's resource store. Completes the integration layer for all SDK resource types.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Integration layer completion)
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Knowledge/Memory/MCP sections)
</critical>

<requirements>
- Register all knowledge bases from project configuration
- Register all memory configurations from project configuration
- Register all MCP servers from project configuration
- Validate each resource before registration using context
- Use logger.FromContext(ctx) for logging
- Handle external dependencies (vector DBs, embedders) gracefully
</requirements>

## Subtasks

- [x] 24.1 Implement RegisterKnowledgeBase(ctx, *knowledge.BaseConfig)
- [x] 24.2 Implement RegisterMemory(ctx, *memory.Config)
- [x] 24.3 Implement RegisterMCP(ctx, *mcp.Config)
- [x] 24.4 Add validation for external dependencies (embedders, vector DBs)
- [x] 24.5 Add unit and integration tests for all paths

## Implementation Details

High-level registration per 02-architecture.md:

```go
// Continuing loadProjectIntoEngine from Tasks 22-23

// 7. Register knowledge bases
for _, kb := range proj.KnowledgeBases {
    if err := resourceStore.RegisterKnowledgeBase(ctx, kb); err != nil {
        return fmt.Errorf("failed to register knowledge base %s: %w", kb.ID, err)
    }
}

// 8. Register memory configs
for _, mem := range proj.Memories {
    if err := resourceStore.RegisterMemory(ctx, mem); err != nil {
        return fmt.Errorf("failed to register memory %s: %w", mem.ID, err)
    }
}

// 9. Register MCP servers
for _, mcp := range proj.MCPs {
    if err := resourceStore.RegisterMCP(ctx, mcp); err != nil {
        return fmt.Errorf("failed to register MCP %s: %w", mcp.ID, err)
    }
}
```

### Relevant Files

- `sdk/compozy/integration.go` (complete)
- `engine/knowledge/base_config.go` (reference)
- `engine/memory/config.go` (reference)
- `engine/mcp/config.go` (reference)

### Dependent Files

- `sdk/knowledge/base.go` (producer)
- `sdk/memory/config.go` (producer)
- `sdk/mcp/builder.go` (producer)

## Deliverables

- Completed sdk/compozy/integration.go with all resource types
- Unit tests for knowledge/memory/MCP registration
- Integration tests with external services (env-gated)
- Logging for each registration with resource details
- Error handling for missing dependencies (embedder, vectorDB)

## Tests

Integration tests from _tests.md:

- [x] Valid knowledge base with embedder and vectorDB registers successfully
- [x] Knowledge base missing embedder returns validation error
- [x] Knowledge base missing vectorDB returns validation error
- [x] Valid memory config with Redis backend registers successfully
- [x] Memory config validation failure includes memory ID
- [x] Valid MCP with stdio transport registers successfully
- [x] Valid MCP with SSE transport registers successfully
- [x] MCP validation failure includes MCP ID and transport type
- [x] Duplicate knowledge base IDs are rejected
- [x] Duplicate memory IDs are rejected
- [x] Duplicate MCP IDs are rejected
- [x] logger.FromContext(ctx) used for all logging

## Success Criteria

- All resource types can be registered successfully
- Integration tests pass with external services (when available)
- Env-gated tests skip gracefully when services unavailable
- Error messages include resource IDs and dependency info
- make lint and make test pass
- Complete integration layer ready for Phase 2
