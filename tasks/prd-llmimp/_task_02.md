## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 2.0: Improve orchestrate prompting and reflexion retry

## Overview

Harden planner prompting to enforce raw-JSON outputs (no fences), include minimal valid example, and add single corrective retry on invalid JSON with appended feedback.

<critical>
- **MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc
- **MUST READ THE SOURCES** before start the task
</critical>

<requirements>
- Strengthen instruction block in compiler to emphasize required fields
- Use JSON/structured mode on LLM client when available
- On invalid schema/JSON, append error feedback to prompt and retry once
- Cap retries; log corrective retry attempt via `logger.FromContext(ctx)`
</requirements>

## Subtasks

- [ ] Update compiler instructions (example, constraints)
- [ ] Implement retry-once with appended feedback in handler path
- [ ] Unit tests for invalid→valid response via single retry
- [ ] Ensure remediation strings align with new behavior

## Implementation Details

Use ReAct-style clarity in prompts and Reflexion loop for self-correction. Keep retry bounded to 1.

### Sources

- ReAct: Yao et al., 2022 — `https://arxiv.org/abs/2210.03629`
- Reflexion: Shinn et al., 2023 — `https://arxiv.org/abs/2303.11366`

### Relevant Files

- `engine/tool/builtin/orchestrate/planner/compiler.go`
- `engine/tool/builtin/orchestrate/handler.go`

### Dependent Files

- `engine/tool/builtin/orchestrate/planner/compiler_test.go`
- `engine/tool/builtin/orchestrate/handler_test.go`

## Success Criteria

- First invalid reply followed by one corrective retry produces valid plan
- Tests assert corrective-feedback content; no markdown fences in responses
