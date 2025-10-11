## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 3.0: Strengthen FSM and parallel execution semantics

## Overview

Improve robustness for parallel groups with explicit timeouts and merge strategies, ensure clear end-state logs, and maintain cancellation responsiveness.

<critical>
- **MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc
- **MUST READ THE SOURCES** before start the task
</critical>

<requirements>
- Add per-step timeout derived from config with context inheritance
- Implement merge policies for parallel groups: all-success, first-success, best-of
- Preserve WaitForCancellation patterns and propagate cancellations
- Produce structured logs for group summaries and child completions
</requirements>

## Subtasks

- [ ] Add timeout plumbing and configuration access via `config.FromContext(ctx)`
- [ ] Implement merge policy enum + aggregator
- [ ] Enhance logging at group and child completion points
- [ ] Unit tests for empty group, timeouts, and each merge policy

## Implementation Details

Maintain current plan format; implement policy inside executor. Keep functions ≤ 50 lines by extracting helpers.

### Sources

- Graph of Thoughts: Besta et al., 2023 — `https://arxiv.org/abs/2308.09687`
- CrewAI parallel orchestration patterns — `https://github.com/crewAIInc/crewAI`

### Relevant Files

- `engine/tool/builtin/orchestrate/executor.go`

### Dependent Files

- `engine/tool/builtin/orchestrate/executor_test.go`

## Success Criteria

- Parallel groups behave per selected policy; cancellations/timeouts verified
- Logs contain aggregate summaries and child logs; tests pass with coverage
