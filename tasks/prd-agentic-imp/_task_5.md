## markdown

## status: pending

<task_context>
<domain>engine/llm/telemetry</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 5.0: Telemetry & Config Tunables Expansion

## Overview

Broaden observability and configurability by capturing richer agent metrics, exposing tunable budgets/timeouts, and integrating defaults across CLI, config, and telemetry sinks.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Extend telemetry middleware to report provider latency, token spend, restart counts, and cooldown activations.
- Expose retry, timeout, and threshold tunables via `pkg/config`, wiring them into orchestrator and provider registry.
- Update CLI/env bindings to surface new tunables with documentation and validation.
- Ensure telemetry redacts sensitive values per logging standards.
</requirements>

## Subtasks

- [ ] 5.1 Introduce telemetry decorators for providers and loop events capturing new metrics.
- [ ] 5.2 Add configuration keys (with defaults) for retries, budgets, restart thresholds, and cooldown penalties.
- [ ] 5.3 Wire config values through orchestrator settings and ensure context-based access (`config.FromContext`).
- [ ] 5.4 Update CLI help/docs and config diagnostics to display new tunables.
- [ ] 5.5 Add tests covering config precedence, telemetry emission, and redaction policies.

## Implementation Details

- PRD “Research-Aligned Improvements” Phase 1 & 4 stress telemetry visibility and config tunables for budgets/timeouts.
- Use existing telemetry package patterns to avoid duplicating logger logic.
- Ensure defaults remain backward-compatible; migrations should not break existing deployments.

### Relevant Files

- `engine/llm/telemetry/*`
- `engine/llm/orchestrator/llm_invoker.go`
- `pkg/config/config.go`
- `pkg/config/provider.go`
- `cli/helpers/global.go`

### Dependent Files

- `engine/llm/service.go`
- `engine/llm/orchestrator/settings.go`

## Deliverables

- Expanded telemetry events and metrics tied to provider and loop instrumentation.
- New configuration knobs with CLI/env integration and documentation.
- Updated diagnostics output confirming tunable sources and effective values.

## Tests

- Tests mapped from PRD test strategy:
  - [ ] Telemetry emits restart and cooldown metrics during orchestrator runs.
  - [ ] Config precedence: CLI/env overrides defaults for retries and thresholds.
  - [ ] Redaction tests ensure sensitive provider data is not logged.

## Success Criteria

- Operators can tune budgets and observe effects without code changes.
- Telemetry dashboards (or logs) expose restart, fallback, and cooldown metrics.
- All configuration and telemetry tests pass with existing suites.
- `make fmt && make lint && make test` pass.
