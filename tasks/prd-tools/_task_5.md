---
status: completed
parallelizable: true
blocked_by: ["2.0", "3.0", "4.0"]
---

<task_context>
<domain>engine/llm</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>configuration</dependencies>
<unblocks>["7.0", "8.0"]</unblocks>
</task_context>

# Task 5.0: Integrate builtin registration, kill switch, and configuration surface

## Overview

Wire the native cp\_\_ tools into the LLM service startup, enforce namespace reservation, and expose the kill-switch configuration that can revert to Bun-backed tools. Ensure runtime respects feature flag states and configuration defaults.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Invoke `builtin.RegisterBuiltins` within `llm.NewService` before loading runtime tool adapters, ensuring cp__ namespace reservation and conflict detection.
- Implement feature flag `native_tools.enabled` (default true) and fallback behavior that reverts to Bun tool registration when disabled.
- Surface configuration overrides for root directory, exec allowlist append list, HTTP limits, and observability toggles via `config.FromContext`.
- Add startup logs confirming activation state and configuration summary (sanitized for sensitive values).
- Ensure tool registry listings reflect cp__ tools when enabled and omit them when disabled.
</requirements>

## Subtasks

- [x] 5.1 Modify `engine/llm/service.go` to call builtin registration and handle namespace reservations.
- [x] 5.2 Implement kill-switch evaluation and fallback to Bun registration path.
- [x] 5.3 Wire configuration structures into service context and propagate to builtin handlers.
- [x] 5.4 Add unit tests covering enabled/disabled states and namespace conflict handling.

## Sequencing

- Blocked by: 2.0, 3.0, 4.0
- Unblocks: 7.0, 8.0
- Parallelizable: Yes (with Task 6.0 once dependencies satisfied)

## Implementation Details

Use tech spec "Integration Points" and "Development Sequencing" guidance. Maintain compliance with context/logger usage rules. Ensure kill switch toggles take effect without redeploy by reading config at startup and via hot reload mechanisms if available.

### Relevant Files

- `engine/llm/service.go`
- `engine/tool/registry.go`
- `pkg/config/native_tools.go`

### Dependent Files

- `engine/runtime/bun_manager.go`
- `engine/llm/tool_registry.go`

## Success Criteria

- Integration tests confirm cp\_\_ tools registered when flag enabled and absent otherwise.
- Logs clearly indicate activation state and configuration summary.
- Bun fallback path remains functional when kill switch is set.
