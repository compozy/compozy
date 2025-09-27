---
status: completed
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/tool/builtin/filesystem</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>filesystem</dependencies>
<unblocks>["5.0", "6.0", "7.0"]</unblocks>
</task_context>

# Task 2.0: Implement filesystem cp\_\_ tools with sandboxing and limits

## Overview

Build the native filesystem tool set (`cp__read_file`, `cp__write_file`, `cp__delete_file`, `cp__list_dir`, `cp__grep`) using the shared framework. Enforce path sandboxing, symlink protection, concurrency-safe traversal, and binary detection heuristics as defined in the PRD/tech spec.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Resolve request paths against `config.NativeTools.RootDir`, rejecting escapes and symlink hops for write/delete operations.
- Implement atomic directory creation, POSIX-safe file modes, and context cancellation checks for long-running operations.
- Enforce traversal limits (`maxResults`, `maxFilesVisited`, `maxFileBytes`) and skip binary files using the specified heuristic (first 8 KiB null-byte or >30% non-printables).
- Return structured outputs consistent with schemas (content, metadata, entries, matches) and canonical error codes for invalid input, permission issues, or sandbox violations.
- Cover edge cases: missing files, relative paths, recursive directory operations, grep on large trees.
</requirements>

## Subtasks

- [x] 2.1 Implement path resolution and sandbox validator leveraging shared helpers.
- [x] 2.2 Build read/write/delete operations with symlink denial and atomic directory creation.
- [x] 2.3 Implement list directory traversal with pagination, filters, and context-aware stopping.
- [x] 2.4 Implement grep streaming with limits and binary-file detection heuristic.
- [x] 2.5 Wire schemas and output types; add unit tests covering success/error scenarios per tool.

## Progress Notes

- Implemented cp\_\_ filesystem toolset with sandbox enforcement in `engine/tool/builtin/filesystem/*.go`, leveraging shared validation helpers for path normalization.
- Added comprehensive unit coverage for read, write, delete, list, and grep flows under `engine/tool/builtin/filesystem/*_test.go`, covering sandbox escapes, traversal limits, and binary detection heuristics.
- Configuration defaults and schemas wired through `pkg/config/native_tools.go` and registry integration, enabling downstream tasks.

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 6.0, 7.0
- Parallelizable: Yes (can run alongside Tasks 3.0 and 4.0 once unblocked)

## Implementation Details

Follow "Tool Behaviors" in the tech spec. Ensure operations leverage shared validation from Task 1.0 and respect context deadlines. For recursion limits, use iterators or channels rather than loading entire directory trees into memory.

### Relevant Files

- `engine/tool/builtin/filesystem/read_file.go`
- `engine/tool/builtin/filesystem/write_file.go`
- `engine/tool/builtin/filesystem/list_dir.go`
- `engine/tool/builtin/filesystem/grep.go`

### Dependent Files

- `engine/tool/builtin/registry.go`
- `engine/tool/builtin/validation.go`

## Success Criteria

- All filesystem tools pass unit tests covering sandbox enforcement, error codes, and limit handling.
- Benchmark script demonstrates latency improvement compared to Bun versions for representative workloads.
- No lint or gofmt issues; path handling audited for symlink and traversal attacks.
