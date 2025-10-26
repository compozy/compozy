## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>v2/docs</domain>
<type>documentation</type>
<scope>migration</scope>
<complexity>low</complexity>
<dependencies>task_53</dependencies>
</task_context>

# Task 54.0: Migration: Hybrid + Advanced (S)

## Overview

Create advanced migration guide covering hybrid SDK+YAML projects, complex features (knowledge, memory, MCP, runtime), and migration strategies.

<critical>
- **ALWAYS READ** tasks/prd-modules/06-migration-guide.md before starting
- **ALWAYS READ** tasks/prd-modules/02-architecture.md (hybrid project integration)
- **MUST** cover hybrid project pattern (SDK + YAML coexistence)
- **MUST** include Examples 3-10 from migration guide
</critical>

<requirements>
- Cover all advanced examples (knowledge, memory, MCP, runtime, signals)
- Document hybrid SDK+YAML pattern with AutoLoad
- Provide migration strategy decision tree
- Include embedded usage pattern with compozy.New()
- Link back to basics guide for foundational patterns
</requirements>

## Subtasks

- [ ] 54.1 Document hybrid SDK+YAML pattern with AutoLoad configuration
- [ ] 54.2 Create knowledge/RAG migration example (Example 3)
- [ ] 54.3 Create memory migration example (Example 4)
- [ ] 54.4 Create MCP integration migration example (Example 5)
- [ ] 54.5 Create runtime + native tools migration example (Example 6)
- [ ] 54.6 Create signals migration example (Example 10)
- [ ] 54.7 Document embedded usage pattern (compozy.New + lifecycle)
- [ ] 54.8 Create migration strategy decision tree

## Implementation Details

**Based on:** tasks/prd-modules/06-migration-guide.md (Examples 3-10, hybrid pattern, embedded usage)

### Content Structure

```markdown
# Advanced Migration Patterns

## Migration Strategies
1. Keep YAML (no change)
2. Hybrid (SDK + YAML)
3. Full migration

[Decision tree graphic/table]

## Hybrid Projects
- AutoLoad pattern
- Mixing YAML and SDK resources
- Registration order

## Advanced Features

### Knowledge/RAG (Example 3)
**Before (YAML):** [embedders + vector_dbs + knowledge_bases]
**After (Go SDK):** [NewEmbedder + NewPgVector + NewBase]

### Memory (Example 4)
**Before (YAML):** [memories + agent references]
**After (Go SDK):** [memory.New + memory.NewReference]

### MCP Integration (Example 5)
**Before (YAML):** [mcps config]
**After (Go SDK):** [mcp.New with transport/headers]

### Runtime + Native Tools (Example 6)
**Before (YAML):** [runtime + native_tools]
**After (Go SDK):** [runtime.New + runtime.NewNativeTools]

### Signals (Example 10)
**Before (YAML):** [signal_send + signal_wait]
**After (Go SDK):** [task.NewSignal unified builder]

## Embedded Usage
[Complete embedded pattern from migration guide]
```

### Relevant Files

- tasks/prd-modules/06-migration-guide.md (Examples 3-10, hybrid, embedded)
- tasks/prd-modules/02-architecture.md (AutoLoad, integration layer)
- tasks/prd-modules/03-sdk-entities.md (all advanced builders)

### Dependent Files

- Task 53.0 deliverable (link to basics guide)

## Deliverables

- `/Users/pedronauck/Dev/compozy/compozy/v2/docs/migration-advanced.md` (new file)
  - Migration strategies section with decision tree
  - Hybrid project pattern documentation
  - 5 advanced feature migration examples (knowledge, memory, MCP, runtime, signals)
  - Complete embedded usage pattern
  - Cross-links to basics guide
- All examples must show full imports and context usage
- Hybrid pattern must include AutoLoad configuration example

## Tests

Documentation validation:
- [ ] All advanced code snippets compile
- [ ] Hybrid pattern example is complete and correct
- [ ] AutoLoad configuration matches engine expectations
- [ ] Embedded usage pattern includes all lifecycle methods
- [ ] All 5 advanced examples are present and complete
- [ ] Decision tree helps users choose migration strategy
- [ ] Links to basics guide work correctly

Manual verification:
- [ ] Test hybrid project pattern with real YAML + SDK mix
- [ ] Verify embedded usage pattern starts/stops correctly
- [ ] Compile all advanced examples in isolation
- [ ] Verify AutoLoad behavior matches documentation

## Success Criteria

- User can choose appropriate migration strategy from decision tree
- Hybrid pattern is clear with working AutoLoad example
- Advanced features have complete migration paths
- Embedded usage pattern is production-ready
- Document complements basics guide without duplication
- All examples reference context-first pattern from basics guide
