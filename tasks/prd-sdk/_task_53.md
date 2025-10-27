## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>sdk/docs</domain>
<type>documentation</type>
<scope>migration</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 53.0: Migration: YAML → SDK Basics (S)

## Overview

Create basic migration guide covering fundamental YAML-to-SDK patterns for simple use cases: project setup, models, workflows, agents, and basic tasks.

<critical>
- **ALWAYS READ** tasks/prd-sdk/06-migration-guide.md before starting
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (context-first patterns)
- **MUST** cover context setup (logger/config) in all examples
- **MUST** show side-by-side YAML → Go SDK comparisons
</critical>

<requirements>
- Cover Examples 1-2 from migration guide (simple project + workflow)
- Document context setup pattern clearly
- Provide copy-paste ready code snippets
- Include validation error examples
- Link to full migration guide for advanced patterns
</requirements>

## Subtasks

- [x] 53.1 Document context setup pattern (logger.WithLogger, config.WithConfig)
- [x] 53.2 Create YAML → SDK comparison for project configuration
- [x] 53.3 Create YAML → SDK comparison for model configuration
- [x] 53.4 Create YAML → SDK comparison for workflow + agent
- [x] 53.5 Create YAML → SDK comparison for basic task types
- [x] 53.6 Add validation error handling examples
- [x] 53.7 Add troubleshooting section for common beginner errors

## Implementation Details

**Based on:** tasks/prd-sdk/06-migration-guide.md (Examples 1-2)

### Content Structure

```markdown
# Quick Start: YAML → SDK Migration

## Context Setup (Required for All Examples)
[Show logger/config setup pattern]

## 1. Simple Project
**Before (YAML):** [Example 1 from migration guide]
**After (Go SDK):** [Example 1 from migration guide]

## 2. Workflow with Agent
**Before (YAML):** [Example 2 from migration guide]
**After (Go SDK):** [Example 2 from migration guide]

## Common Patterns
- Environment variables: os.Getenv()
- Template expressions: same as YAML
- Validation errors: Build(ctx) error handling

## Troubleshooting
- Import errors → go get/mod tidy
- Validation errors → check Build() error messages
- Context missing → see context setup section
```

### Relevant Files

- tasks/prd-sdk/06-migration-guide.md (source content Examples 1-2)
- tasks/prd-sdk/02-architecture.md (context-first patterns)
- tasks/prd-sdk/05-examples.md (runnable examples)

### Dependent Files

None (standalone documentation task)

## Deliverables

- `/Users/pedronauck/Dev/compozy/compozy/sdk/docs/migration-basics.md` (new file)
  - Context setup section (mandatory first section)
  - 2 complete side-by-side examples (project + workflow)
  - Common patterns section
  - Troubleshooting section with 3+ common errors
- Examples must be copy-paste ready with all imports
- All code snippets must include `ctx` parameter in Build() calls

## Tests

Documentation validation:
- [x] All code snippets compile without errors
- [x] Context setup pattern is correct (logger.WithLogger + config.WithConfig)
- [x] YAML examples match Go SDK output semantically
- [x] Error handling examples show proper BuildError usage
- [x] Links to full migration guide work
- [x] Troubleshooting covers: import errors, validation errors, context missing

Manual verification:
- [x] Copy each Go snippet to test file and verify it compiles
- [x] Run `go vet` on example snippets
- [x] Verify imports are complete and correct

## Success Criteria

- User can copy-paste first example and get working SDK code
- Context setup is clear and explained before any examples
- Side-by-side comparison makes migration path obvious
- Troubleshooting section covers 80% of beginner issues
- Document links to advanced patterns in full migration guide
