---
status: completed
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/tool/builtin/fetch</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>["5.0", "6.0", "7.0"]</unblocks>
</task_context>

# Task 4.0: Implement cp\_\_ fetch tool with HTTP safety limits

## Overview

Create the native HTTP fetch tool that enforces strict method allowlists, timeouts, body caps, redirect limits, and telemetry requirements. Provide structured responses and error reporting for network failures.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Support HTTP methods {GET, POST, PUT, PATCH, DELETE, HEAD}; reject others with `InvalidArgument`.
- Enforce default timeout (5 s), response body cap (2 MiB), and redirect limit (5) with configuration overrides logged for auditing.
- Marshal request bodies/headers safely, normalizing header casing and supporting JSON encoding for map inputs.
- Return structured output (status code, headers, latency, body with truncation flag) and canonical error codes for network failures, TLS issues, and cap breaches.
- Instrument latency, response size, and error counters for each invocation.
</requirements>

## Subtasks

- [x] 4.1 Implement HTTP client wrapper with shared timeout and redirect policy.
- [x] 4.2 Add request building logic supporting JSON encoding and header normalization.
- [x] 4.3 Enforce body cap streaming (e.g., `io.LimitReader`) and flag truncated responses.
- [x] 4.4 Emit structured telemetry and canonical errors for network failures.
- [x] 4.5 Create unit tests covering allowed methods, timeout enforcement, body truncation, and TLS error handling.

## Progress Notes

- `engine/tool/builtin/fetch/handler.go` now normalizes methods, validates bodies, enforces allowlists, and orchestrates requests through a shared client with redirect limits and timeout overrides.
- Response handling uses limited readers to enforce body caps, surfaces truncation flags, and logs structured telemetry for duration, method, and status metadata.
- Error classification converts transport failures and context deadlines into canonical builtin error codes for downstream consumption.
- Unit coverage in `engine/tool/builtin/fetch/fetch_test.go` exercises allowed/disallowed methods, timeout enforcement, body truncation, and JSON encoding of map payloads.

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 6.0, 7.0
- Parallelizable: Yes (with Tasks 2.0 and 3.0)

## Implementation Details

Refer to tech spec "Fetch Tool" behavior and "Security Considerations". Utilize context-aware HTTP clients and ensure TLS verification remains enabled by default. Document configuration overrides through structured logging.

### Relevant Files

- `engine/tool/builtin/fetch/fetch.go`
- `engine/tool/builtin/fetch/config.go`

### Dependent Files

- `pkg/config/native_tools.go`
- `engine/tool/builtin/registry.go`

## Success Criteria

- Unit tests validate enforcement of limits and allowed methods.
- Observability emits latency, response size, and error metrics.
- Default configuration achieves required timeout and body cap behavior.
