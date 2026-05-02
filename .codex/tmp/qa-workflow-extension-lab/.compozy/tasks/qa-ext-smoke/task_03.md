---
status: pending
title: Qa Ext Smoke QA execution and operator-flow validation
type: test
complexity: critical
dependencies:
  - task_02
---

# Task 03: Qa Ext Smoke QA execution and operator-flow validation

<!-- compozy-qa-workflow:qa-execution -->

## Overview

Execute the QA plan for this workflow against the real repository. Validate user-visible and operator-critical behavior, fix root-cause defects discovered by the tests, and persist evidence under the shared QA root.

<critical>
- ACTIVATE `/qa-execution` with `qa-output-path=.compozy/tasks/qa-ext-smoke` before executing QA
- CONSUME the QA report artifacts under `.compozy/tasks/qa-ext-smoke/qa/test-plans/` and `.compozy/tasks/qa-ext-smoke/qa/test-cases/`
- FIX production code for real bugs uncovered by QA; do not weaken tests to match broken behavior
- RUN `make verify` after fixes and keep the final verification evidence in the QA output path
</critical>

## Requirements

1. MUST use `/qa-execution` with `qa-output-path=.compozy/tasks/qa-ext-smoke`.
2. MUST execute the generated smoke and P0/P1 regression cases first.
3. MUST create bug reports for confirmed failures and link evidence to the originating test cases.
4. MUST fix root causes for regressions in production code before declaring the task complete.
5. MUST finish only after `make verify` passes.

## Success Criteria

- QA execution evidence is persisted under the workflow QA root.
- Confirmed product defects are fixed at the root cause.
- `make verify` passes with no warnings or failures.
