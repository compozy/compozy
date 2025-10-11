## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 1.0: Enhance plan schema and compiler defaults

## Overview

Enforce strict plan schema (required fields across nesting), inject safe defaults during compile (e.g., `type`, `status`), and validate via compiled schema instance to eliminate invalid plan errors.

<critical>
- **MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc
- **MUST READ THE SOURCES** before start the task
</critical>

<requirements>
- Validate using a compiled `schema.Schema` instance (no raw map ops)
- Default missing step `type` to `agent` and `status` to `pending`
- Cap total steps by `Options.MaxSteps` and return explicit error if exceeded
- Normalize placeholders and convert `inputs[]` → `with{}` consistently
- Keep external `spec.Plan` stable (backwards-compat not required, but avoid breaking public DTOs unintentionally)
- Use `logger.FromContext(ctx)` and `config.FromContext(ctx)` patterns
</requirements>

## Subtasks

- [ ] Update spec to require fields at all levels (plan, steps, agent, parallel)
- [ ] Implement default injection and normalization before JSON decode
- [ ] Enforce `MaxSteps` with clear error message
- [ ] Extend unit tests: defaults injection, required fields, inputs→with
- [ ] Update remediation strings (if needed) to align with new errors

## Implementation Details

Relevant insights from `_techspec.md` Section 2.1 (Plan-and-Solve; schema defaults). Integrate defaults before decode, then validate against extended schema.

### Sources

- Plan-and-Solve: Wang et al., 2023 — `https://arxiv.org/abs/2305.04091`
- Structured outputs guidance (industry): Anthropic eng. blogs

### Relevant Files

- `engine/tool/builtin/orchestrate/planner/compiler.go`
- `engine/tool/builtin/orchestrate/spec/*`

### Dependent Files

- `engine/tool/builtin/orchestrate/planner/schema_strict_test.go`
- `engine/tool/builtin/orchestrate/planner/normalize_test.go`
- `engine/tool/builtin/orchestrate/planner/compiler_test.go`

## Success Criteria

- Invalid shapes produce specific errors; valid raw plans compile after defaults
- All new and existing planner tests pass (>80% package coverage)
- No linter violations; functions ≤ 50 lines; clear error messages
