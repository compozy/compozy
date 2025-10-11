# PRD-IMP Test Plan: Improving Agentic Orchestration

## Scope

This plan defines unit, integration, and observability tests for the agent orchestrator improvements described in `_techspec.md`. It maps test coverage to each parent task (1.0–6.0) and follows @.cursor/rules/test-standards.mdc.

## Test Matrix (by Task)

- 1.0 Enhance plan schema and compiler defaults
  - Unit: schema required fields, default injection (`type`, `status`), MaxSteps enforcement, inputs→with normalization, stable step ids
  - Integration: `cp__agent_orchestrate` rejects invalid plan, accepts valid compiled plan

- 2.0 Improve orchestrate prompting and retry with reflexion
  - Unit: planner compile path retries once on invalid JSON, appends corrective feedback, respects ForceJSON/structured mode when supported
  - Integration: invalid first response → valid second response yields compiled plan

- 3.0 Strengthen FSM and parallel execution
  - Unit: parallel aggregator (all-success, first-success), child timeout handling, cancellation propagation, run summaries
  - Integration: multi-agent parallel plan executes, logs include group summaries

- 4.0 Error handling and memory feedback loops
  - Unit: remediation hint mapping (known substrings), episodic failure record, nearest tool suggestions on not-found
  - Integration: orchestrate returns remediation hints in tool error payloads

- 5.0 Tool registry discovery and caching
  - Unit: TTL cache hit/miss, background refresh, allow/deny lists, adapter passthrough
  - Integration: not-found → suggestion via registry ListAll

- 6.0 Testing and metrics for orchestrator
  - Unit: counters/gauges registration and increments for invalid-plan, retries, parallel timing
  - Integration: metrics emitted on orchestrate flows (scrapable via monitoring test sink)

## Unit Test Suites

### Planner & Spec

- Package: `engine/tool/builtin/orchestrate/planner`
  - File: `compiler_test.go`
    - Should inject defaults for missing step fields (type, status)
    - Should error on invalid shapes with specific messages
    - Should cap steps at MaxSteps and return explicit error
    - Should convert inputs list to `with` map as required
    - Should retry once on invalid JSON with corrective prompt
  - File: `schema_strict_test.go`
    - Should require fields at all nesting levels (plan, steps, agent, parallel)
  - File: `normalize_test.go`
    - Should remove placeholder-only values, preserve semantics for nil/empty

### Orchestrate Executor & Handler

- Package: `engine/tool/builtin/orchestrate`
  - File: `executor_test.go`
    - Should execute parallel groups and aggregate per merge policy
    - Should enforce step timeouts and propagate cancellations
    - Should produce run summaries for groups and children
  - File: `handler_test.go`
    - Should map invalid plan to remediation hint and 4xx code
    - Should pass compiled plan to engine and return results on success

### Orchestrator (LLM loop, response builder)

- Package: `engine/llm/orchestrator`
  - File: `response_handler_test.go`
    - Should produce stable, length-bounded IDs for feedback keys
    - Should map common error strings to deterministic remediation hints
  - File: `tool_executor_test.go`
    - Should surface tool-not-found with remediation and suggestions
  - File: `request_builder_test.go`
    - Should query registry and construct tool inputs; suggest nearest matches on miss

### Tool Registry

- Package: `engine/llm`
  - File: `tool_registry_test.go`
    - Should cache find results within TTL, refresh after expiry
    - Should apply allow/deny lists when configured
    - Should list and filter tools correctly

### Monitoring

- Package: `engine/infra/monitoring`
  - File: `orchestrate_metrics_test.go`
    - Should register counters/gauges and increment on invalid-plan, retries, parallel duration

## Integration Tests (test/integration)

- `orchestrate_plan_validation_test.go`
  - Invalid plan → 400 with remediation; valid compiled plan → 200
- `orchestrate_parallel_execution_test.go`
  - Parallel steps run; aggregated result shape and logs validated
- `orchestrate_retry_reflexion_test.go`
  - First invalid LLM response → single corrective retry → valid plan executed
- `tool_registry_suggestions_test.go`
  - Unknown tool triggers suggestions from registry

## Non-Functional Tests

- Metrics emission during orchestrate flows (prom-compatible test sink)
- Log redaction and structure checks (no sensitive fields)

## References (Sources)

- ReAct: Yao et al., 2022 — `https://arxiv.org/abs/2210.03629`
- Reflexion: Shinn et al., 2023 — `https://arxiv.org/abs/2303.11366`
- Plan-and-Solve: Wang et al., 2023 — `https://arxiv.org/abs/2305.04091`
- Graph of Thoughts: Besta et al., 2023 — `https://arxiv.org/abs/2308.09687`
- Toolformer: Schick et al., 2023 — `https://arxiv.org/abs/2302.04761`
- LangChain multi-agent blog — `https://blog.langchain.dev/multi-agent-systems/`
- CrewAI docs — `https://docs.crewai.com/core-concepts/Agents/`

## Execution

```bash
make test
GOFLAGS=-run TestCompiler go test -v ./engine/tool/builtin/orchestrate/planner
GOFLAGS=-run TestCompilerCompile_ShouldUseStructuredPlan go test -v ./engine/tool/builtin/orchestrate/planner
```
