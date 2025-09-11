---
status: pending # Options: pending, in-progress, completed, excluded
parallelizable: true # Whether this task can run in parallel when preconditions are met
blocked_by: ["5.0"] # List of task IDs that must be completed first
---

<task_context>
<domain>test|examples|docs</domain>
<type>testing|documentation</type>
<scope>validation|migration</scope>
<complexity>medium</complexity>
<dependencies>examples|documentation_system</dependencies>
<unblocks>none</unblocks>
</task_context>

# Task 6.0: Tests, Examples, and Documentation

## Overview

Comprehensive testing, example migration, and documentation for the attachments feature. This task ensures quality through extensive test coverage, provides clear migration path for users, and documents the new capabilities with practical examples.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Comprehensive unit tests covering all components with >85% coverage
- Integration tests for end-to-end attachment functionality
- Resource cleanup and context cancellation testing
- Security testing (path traversal, MIME validation, size limits)
- Migration of `examples/pokemon-img` to use new attachments syntax
- Additional examples showcasing pluralized sources and templates
- Complete documentation covering configuration, migration, and provider support
- Clear migration guide from legacy `image_url/images` to `attachments`
</requirements>

## Subtasks

- [ ] 6.1 Add comprehensive unit tests for all attachment components
- [ ] 6.2 Add integration tests for end-to-end functionality
- [ ] 6.3 Add security and resource management tests
- [ ] 6.4 Update existing examples to use attachments syntax
- [ ] 6.5 Create new examples showcasing enhanced features
- [ ] 6.6 Write comprehensive documentation
- [ ] 6.7 Create migration guide and validate documentation completeness

## Sequencing

- Blocked by: 5.0 (Execution Wiring & Orchestrator Integration)
- Unblocks: None (final task)
- Parallelizable: Yes (testing can begin incrementally as earlier tasks complete)

## Implementation Details

### Comprehensive Unit Testing

**Resolver Factory & Selection Tests:**

- Correct resolver chosen by `Attachment.Type()` for all 6 types
- Factory handles unknown types gracefully
- Source-based routing (URL vs Path) works correctly

**Per-Type Resolver Tests:**

- Success cases for each resolver type
- Size limit enforcement (over-limit downloads rejected)
- Timeout handling (slow downloads cancelled)
- MIME allowlist validation (disallowed types rejected)
- Error handling for network failures, file access errors

**Resource Cleanup Tests:**

- Verify temp file cleanup on success paths
- Verify temp file cleanup on failure paths
- Verify temp file cleanup on panic paths (using `defer` and `recover`)
- Context cancellation immediately stops network requests
- No resource leaks in concurrent scenarios

**Merge Logic Tests:**

- Task → agent → action precedence ordering
- De-duplication using canonical keys (Type + URL/Path)
- Metadata override behavior (later wins)
- Edge cases: empty lists, duplicate keys, mixed source types

**Path Traversal Prevention Tests:**

- Attempts to escape CWD are rejected
- Symlink traversal attacks blocked
- Relative path resolution works within CWD
- Edge cases: `..`, `/`, `~`, Windows paths

### Integration Testing

**End-to-End Attachment Resolution:**

- Full workflow: YAML → config → normalization → resolution → LLM parts
- Template evaluation with workflow context
- Image attachments appear correctly in LLM requests
- Resource cleanup in real execution scenarios

**Global Configuration Integration:**

- Limit enforcement works in practice
- CLI flags and environment variables affect behavior
- Configuration validation catches real-world errors

**Template Integration:**

- Two-phase resolution with workflow context
- Deferred `.tasks.*` evaluation works correctly
- Template errors provide actionable messages

### Example Updates & Creation

**Migrate `examples/pokemon-img`:**

- Replace `image_url` with `attachments` configuration
- Demonstrate both URL and path sources
- Update README and API tests
- Ensure example still works end-to-end

**New Examples:**

- **Pluralized Sources**: Example using `paths` with glob patterns
- **Template Integration**: Example using workflow input and task outputs
- **Mixed Attachment Types**: Example with images, PDFs, and files

### Documentation Requirements

**Configuration Guide:**

- Complete reference for all attachment types and options
- Global configuration settings and their effects
- YAML syntax examples for all supported patterns

**Provider Support Matrix:**

- Document which LLM providers support `BinaryPart` vs `ImageURLPart`
- Provide fallback guidance when `BinaryPart` not supported
- Update adapter documentation as needed

**Migration Guide:**

- Step-by-step migration from `image_url/images` to `attachments`
- Before/after examples showing equivalent configurations
- Breaking changes and compatibility notes
- Common migration issues and solutions

**Template Integration Guide:**

- Available template context variables
- Two-phase resolution explanation with examples
- Best practices for task chaining with attachments

### Relevant Files

**Test Files:**

- `engine/attachment/*_test.go` - Unit tests for all components
- `test/integration/attachment_test.go` - Integration tests
- `test/integration/orchestrator_attachment_test.go` - Orchestrator integration

**Example Files:**

- `examples/pokemon-img/*` - Updated example
- `examples/attachments-demo/*` - New comprehensive example
- `examples/template-attachments/*` - Template integration example

**Documentation Files:**

- `docs/content/docs/configuration/attachments.mdx` - Configuration guide
- `docs/content/docs/migration/attachments.mdx` - Migration guide
- `docs/content/docs/examples/attachments.mdx` - Usage examples

### Dependent Files

- All attachment implementation files from previous tasks
- Existing example and documentation infrastructure

## Success Criteria

- **Test Coverage**: >85% coverage on attachment-related code
- **Security**: All security tests pass (path traversal, MIME validation, size limits)
- **Resource Management**: No temp file leaks in any test scenario
- **Examples**: All examples run successfully and demonstrate key features
- **Documentation**: Complete, accurate, and user-friendly documentation
- **Migration**: Clear migration path with working before/after examples
- **CI/CD**: All tests pass in continuous integration
- **Quality**: All linter checks pass (`make lint`)
- **Integration**: All integration tests demonstrate end-to-end functionality
- **Performance**: Tests validate performance requirements (p95 <100ms for local files)
