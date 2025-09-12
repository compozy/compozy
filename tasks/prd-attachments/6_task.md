---
status: pending # Options: pending, in-progress, completed, excluded
parallelizable: true
blocked_by: ["5.0"]
---

<task_context>
<domain>tests|examples</domain>
<type>testing</type>
<scope>validation</scope>
<complexity>medium</complexity>
<dependencies>orchestrator|examples_system</dependencies>
<unblocks>7.0</unblocks>
</task_context>

# Task 6.0: Tests & Examples for Attachments

## Overview

Plan, implement, and stabilize unit/integration tests and ship a new end‑to‑end example demonstrating the unified attachments model. Integration tests must follow existing patterns under `test/integration/**` and avoid external network I/O. The example will be renamed and expanded to cover image, audio, and video inputs with a router task.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Follow `.cursor/rules/test-standards.mdc`: `t.Run("Should …")`, `testify`, ≥80% coverage for business logic
- Use repo helpers: `test/helpers.InitializeTestConfig`, `test/integration/worker/helpers/*`
- No external network in tests; use local, small fixtures checked into `test/integration/**/fixtures`
- Integration tests verify router branching and `ContentPart` mapping end‑to‑end
- Example uses the new `attachments` spec
- `make lint` and `make test` must pass before closing this task
</requirements>

## Subtasks

- [ ] 6.1 Unit tests for attachments
  - [ ] Factory selection by `Attachment.Type()`
  - [ ] Per‑type resolvers: success paths, timeouts, size caps, redirect limits, MIME allowlist
  - [ ] Resource cleanup and context cancellation (temp files closed on all paths)
  - [ ] Merge logic ordering, de‑duplication, metadata override
  - [ ] Filesystem traversal prevention (cwd, symlinks, `..`)

- [ ] 6.2 Integration tests (end‑to‑end)
  - [ ] New package `test/integration/worker/attachments`
  - [ ] Router flow: inputs with {image|audio|video} lead to the correct branch
  - [ ] Orchestrator request contains expected `ContentPart`s (URL→`ImageURLPart`, Path→`BinaryPart`)
  - [ ] Global `attachments.*` limits applied via env/CLI mapping
  - [ ] Template deferral/evaluation across workflow context verified

- [ ] 6.3 Examples
  - [ ] Rename `examples/pokemon-img` → `examples/pokemon`
  - [ ] Single workflow with a task router → three analysis tasks (`analyze-image`, `analyze-audio`, `analyze-video`)
  - [ ] Seed minimal media under `examples/pokemon/media/` (small CC‑licensed)
  - [ ] Example only may use Perplexity to locate/download assets; tests must not
  - [ ] Update README and workflow to the new attachments spec

## Sequencing

- Blocked by: 5.0 (Execution wiring & orchestrator integration)
- Can run in parallel with 7.0 (documentation) after basic flows are stable

## Implementation Details

### Unit Tests (engine/attachment)

- Files: `engine/attachment/*_test.go`
- Cover: normalization (`paths`/`urls` globbing), resolver HTTP/FS helpers, MIME detection, limits, and `Cleanup()` behavior.
- Validate error specificity (e.g., `assert.ErrorAs`, `assert.ErrorContains`).

### Integration Tests

- Location: `test/integration/worker/attachments`
- Bootstrapping: use `TestMain` + `test/helpers.InitializeTestConfig()`
- Infra: `DatabaseHelper`, `RedisHelper`, `TemporalHelper` with `t.Cleanup` for teardown
- Router flow fixture: YAML fixture triggers image/audio/video branches; verify DB state and outputs
- Orchestrator assertions: use DynamicMockLLM or equivalent to capture request `ContentPart`s
- Config coverage: enforce `attachments.*` limits via env/CLI and assert outcomes

### Example: `examples/pokemon`

- Structure: `workflow.yaml` with an initial router task that dispatches to one of three analysis tasks
- Media: small, CC‑licensed example files (image, audio, video) under `media/`
- Acquisition note: at implementation time, use Perplexity to find and download suitable tiny files (≤1–2 MB) with clear licensing; pin URLs in README
- Spec: use `attachments` only

## Success Criteria

- `make lint` passes; all tests pass locally and in CI
- Integration tests deterministic (no network) and parallel‑safe
- Example runs locally and demonstrates all three branches
- Coverage on attachment packages improved and meaningful

## Relevant Paths (planning)

- Tests: `engine/attachment/*_test.go`, `test/integration/worker/attachments/**`
- Example: `examples/pokemon/**`
