## status: pending

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

- [ ] 52.1 Create sdk/examples/10_complete_project.go:
  - [ ] Models (multiple providers)
  - [ ] Embedder + VectorDB
  - [ ] Knowledge bases
  - [ ] Memory (full features)
  - [ ] MCP (remote + local)
  - [ ] Runtime + native tools
  - [ ] Monitoring (Prometheus + tracing)
  - [ ] Agents with all integrations
  - [ ] Workflows with multiple task types
  - [ ] Schedules
  - [ ] Complete project assembly
  - [ ] Embedded Compozy lifecycle
- [ ] 52.2 Create sdk/examples/11_debugging.go:
  - [ ] Error accumulation examples
  - [ ] BuildError handling
  - [ ] Config inspection (AsMap())
  - [ ] Manual validation examples
  - [ ] Performance monitoring
  - [ ] Debug logging setup
  - [ ] Logger from context pattern
- [ ] 52.3 Add comprehensive comments:
  - [ ] Feature section headers
  - [ ] When to use each feature
  - [ ] Common patterns
  - [ ] Debugging tips
- [ ] 52.4 Create detailed README section:
  - [ ] Prerequisites (Postgres, Redis, Temporal)
  - [ ] Environment variables
  - [ ] Run instructions
  - [ ] Debugging guide
  - [ ] Common issues
- [ ] 52.5 Test both examples run successfully

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

- [ ] sdk/examples/10_complete_project.go (runnable)
- [ ] sdk/examples/11_debugging.go (runnable)
- [ ] Comprehensive README.md section:
  - [ ] Prerequisites and setup
  - [ ] Environment variables
  - [ ] Run instructions
  - [ ] Feature guide
  - [ ] Debugging guide
  - [ ] Troubleshooting section
- [ ] Comments organizing feature sections
- [ ] All 30 builders demonstrated
- [ ] Debugging patterns shown
- [ ] Verified both examples run successfully

## Tests

From _tests.md:

- Complete example validation:
  - [ ] Code compiles without errors
  - [ ] All builders used correctly
  - [ ] All integrations configured
  - [ ] Monitoring setup works
  - [ ] Embedded Compozy lifecycle works
  - [ ] Example runs end-to-end

- Debugging example validation:
  - [ ] BuildError aggregation demonstrated
  - [ ] Multiple errors reported correctly
  - [ ] Config inspection works (AsMap())
  - [ ] Manual validation examples work
  - [ ] Performance monitoring accurate
  - [ ] Debug logging setup correct
  - [ ] Logger from context pattern followed

## Success Criteria

- Complete example demonstrates ALL SDK features
- Debugging example shows all troubleshooting patterns
- README provides comprehensive guide
- Comments organize features into logical sections
- Prerequisites and setup clearly documented
- Both examples run end-to-end successfully
- Code passes `make lint`
- Examples serve as reference for SDK users
