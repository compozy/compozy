---
provider: manual
pr:
round: 1
round_created_at: 2026-07-16T07:23:44Z
status: resolved
file: .compozy/tasks/nested-workflows/task_05.md
line: 76
severity: medium
author: claude-code
provider_ref:
---

# Issue 006: Checked planning E2E cases have no executable coverage

## Review Comment

Task 05 marks E2E-001, E2E-002, and E2E-003 complete, but no executable test references or performs those journeys. Their only implementation-side occurrence is the static scenario table in `skills/cy-create-tasks/references/task-group-planning.md`. The Playwright suite exercises daemon web inventory flows and does not invoke `cy-create-tasks`, choose ordinary versus Task Groups, edit a proposal, or cancel the 100-task-group proposal.

Therefore `make verify` cannot detect regressions in the feature's primary planning entry point even though the task and test contract claim those cases are implemented. Add an executable skill/CLI E2E harness that asserts the exact artifacts and cancellation behavior specified by E2E-001 through E2E-003, or leave the task cases unchecked until such coverage exists.

## Triage

- Decision: `VALID`
- Root cause: Task 05 marked E2E-001 through E2E-003 complete when the static
  skill-contract table was added, but no executable CLI or browser test invokes
  `cy-create-tasks` and asserts the three required journeys.
- Evidence: The canonical `_tests.md` defines concrete planning journeys for
  those IDs. A repository-wide search finds the IDs only in the contract,
  `task-group-planning.md`, Task 05, this review file, and generated skill
  copies—not in an executable test.
- Fix approach: Keep the test contract unchanged and uncheck only the three
  falsely completed Task 05 cases. Adding the missing E2E harness would require
  production and test files outside this batch's sole code-file scope.
- Resolution: E2E-001, E2E-002, and E2E-003 are now unchecked in Task 05 until
  executable coverage implements and verifies their required journeys.
