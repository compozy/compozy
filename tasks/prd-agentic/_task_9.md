---
status: completed
parallelizable: true
blocked_by: ["5.0", "6.0", "8.0"]
---

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>implementation</type>
<scope>performance</scope>
<complexity>medium</complexity>
<dependencies></dependencies>
<unblocks>"10.0"</unblocks>
</task_context>

# Task 9.0: Telemetry/metrics and logging for steps and totals

## Overview

Instrument builtin invocations and per‑step execution with existing telemetry framework. Ensure logs include agent_id, action_id, exec_id.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Use `builtin.RecordInvocation` and OpenTelemetry histograms.
- Add structured logs; redact sensitive data.
</requirements>

## Subtasks

- [x] 9.1 Metrics wiring
- [x] 9.2 Structured logging

## Sequencing

- Blocked by: 5.0, 6.0, 8.0
- Unblocks: 10.0
- Parallelizable: Yes

## Success Criteria

- Metrics appear in tests; logs contain exec metadata
