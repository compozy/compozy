## status: completed

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>medium</complexity>
<dependencies>all sdk builders</dependencies>
</task_context>

# Task 52.0: Example: All‑in‑One + Debug/Inspect (M)

## Overview

Create comprehensive "kitchen sink" example demonstrating all SDK features together, plus debugging and inspection patterns. This serves as both a complete reference and troubleshooting guide.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Examples 10 + 11)
- **MUST** demonstrate ALL builder types (30 builders)
- **MUST** include debugging and error handling examples
</critical>

<requirements>
- Two runnable examples:
  - sdk/examples/10_complete_project.go (all features)
  - sdk/examples/11_debugging.go (debug/inspect patterns)
- Complete project demonstrates: All builders, all integrations, monitoring
- Debugging example shows: Error accumulation, config inspection, validation, performance monitoring
- Clear section comments organizing features
- Comprehensive README section
</requirements>

## Subtasks

- [x] 52.1 Create sdk/examples/10_complete_project.go:
  - [x] Models (multiple providers)
  - [x] Embedder + VectorDB
  - [x] Knowledge bases
  - [x] Memory (full features)
  - [x] MCP (remote + local)
  - [x] Runtime + native tools
  - [x] Monitoring (Prometheus + tracing)
  - [x] Agents with all integrations
  - [x] Workflows with multiple task types
  - [x] Schedules
  - [x] Complete project assembly
  - [x] Embedded Compozy lifecycle
- [x] 52.2 Create sdk/examples/11_debugging.go:
  - [x] Error accumulation examples
  - [x] BuildError handling
  - [x] Config inspection (AsMap())
  - [x] Manual validation examples
  - [x] Performance monitoring
  - [x] Debug logging setup
  - [x] Logger from context pattern
- [x] 52.3 Add comprehensive comments:
  - [x] Feature section headers
  - [x] When to use each feature
  - [x] Common patterns
  - [x] Debugging tips
- [x] 52.4 Create detailed README section:
  - [x] Prerequisites (Postgres, Redis, Temporal)
  - [x] Environment variables
  - [x] Run instructions
  - [x] Debugging guide
  - [x] Common issues
- [x] 52.5 Test both examples run successfully

## Implementation Details

Per 05-examples.md sections 10 + 11:

**Complete project structure:**
- Models: OpenAI GPT-4, Anthropic Claude
- Knowledge: Embedder, PGVector, knowledge base with sources
- Memory: Full features (flush, privacy, persistence, locking)
- MCP: GitHub API (remote), filesystem (local)
- Runtime: Bun with native tools
- Monitoring: Prometheus + Jaeger tracing
- Agents: With knowledge, memory, MCP, tools
- Workflows: Multiple task types
- Schedules: Daily + weekly
- Embedded Compozy: Full lifecycle

**Debugging patterns:**
- Error accumulation in BuildError
- Multiple errors reported together
- Config inspection via AsMap()
- Manual validation timing
- Performance monitoring with time.Since()
- Debug logging setup
- Logger from context pattern

### Relevant Files

- `sdk/examples/10_complete_project.go` - Complete example
- `sdk/examples/11_debugging.go` - Debug example
- `sdk/examples/README.md` - Comprehensive guide

### Dependent Files

All sdk builder packages:
- `sdk/project/` `sdk/model/` `sdk/workflow/` `sdk/agent/`
- `sdk/task/` `sdk/knowledge/` `sdk/memory/` `sdk/mcp/`
- `sdk/runtime/` `sdk/tool/` `sdk/schema/` `sdk/schedule/`
- `sdk/monitoring/` `sdk/compozy/`

## Deliverables

- [x] sdk/examples/10_complete_project.go (runnable)
- [x] sdk/examples/11_debugging.go (runnable)
- [x] Comprehensive README.md section:
  - [x] Prerequisites and setup
  - [x] Environment variables
  - [x] Run instructions
  - [x] Feature guide
  - [x] Debugging guide
  - [x] Troubleshooting section
- [x] Comments organizing feature sections
- [x] All 30 builders demonstrated
- [x] Debugging patterns shown
- [x] Verified both examples run successfully

## Tests

From _tests.md:

- Complete example validation:
  - [x] Code compiles without errors
  - [x] All builders used correctly
  - [x] All integrations configured
  - [x] Monitoring setup works
  - [x] Embedded Compozy lifecycle works
  - [x] Example runs end-to-end

- Debugging example validation:
  - [x] BuildError aggregation demonstrated
  - [x] Multiple errors reported correctly
  - [x] Config inspection works (AsMap())
  - [x] Manual validation examples work
  - [x] Performance monitoring accurate
  - [x] Debug logging setup correct
  - [x] Logger from context pattern followed

## Success Criteria

- Complete example demonstrates ALL SDK features
- Debugging example shows all troubleshooting patterns
- README provides comprehensive guide
- Comments organize features into logical sections
- Prerequisites and setup clearly documented
- Both examples run end-to-end successfully
- Code passes `make lint`
- Examples serve as reference for SDK users
