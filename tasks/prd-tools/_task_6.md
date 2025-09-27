---
status: pending
parallelizable: true
blocked_by: ["2.0", "3.0"]
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>integration</type>
<scope>performance</scope>
<complexity>medium</complexity>
<dependencies>monitoring</dependencies>
<unblocks>["7.0"]</unblocks>
</task_context>

# Task 6.0: Instrument observability and canonical error catalog

## Overview

Add structured logging and metrics for cp\_\_ tools, wire canonical error codes, and ensure telemetry integrates with existing monitoring pipelines. Provide dashboards or documentation for on-call teams.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Emit structured logs per invocation containing `tool_id`, `request_id`, `exit_code`, latency, and `stderr_truncated` flag (where applicable).
- Register Prometheus counters/histograms for success/error counts, latency, response size, and failure categories.
- Map builtin tool errors to canonical codes defined in Task 1.0 and propagate through orchestrator responses.
- Provide alerting recommendations for configuration overrides or repeated security violations (e.g., sandbox rejections).
- Update dashboards or runbooks to include cp__ metrics and logging fields.
</requirements>

## Subtasks

- [ ] 6.1 Extend logging middleware or helper to include new structured fields for cp\_\_ tools.
- [ ] 6.2 Define Prometheus metrics and ensure registration occurs during service startup.
- [ ] 6.3 Integrate canonical error mapping into tool outputs and orchestrator error handling.
- [ ] 6.4 Update monitoring documentation/runbooks with new metrics and alert guidance.

## Sequencing

- Blocked by: 2.0, 3.0
- Unblocks: 7.0
- Parallelizable: Yes (after dependent tools exist)

## Implementation Details

Refer to tech spec "Monitoring & Observability" section. Ensure logging uses `logger.FromContext`. Metrics should reuse existing registry/namespace conventions to avoid duplication.

### Relevant Files

- `engine/tool/builtin/telemetry.go`
- `engine/infra/monitoring/metrics.go`
- `engine/llm/orchestrator/*`

### Dependent Files

- `engine/tool/builtin/registry.go`
- `engine/tool/builtin/filesystem/*`

## Success Criteria

- Metrics visible in Prometheus scrape with correct labels.
- Logs contain required fields and pass linting/static analysis.
- On-call documentation updated to include cp\_\_ telemetry.
