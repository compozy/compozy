---
status: pending
---

<task_context>
<domain>engine/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>cel_library</dependencies>
</task_context>

# Task 3.0: Implement CEL-Based Condition Evaluator

## Overview

Create secure CEL expression evaluator with resource limits and proper error handling. This component safely evaluates user-provided conditions against signal data with built-in security constraints.

## Subtasks

- [ ] 3.1 Create CELEvaluator struct with environment configuration
- [ ] 3.2 Implement NewCELEvaluator constructor with security constraints
- [ ] 3.3 Implement Evaluate method with context timeout and cost tracking
- [ ] 3.4 Add CEL type declarations for SignalEnvelope and ProcessorOutput
- [ ] 3.5 Implement cost limit enforcement and resource constraints
- [ ] 3.6 Add proper error handling with core.NewError at boundaries

## Implementation Details

Implement CELEvaluator struct with security constraints:

```go
type CELEvaluator struct {
    env     cel.Env
    options []cel.EnvOption
}

func NewCELEvaluator() (*CELEvaluator, error) {
    env, err := cel.NewEnv(
        cel.Types(&SignalEnvelope{}, &ProcessorOutput{}),
        cel.Declarations(
            decls.NewVar("signal", decls.NewObjectType("SignalEnvelope")),
            decls.NewVar("processor", decls.NewObjectType("ProcessorOutput")),
        ),
        // Security: Limit computational complexity
        cel.CostLimit(1000),
        cel.OptimizeRegex(library.BoundedRegexComplexity(100)),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create CEL environment: %w", err)
    }

    return &CELEvaluator{env: *env}, nil
}
```

Key security features:

- Cost limits to prevent DoS attacks
- Regex complexity bounds
- Type safety with declared variables
- Context timeout enforcement
- Resource limit validation

## Success Criteria

- [ ] CEL environment properly configured with security constraints
- [ ] Cost limits enforced and violations handled appropriately
- [ ] Type declarations support SignalEnvelope and ProcessorOutput
- [ ] Context timeout prevents long-running evaluations
- [ ] Error handling follows project patterns (fmt.Errorf internally, core.NewError at boundaries)
- [ ] Expression compilation and execution separated for performance

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
