---
status: completed
title: Documentation, Help, and Regression Cleanup
type: docs
complexity: medium
dependencies:
  - task_03
---

# Documentation, Help, and Regression Cleanup

## Overview

Finish the feature by aligning user-facing documentation, command help snapshots, and regression expectations with the shipped `exec` flow and the new `.compozy/runs/` artifact model. This task closes the loop so the repository no longer documents or tests the retired legacy path as current behavior.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST document `compozy exec` usage, prompt-source options, output-format behavior, and `.compozy/runs/` artifacts in user-facing docs
- MUST update or add command-help snapshot coverage so the new CLI contract is locked in tests
- MUST remove or update remaining product-facing references that present `.tmp/codex-prompts` as the active runtime artifact location
- MUST keep regression coverage embedded in the touched docs/help/test files rather than creating a standalone testing task
- MUST leave historical references in archived artifacts or continuity notes alone unless they are part of the active product surface
</requirements>

## Subtasks
- [x] 4.1 Update README and active user-facing docs to describe `compozy exec` and `.compozy/runs/`
- [x] 4.2 Add or refresh help snapshot fixtures and command-help tests for the new `exec` contract
- [x] 4.3 Remove active-product references to `.tmp/codex-prompts` where they no longer reflect runtime behavior
- [x] 4.4 Add final regression assertions proving docs/help/test fixtures match the shipped CLI surface
- [x] 4.5 Re-run task metadata validation and repository verification after all documentation and regression updates

## Implementation Details

Use the TechSpec sections "Executive Summary", "Monitoring and Observability", and "Development Sequencing" as the contract for what the documentation and help output must convey. This task should not reopen runtime behavior; it should codify and lock in the behavior delivered by tasks 01-03.

### Relevant Files
- `README.md` — User-facing command documentation and workspace artifact explanations should be updated here
- `internal/cli/root_test.go` — Help-text regression tests should be extended for `exec`
- `internal/cli/testdata/start_help.golden` — Existing golden-file pattern should guide new or updated help fixtures
- `internal/core/plan/input.go` — Active legacy path strings should be removed or updated if they remain user-facing
- `internal/core/run/execution_acp_integration_test.go` — End-to-end regression coverage should assert final behavior around results and artifacts
- `.compozy/tasks/exec-command/_techspec.md` — The final implementation should stay aligned with the approved technical design

### Dependent Files
- `internal/cli/root.go` — Help text comes from the command definitions implemented in task 03
- `internal/core/run/execution.go` — Final regression coverage depends on the structured-output behavior delivered in task 03

### Related ADRs
- [ADR-002: Replace `.tmp/codex-prompts` with Workspace-Scoped Run Artifacts](../adrs/adr-002.md) — Drives the documentation update away from the legacy path
- [ADR-003: Support Multi-Source Prompt Input and Structured Output for `compozy exec`](../adrs/adr-003.md) — Drives the final user-facing command contract

## Deliverables
- Updated README and active documentation for `compozy exec`
- Updated command-help tests and golden fixtures covering the `exec` contract
- Cleanup of active product references to `.tmp/codex-prompts`
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for documented and user-facing CLI behavior **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Help-text regression tests assert `exec` flags, examples, and descriptions accurately
  - [x] Snapshot or golden-fixture tests fail if `.tmp/codex-prompts` reappears in active CLI help or documented runtime paths
- Integration tests:
  - [x] README examples and command-help expectations match the shipped `exec` behavior and `.compozy/runs/` artifact model
  - [x] End-to-end regression coverage confirms the final user-facing output contract for `exec` in both text and JSON modes
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Active docs and help output describe `compozy exec` accurately
- Active product surfaces no longer describe `.tmp/codex-prompts` as the canonical artifact root
- The feature can be handed to `cy-execute-task` consumers without missing documentation or unstable help output
