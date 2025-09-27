---
status: completed
parallelizable: false
blocked_by: []
---

<task_context>
<domain>engine/tool/builtin</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
<unblocks>["2.0", "3.0", "4.0", "5.0"]</unblocks>
</task_context>

# Task 1.0: Establish builtin tool framework and shared validation utilities

## Overview

Create the foundational Go packages required to host cp\_\_ native tools, including the builtin registry, shared validation helpers, canonical error catalog, and configuration structures. This unlocks downstream tool implementations by providing consistent contracts and context-aware wiring.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Scaffold `engine/tool/builtin` with clear package boundaries (registry, filesystem, exec, fetch, validation).
- Implement `RegisterBuiltins(ctx, registry, opts)` ensuring logger/config retrieval via context helpers.
- Define `BuiltinDefinition` / `BuiltinTool` types plus schema references for JSON argument/response structures.
- Add shared validation utilities for path cleaning, sandbox enforcement helpers, and error constructors emitting canonical codes (`InvalidArgument`, `PermissionDenied`, `FileNotFound`, `CommandNotAllowed`, `Internal`).
- Introduce configuration structs in `pkg/config` (`NativeTools.RootDir`, `NativeTools.Exec`) and expose defaults aligned with tech spec guardrails.
</requirements>

## Subtasks

- [x] 1.1 Scaffold package directories and baseline Go files for registry and validators.
- [x] 1.2 Implement builtin definition/registration types with schema placeholders and context-aware wiring.
- [x] 1.3 Create shared validation helpers (path normalization, symlink checks, context cancellation helper).
- [x] 1.4 Add configuration structs and default values, wiring them through `config.FromContext` patterns.
- [x] 1.5 Document exposed APIs for downstream packages (README or code comments where needed).

## Sequencing

- Blocked by: None
- Unblocks: 2.0, 3.0, 4.0, 5.0
- Parallelizable: No (foundational package work)

## Implementation Details

Use the tech spec sections "Core Interfaces" and "Tool Behaviors" as implementation guides. Ensure JSON schema definitions are compatible with existing orchestrator expectations and that no global singletons are introduced. Apply greenfield assumption to avoid Bun dependencies.

### Relevant Files

- `engine/tool/builtin/registry.go`
- `engine/tool/builtin/validation.go`
- `pkg/config/native_tools.go`

### Dependent Files

- `engine/llm/service.go`
- `engine/tool/registry.go`

## Success Criteria

- Package builds without unused exports and passes `make lint`.
- Canonical error codes are accessible to downstream tools.
- Configuration defaults load via `config.FromContext` without global state.
