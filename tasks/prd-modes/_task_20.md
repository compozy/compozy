## status: completed

<task_context>
<domain>schemas</domain>
<type>schema_update</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 20.0: Update JSON Schemas

## Overview

Update JSON schemas (`config.json` and `compozy.json`) to reflect the new three-mode system (memory/persistent/distributed), replacing references to the old standalone mode.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from `_techspec.md` Phase 5.1 before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha
</critical>

<research>
# When you need information about JSON Schema:
- use perplexity and context7 to find out how to properly define enum types and defaults
- validate against JSON Schema specification standards
</research>

<requirements>
- Update mode enums to: ["memory", "persistent", "distributed"]
- Change default mode from "distributed" to "memory"
- Update all mode-related descriptions and help text
- Update component-level mode fields (temporal.mode, redis.mode)
- Ensure schema validation passes for all example configs
</requirements>

## Subtasks

- [x] 20.1 Update mode enum and default in `schemas/config.json`
- [x] 20.2 Update mode enum and component modes in `schemas/compozy.json`
- [x] 20.3 Update mode descriptions and help text in both schemas
- [x] 20.4 Validate schemas against example configs

## Implementation Details

See `_techspec.md` Phase 5.1 for complete implementation details.

### Key Changes

**schemas/config.json:**
- Update mode enum: `["memory", "persistent", "distributed"]`
- Update default: `"memory"`
- Update description to explain each mode's purpose

**schemas/compozy.json:**
- Update root-level mode enum and default
- Update temporal.mode enum: `["memory", "persistent", "remote"]`
- Update redis.mode enum: `["memory", "persistent", "distributed"]`
- Add inheritance description (empty = inherit from global)

### Relevant Files

- `schemas/config.json`
- `schemas/compozy.json`

### Dependent Files

- `examples/memory-mode/compozy.yaml`
- `examples/persistent-mode/compozy.yaml`
- `examples/distributed-mode/compozy.yaml`

## Deliverables

- Updated `schemas/config.json` with new mode system
- Updated `schemas/compozy.json` with new mode system
- Schema validation passes for all example configs
- IDE autocomplete shows correct mode options

## Tests

- Schema validation tests:
  - [x] Validate memory mode config against schema
  - [x] Validate persistent mode config against schema
  - [x] Validate distributed mode config against schema
  - [x] Validate component mode override configs
  - [x] Reject invalid mode values (e.g., "standalone")
  - [x] Validate mode inheritance (component inherits from global)

## Success Criteria

- All JSON schemas updated with new mode names
- Schema validation passes for all example configs
- Default mode is "memory" in schemas
- IDE autocomplete/validation works correctly
- No references to "standalone" mode in schemas
