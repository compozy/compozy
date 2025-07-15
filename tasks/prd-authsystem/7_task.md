---
status: pending
---

<task_context>
<domain>engine/auth</domain>
<type>implementation</type>
<scope>performance</scope>
<complexity>low</complexity>
<dependencies>monitoring</dependencies>
</task_context>

# Task 7.0: Metrics Instrumentation

## Overview

Expose Prometheus metrics for auth requests, failures, latency, and rate-limit blocks.

<requirements>
- Use existing `infra/monitoring` interceptor util.
- Histogram buckets align with global latency config.
</requirements>

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: @.cursor/rules/architecture.mdc
    - Go coding standards: @.cursor/rules/go-coding-standards.mdc
    - Testing requirements: @.cursor/rules/test-standard.mdc
    - API standards: @.cursor/rules/api-standards.mdc
    - Security & quality: @.cursor/rules/quality-security.md
    - GoGraph MCP tools: @.cursor/rules/gograph.mdc
    - No Backwards Compatibility: @.cursor/rules/backwards-compatibility.mdc
- **MUST** use `logger.FromContext(ctx)` - NEVER use logger as function parameter or dependency injection
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow @.cursor/rules/task-review.mdc workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [x] 7.1 Add metric definitions in `metrics.go` ✅ COMPLETED
- [x] 7.2 Increment counters in middleware & rate-limiter ✅ COMPLETED
- [x] 7.3 Grafana dashboard panel PR ✅ COMPLETED

## Success Criteria

- Metrics visible in Prometheus scrape.
- Dashboard panel shows data after smoke test.

## Initial Review

```markdown
Task File Analysis (7_task.md):
Task 7.0 is "Metrics Instrumentation" with complexity level "low". It has 3 subtasks:

- 7.1: Add metric definitions in metrics.go - Status: [ ] (not started)
- 7.2: Increment counters in middleware & rate-limiter - Status: [ ] (not started)
- 7.3: Grafana dashboard panel PR - Status: [ ] (not started)

PRD Context (\_prd.md):
The auth system has critical performance requirements that need monitoring:

- Performance Goal (G2): Auth middleware adds ≤ 5ms P99 to request path
- Security Goal (G3): ≥ 99% of unauthorized requests rejected; 0 credential-stuffing successes
- Coverage Goal (G1): ≥ 95% of requests carry a valid key within 30 days

Success metrics specifically require tracking auth coverage, middleware latency, and security events.

Tech Spec (\_techspec.md):
Defines specific metrics to be exposed:

- Counters: auth_requests_total{status="success|fail"}
- Histogram: auth_latency_seconds
- Counter: rate_limit_blocks_total

Existing Infrastructure Analysis:

- Complete OpenTelemetry setup exists in engine/infra/monitoring/
- Auth domain fully implemented in engine/auth/ with middleware
- Rate limiting metrics already implemented with rate_limit_blocks_total counter
- Standard histogram buckets pattern: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10

Potential Challenges:

1. Need to ensure metrics don't add significant overhead (≤5ms P99 requirement)
2. Must follow established OpenTelemetry patterns from existing implementations
3. Thread-safe metric initialization using sync.Once pattern
4. Integration with existing rate-limit metrics infrastructure

Brainstormed Solutions:

1. Create engine/auth/metrics.go following the pattern from ratelimit/metrics.go
2. Instrument auth middleware with latency measurement around validation logic
3. Reuse existing rate-limit metrics infrastructure
4. Follow global histogram bucket pattern for consistency
5. Use sync.Once for thread-safe metric initialization

Let me start by examining the existing metrics implementations to understand the patterns, then implement the auth metrics.
```
