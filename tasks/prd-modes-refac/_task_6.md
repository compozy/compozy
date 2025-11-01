## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>config</dependencies>
</task_context>

# Task 6.0: Documentation Update

## Overview

This task updates all user-facing documentation to remove references to "standalone" mode and replace them with accurate descriptions of memory, persistent, and distributed modes. This ensures documentation matches the implementation exactly.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the _techspec.md and the _prd.md docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Update Redis configuration documentation to show memory/persistent/distributed modes
- Update Temporal architecture documentation to reference memory/persistent modes
- Update mode configuration documentation
- Update deployment documentation
- Update CLI command documentation
- Replace all "standalone" mode references with correct mode names
- Update YAML examples to show correct modes
- Ensure all documentation matches implementation
</requirements>

## Subtasks

- [x] 6.1 Update `docs/content/docs/configuration/redis.mdx` - Replace "standalone" with memory/persistent/distributed
- [x] 6.2 Update `docs/content/docs/configuration/redis.mdx` - Change default mode documentation
- [x] 6.3 Update `docs/content/docs/configuration/redis.mdx` - Replace "Standalone Mode" section with "Embedded Modes"
- [x] 6.4 Update `docs/content/docs/architecture/embedded-temporal.mdx` - Update description and callout
- [x] 6.5 Update `docs/content/docs/architecture/embedded-temporal.mdx` - Replace all "standalone mode" references
- [x] 6.6 Update `docs/content/docs/architecture/embedded-temporal.mdx` - Update YAML examples
- [x] 6.7 Update `docs/content/docs/configuration/mode-configuration.mdx` - Verify mode descriptions
- [x] 6.8 Update `docs/content/docs/deployment/temporal-modes.mdx` (if exists) - Replace standalone references
- [x] 6.9 Update `docs/content/docs/cli/compozy-start.mdx` (if exists) - Update mode examples
- [x] 6.10 Search for any other documentation files referencing "standalone" mode
- [x] 6.11 Verify all YAML examples show correct modes (memory/persistent/distributed)
- [x] 6.12 Verify documentation builds successfully

## Implementation Details

See Phase 1.3 in the techspec for detailed implementation steps.

Key changes:
- Redis docs: Change from "standalone/distributed" to "memory/persistent/distributed"
- Temporal docs: Update descriptions to reference memory/persistent modes
- All YAML examples: Update `mode: standalone` to `mode: memory` or `mode: persistent`
- Mode descriptions: Clarify when to use each mode

### Relevant Files

- `docs/content/docs/configuration/redis.mdx`
- `docs/content/docs/architecture/embedded-temporal.mdx`
- `docs/content/docs/configuration/mode-configuration.mdx`
- `docs/content/docs/deployment/temporal-modes.mdx` (if exists)
- `docs/content/docs/cli/compozy-start.mdx` (if exists)
- All other documentation files referencing modes

### Dependent Files

- Documentation build system (`docs/next.config.mjs`)
- Example files may reference documentation

## Deliverables

- All documentation updated to reference memory/persistent/distributed modes
- All YAML examples show correct modes
- All "standalone" references removed (except historical context)
- Documentation builds successfully
- Documentation is accurate and matches implementation
- User-facing guides are clear and helpful

## Tests

- Documentation tests:
  - [x] Verify documentation builds without errors (`cd docs && npm run build`)
  - [x] Verify all YAML examples are syntactically correct
  - [x] Verify no broken links in documentation

- Manual verification:
  - [x] Review Redis configuration documentation
  - [x] Review Temporal architecture documentation
  - [x] Review mode configuration documentation
  - [x] Verify examples match actual implementation
  - [x] Run grep to verify no "standalone" mode references remain

## Success Criteria

- All documentation references memory/persistent/distributed modes
- All YAML examples show correct modes
- No "standalone" mode references in user-facing documentation
- Documentation builds successfully
- Documentation is accurate and clear
- Examples are correct and work as documented
- All links work correctly
