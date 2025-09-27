---
status: completed
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/tool/builtin/exec</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>system_process</dependencies>
<unblocks>["5.0", "6.0", "7.0"]</unblocks>
</task_context>

# Task 3.0: Implement cp\_\_ exec tool with absolute-path allowlist and Windows fallback

## Overview

Replace the Bun-based exec tool with a Go-native implementation that enforces absolute-path execution, command allowlists, argument schema validation, and platform-specific safeguards. Provide structured logging, timeouts, and canonical error reporting.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Execute commands via `execabs.CommandContext` on Unix platforms and a validated `exec.CommandContext` fallback on Windows that verifies absolute paths with `LookPath`.
- Enforce per-command argument patterns (regex/enum) and maximum argument count; reject disallowed commands with `CommandNotAllowed` errors.
- Apply configurable timeout enforcement and kill processes on context cancellation, capturing exit codes and stderr (truncated to 1 KiB).
- Append project-level allowlists to the default command set without replacing core defaults; log configuration overrides.
- Emit structured telemetry (tool_id, command, exit_code, duration, stderr_truncated flag) per invocation.
</requirements>

## Subtasks

- [x] 3.1 Define allowlist/argument schema structures and load them from configuration.
- [x] 3.2 Implement cross-platform command execution with timeout handling and context cancellation.
- [x] 3.3 Add validation logic for arguments and produce canonical errors for blocked commands or invalid input.
- [x] 3.4 Capture stdout/stderr safely, truncating stderr and streaming to log/metrics hooks.
- [x] 3.5 Write unit tests covering allowed vs blocked commands, timeout behavior, Windows path checks, and configuration append semantics.

## Progress Notes

- Config-driven allowlists and argument schemas compiled in `engine/tool/builtin/exec/config.go`, merging defaults with overrides while logging replacements.
- Cross-platform execution implemented via `command_unix.go`/`command_windows.go`, with timeout and cancellation handling in `handler.go` and buffered stdout/stderr truncation.
- Validation helpers enforce enums, regex patterns, and additional-argument guards; canonical errors emitted via `builtin` package utilities.
- Unit tests in `engine/tool/builtin/exec/exec_test.go` cover allowlist enforcement, timeout overrides, stderr truncation, and environment handling; Windows-specific path validation handled through build-tagged command constructors.

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 6.0, 7.0
- Parallelizable: Yes (after framework exists)

## Implementation Details

Leverage `golang.org/x/sys/execabs` for Unix builds, with build tags to swap in Windows-compatible logic. Refer to tech spec "Command Execution Tooling" and "Security Considerations" sections. Ensure outputs integrate with shared error catalog.

### Relevant Files

- `engine/tool/builtin/exec/exec.go`
- `engine/tool/builtin/exec/config.go`

### Dependent Files

- `pkg/config/native_tools.go`
- `engine/tool/builtin/registry.go`

## Success Criteria

- Unit tests cover allowlist enforcement and timeout logic across platforms.
- Structured logs include exit codes and truncated stderr metadata.
- Configuration overrides append to defaults without removing baseline commands.
