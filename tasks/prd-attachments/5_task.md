---
status: pending # Options: pending, in-progress, completed, excluded
parallelizable: false # Whether this task can run in parallel when preconditions are met
blocked_by: ["2.0", "3.0", "4.0"] # List of task IDs that must be completed first
---

<task_context>
<domain>engine/llm|engine/task</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>llm_adapters|orchestrator</dependencies>
<unblocks>"6.0"</unblocks>
</task_context>

# Task 5.0: Execution Wiring & Orchestrator Integration

## Overview

Integrate the attachment system into the execution flow and LLM orchestrator. This task implements the merge logic for effective attachments, creates the LLM adapter integration, and removes legacy image handling. This is the critical integration point that makes attachments functional end-to-end.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Implement merge logic to compute effective attachments (task ∪ agent ∪ action) with deterministic order and de-duplication
- Apply normalization/expansion before merging to handle pluralized sources correctly
- Create `ToContentParts` helper mapping attachments to `llmadapter.ContentPart`
- Map `ImageAttachment` + URL → `ImageURLPart`, Path → `BinaryPart` with detected `image/*` MIME
- Integrate into `engine/task/uc/exec_task.go` to compute effective attachments and pass to orchestrator
- Update `engine/llm/orchestrator.go` to use attachments exclusively, removing legacy `image_url/images` handling
- Ensure integration tests verify image parts appear correctly in LLM requests
</requirements>

## Subtasks

- [ ] 5.1 Implement merge logic (`merge.go`) to compute effective attachments (task ∪ agent ∪ action) with deterministic order and de-duplication by canonical key
- [ ] 5.2 Apply normalization/expansion before merging to handle pluralized sources correctly
- [ ] 5.3 Implement `ToContentParts` helper (`to_llm_parts.go`) mapping attachments to `llmadapter.ContentPart`
- [ ] 5.4 Map `ImageAttachment` + URL → `ImageURLPart`, Path → `BinaryPart` with detected `image/*` MIME
- [ ] 5.5 Integrate into `engine/task/uc/exec_task.go` to compute effective attachments and pass to orchestrator
- [ ] 5.6 Update `engine/llm/orchestrator.go` to use attachments exclusively, removing legacy `image_url/images` handling
- [ ] 5.7 Integration tests verify image parts appear correctly; legacy fields removed; expanded attachments work

## Sequencing

- Blocked by: 2.0 (Resolvers & Resource Management), 3.0 (Normalization & Template Integration), 4.0 (Global Configuration & Schema Integration)
- Unblocks: 6.0 (Tests, Examples, and Documentation)
- Parallelizable: No (requires all foundational components to be complete)

## Implementation Details

### Merge Logic Requirements

Effective attachments computation:

- **Order**: task → agent → action (deterministic precedence)
- **De-duplication key**: `Type + normalized URL` OR `Type + canonical absolute Path` (exclude Name)
- **Metadata override**: Later metadata (Name/Meta) overwrites earlier for duplicate resources
- **Normalization first**: Apply expansion of `paths`/`urls` before merging to handle pluralized sources

### LLM Adapter Integration

Phase 1 mapping (images only):

- `ImageAttachment` + URL source → `llmadapter.ImageURLPart{URL: url, Detail: detail}`
- `ImageAttachment` + Path source → `llmadapter.BinaryPart{MIMEType: detectedMIME, Data: bytes}`

Future extensibility for non-image types:

- PDF/File/URL attachments defined but not mapped to LLM parts in Phase 1
- Architecture must support future mapping without breaking changes

### Orchestrator Integration Points

Current `engine/llm/orchestrator.go` changes:

- Remove `buildImagePartsFromInput()` method (lines 1067-1118)
- Remove legacy field handling: `image_url`, `image_urls`, `images`, `image_detail`
- Replace with `attachment.ToContentParts(ctx, effectiveAttachments)` call
- Update `buildMessages()` to use attachment-derived parts

### Task Execution Integration

In `engine/task/uc/exec_task.go`:

- Compute effective attachments during task execution setup
- Apply two-phase template resolution (Phase 2) before resolution
- Pass effective attachments to orchestrator request
- Ensure proper resource cleanup via `defer resolved.Cleanup()`

### Relevant Files

- `engine/attachment/merge.go` - Merge logic implementation
- `engine/attachment/to_llm_parts.go` - LLM adapter integration
- `engine/task/uc/exec_task.go` - Task execution integration
- `engine/llm/orchestrator.go` - Orchestrator updates (remove legacy)
- `engine/llm/orchestrator_test.go` - Update tests to use attachments

### Dependent Files

- `engine/llm/adapter/interface.go` - For `ContentPart` types
- `engine/attachment/config.go` - For attachment types
- `engine/attachment/normalize.go` - For normalization logic

## Success Criteria

- Effective attachments merge correctly with task → agent → action precedence
- De-duplication works using canonical keys (excluding metadata)
- Pluralized sources are expanded before merging
- `ImageAttachment` URLs map to `ImageURLPart` correctly
- `ImageAttachment` paths map to `BinaryPart` with detected MIME type
- Legacy `image_url/images` handling completely removed from orchestrator
- Integration tests verify image parts appear in LLM requests
- Resource cleanup works properly in execution flow
- All existing orchestrator tests updated to use attachment patterns
- All linter checks pass (`make lint`)
- All tests pass (`make test`)
- Integration tests demonstrate end-to-end attachment functionality
