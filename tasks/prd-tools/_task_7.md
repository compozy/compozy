---
status: pending
parallelizable: false
blocked_by: ["2.0", "3.0", "4.0", "5.0", "6.0"]
---

<task_context>
<domain>engine/tool/builtin</domain>
<type>testing</type>
<scope>performance</scope>
<complexity>high</complexity>
<dependencies>testing</dependencies>
<unblocks>[]</unblocks>
</task_context>

# Task 7.0: Build verification suite and tests

## Overview

Deliver comprehensive automated tests validating cp\_\_ tool parity, sandbox protections, error codes, and latency improvements compared to Bun implementations.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add unit and integration tests covering success/error flows for each cp__ tool, including sandbox violations and allowlist blocks.
- Validate kill switch behavior through integration tests toggling `native_tools.enabled`.
</requirements>

## Subtasks

- [ ] 7.1 Write unit tests for filesystem, exec, and fetch tools covering canonical errors and limits.
- [ ] 7.2 Implement integration tests ensuring registry registration, kill switch behavior, and orchestrator compatibility.

## Sequencing

- Blocked by: 2.0, 3.0, 4.0, 5.0, 6.0
- Unblocks: None (final verification)
- Parallelizable: No (aggregates previous work)

## Implementation Details

Follow tech spec "Testing Approach" guidance. Use existing testing patterns (`t.Run("Should...")`) and ensure no reliance on `context.Background()`.

### Relevant Files

- `engine/tool/builtin/filesystem/*_test.go`
- `engine/tool/builtin/exec/exec_test.go`
- `engine/tool/builtin/fetch/fetch_test.go`
- `test/integration/tool/cp_builtin_test.go`

### Dependent Files

- `engine/tool/builtin/registry.go`
- `engine/runtime/bun_manager.go`

## Success Criteria

- Tests pass reliably under `make test` with cp\_\_ tools enabled/disabled.
