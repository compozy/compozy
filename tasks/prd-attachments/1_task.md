---
status: pending # Options: pending, in-progress, completed, excluded
parallelizable: false # Whether this task can run in parallel when preconditions are met
blocked_by: [] # List of task IDs that must be completed first
---

<task_context>
<domain>engine/attachment</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>engine/core</dependencies>
<unblocks>"2.0", "4.0"</unblocks>
</task_context>

# Task 1.0: Domain Model & Core Interfaces

## Overview

Establish the foundational polymorphic attachment architecture with core interfaces and concrete types. This task implements the type-safe domain model that supports all attachment types (image, PDF, audio, video, URL, file) with pluralized source support and proper validation.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Define `Attachment` and `Resolved` interfaces following project interface patterns
- Implement all 6 concrete attachment types with embedded `baseAttachment`
- Support pluralized sources: `URLs []string` and `Paths []string` fields
- Polymorphic unmarshaling with `type` discriminator and alias normalization
- Struct-level validation ensuring exactly one source type per attachment
- Robust `Cleanup()` method design for resource management
</requirements>

## Subtasks

- [ ] 1.1 Define `Attachment` and `Resolved` interfaces in `engine/attachment/resolver.go`
- [ ] 1.2 Implement `baseAttachment` struct and all concrete types in `engine/attachment/config.go`
- [ ] 1.3 Add pluralized source support: `URLs []string` and `Paths []string` fields to applicable concrete types
- [ ] 1.4 Implement polymorphic `UnmarshalYAML`/`UnmarshalJSON` with `type` discriminator and alias normalization (`document` → `pdf`)
- [ ] 1.5 Add struct-level validation to ensure exactly one of `url`, `path`, `urls`, or `paths` is present per attachment
- [ ] 1.6 Design `Resolved` interface with robust `Cleanup()` method for all resource handles (file descriptors, temp files)
- [ ] 1.7 Unit tests for polymorphic unmarshaling, validation errors, and interface contracts

## Sequencing

- Blocked by: None (foundational task)
- Unblocks: 2.0 (Resolvers & Resource Management), 4.0 (Global Configuration & Schema Integration)
- Parallelizable: No (foundation for all other tasks)

## Implementation Details

### Core Interfaces

From technical specification, implement these key interfaces:

```go
// Common interface for all attachments
type Attachment interface {
    Type() Type
    Name() string
    Meta() map[string]any
    // Resolution delegates to per-type resolver via factory
    Resolve(ctx context.Context, cwd *core.PathCWD) (Resolved, error)
}

// Resolved is a transport-agnostic handle
type Resolved interface {
    AsURL() (string, bool)
    AsFilePath() (string, bool)
    Open() (io.ReadCloser, error)
    MIME() string
    Cleanup()
}
```

### Concrete Types

Implement all 6 concrete types:

- `ImageAttachment` - supports `URL`, `Path`, `URLs`, `Paths`
- `PDFAttachment` - supports `URL`, `Path`, `URLs`, `Paths`
- `AudioAttachment` - supports `URL`, `Path`, `URLs`, `Paths`
- `VideoAttachment` - supports `URL`, `Path`, `URLs`, `Paths`
- `URLAttachment` - supports `URL` only (no file fetch)
- `FileAttachment` - supports `Path` only (generic binary)

### Relevant Files

- `engine/attachment/resolver.go` - Core interfaces
- `engine/attachment/config.go` - Concrete types and validation
- `engine/attachment/config_test.go` - Unit tests

### Dependent Files

- `engine/core/types.go` - For `PathCWD` and core types
- `engine/schema/schema.go` - For validation patterns

## Success Criteria

- All 6 concrete attachment types implement `Attachment` interface correctly
- Polymorphic unmarshaling correctly instantiates concrete types based on `type` discriminator
- Alias normalization works: `document` → `pdf`
- Validation rejects configurations with multiple or no source fields
- `Resolved` interface design supports cleanup for all resource types
- Unit tests achieve >90% coverage on core domain logic
- All linter checks pass (`make lint`)
- All tests pass (`make test`)
