---
status: completed # Options: pending, in-progress, completed, excluded
parallelizable: true # Whether this task can run in parallel when preconditions are met
blocked_by: ["1.0"] # List of task IDs that must be completed first
---

<task_context>
<domain>engine/attachment</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>external_apis|http_client|filesystem</dependencies>
<unblocks>"3.0", "5.0"</unblocks>
</task_context>

# Task 2.0: Resolvers & Resource Management

## Overview

Implement the resolver architecture with per-type resolvers, shared HTTP/filesystem helpers, MIME detection, and robust resource management. This task focuses on the core resolution logic that safely downloads, opens, and manages attachment resources with proper cleanup and security constraints.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- MIME detection using `net/http.DetectContentType` + `github.com/gabriel-vasile/mimetype` fallback
- Shared HTTP helper with timeouts, redirects, size caps, and context cancellation
- Shared filesystem helper with CWD-awareness and path traversal prevention
- Resolver factory pattern selecting resolvers by `Attachment.Type()`
- Per-type resolvers with type-specific allowlists and limits
- All file-based `Resolved` handles must correctly manage temp files and implement `Cleanup()`
- Resource cleanup must work on success, failure, and panic paths
</requirements>

## Subtasks

- [x] 2.1 Implement MIME detection logic (`mimedetect.go`) using `net/http.DetectContentType` + `mimetype` fallback
- [x] 2.2 Implement shared HTTP helper (`resolver_http.go`) with timeouts, redirects, size caps, and context cancellation
- [x] 2.3 Implement shared Filesystem helper (`resolver_fs.go`) with CWD-awareness and path traversal prevention
- [x] 2.4 Implement resolver factory (`resolver_factory.go`) to select resolvers based on `Attachment.Type()`
- [x] 2.5 Implement per-type resolvers (`resolver_image.go`, `resolver_pdf.go`, `resolver_audio.go`, `resolver_video.go`, `resolver_url.go`) with type-specific allowlists and limits
- [x] 2.6 Ensure all file-based `Resolved` handles correctly manage temp files and implement `Cleanup()`
- [x] 2.7 Unit tests for MIME detection, URL/path resolution, limits, cleanup, and context cancellation

## Sequencing

- Blocked by: 1.0 (Domain Model & Core Interfaces)
- Unblocks: 3.0 (Normalization & Template Integration), 5.0 (Execution Wiring & Orchestrator Integration)
- Parallelizable: Yes (can run parallel with 4.0 after 1.0 is complete)

## Implementation Details

### MIME Detection Strategy

From technical specification:

- Use `net/http.DetectContentType` on first 512 bytes (reliable, magic bytes)
- Fallback to `github.com/gabriel-vasile/mimetype` for broader coverage
- Treat user-provided `mime` as hint only; validation uses detected MIME type

### Security & Limits Implementation

Per-type allowlists and constraints:

- **Image**: `image/*` MIME types only
- **PDF**: `application/pdf` only
- **Audio**: `audio/*` MIME types only
- **Video**: `video/*` MIME types only
- **File/URL**: Configurable allowlist (from global config)

Security measures:

- Path traversal: resolve against CWD → `filepath.Abs()` → reject if outside CWD root
- Max download size enforcement per attachment
- Timeout handling with context cancellation
- Redirect limits to prevent infinite chains

### Resource Management Pattern

Critical cleanup requirements:

- Stream to temp files; optional temp dir quota
- Always call `defer resolved.Cleanup()` immediately after successful resolve
- Handle cleanup errors appropriately (log but don't fail)
- Ensure cleanup works on panic paths

### Relevant Files

- `engine/attachment/mimedetect.go` - MIME detection logic
- `engine/attachment/resolver_http.go` - HTTP download helper
- `engine/attachment/resolver_fs.go` - Filesystem access helper
- `engine/attachment/resolver_factory.go` - Factory pattern implementation
- `engine/attachment/resolver_image.go` - Image-specific resolver
- `engine/attachment/resolver_pdf.go` - PDF-specific resolver
- `engine/attachment/resolver_audio.go` - Audio-specific resolver
- `engine/attachment/resolver_video.go` - Video-specific resolver
- `engine/attachment/resolver_url.go` - URL-only resolver
- `engine/attachment/resolver_*_test.go` - Per-resolver unit tests

### Dependent Files

- `engine/core/types.go` - For `PathCWD` type
- `pkg/logger/logger.go` - For structured logging using `logger.FromContext(ctx)`

## Success Criteria

- MIME detection correctly identifies file types and handles edge cases
- HTTP downloads respect timeouts, size limits, and redirect caps
- Filesystem access prevents path traversal attacks
- All resolvers enforce their type-specific MIME allowlists
- Resource cleanup works reliably on all code paths (success, error, panic)
- Context cancellation immediately stops network requests
- Unit tests achieve >85% coverage including error paths
- All security constraints are tested (path traversal, size limits, timeouts)
- All linter checks pass (`make lint`)
- All tests pass (`make test`)
