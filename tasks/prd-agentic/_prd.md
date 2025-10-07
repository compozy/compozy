# Product Requirements Document (PRD): Agentic Built‑in Tool (cp__agent_orchestrate)

## Overview

Enable Compozy users to orchestrate multiple agents and agent actions directly from a task prompt without pre‑wiring a `task.Config.Agent`. The feature introduces a native built‑in tool that interprets natural language or a structured plan and executes one or more agent calls, including optional parallel branches, returning consolidated outputs. This reduces YAML ceremony, unlocks higher‑order agentic workflows, and shortens time‑to‑value for complex automations.

Primary users: workflow authors and advanced operators who currently compose multi‑step agent flows through explicit task graphs. Secondary users: product teams prototyping new flows and tool‑calling agents needing ad‑hoc delegation.

## Goals

- Reduce configuration overhead for multi‑agent workflows by enabling prompt‑driven orchestration.
- Provide reliable, observable execution of sequential and parallel agent calls with clear outputs.
- Preserve safety via timeouts, recursion guards, and concurrency limits.
- Integrate with existing execution/state stores so sub‑executions are traceable.

Key success metrics (measure over first 60 days GA unless noted):
- Task success rate: ≥ 85% of orchestrated runs complete without manual retry (30 days post‑GA).
- p90 prompt‑to‑result latency for top 3 workflows: ≤ 5s with two agent steps (same project hardware/settings).
- Manual configuration reduction: ≥ 40% decrease in average YAML/task lines for equivalent multi‑agent flows.
- Adoption: ≥ 25% of active workflow authors invoke at least one orchestrated action weekly.
- Cost efficiency: ≤ $0.02 average variable compute cost per orchestrated task by end of Q3 (internal metering).
- Error‑loop containment: < 1% of sessions exceed three agent retries.

## User Stories

- As a workflow author, I can ask “Call `my_agent` to summarize input X, then call `another_agent.action_x` and `action_y` in parallel with the summary”, and receive merged results in one task output.
- As an agent developer, I can reference the built‑in tool from an agent with tool‑calling and delegate subtasks to other agents without adding new workflow tasks.
- As an operator, I can inspect exec IDs and logs for each sub‑execution to debug failures.

## Core Features

1. Prompt‑driven orchestration
   - Accept free‑form prompt or structured JSON plan describing sequential/parallel steps.
   - Resolve agents/actions by ID; pass `with` parameters (templated) per step.
2. Parallel execution
   - Execute multiple agent actions concurrently with configurable max concurrency and result merge strategies.
3. Safety & limits
   - Enforce recursion depth, max steps, max parallelism, and per‑step/overall timeouts.
4. Observability
   - Emit per‑step exec IDs; sub‑executions persisted in task repository; telemetry for successes/failures/latency.
5. Deterministic outputs
   - Return structured summary: step statuses, outputs, errors, and final bindings map.

Functional requirements (numbered):
- R1. The system MUST expose a built‑in tool ID `cp__agent_orchestrate` discoverable by the tool registry.
- R2. The tool MUST accept either a `prompt` or a structured `plan`; at least one is required.
- R3. The tool MUST support sequential and parallel blocks with per‑step `agent_id`, optional `action_id`, optional `prompt`, and `with` parameters.
- R4. The tool MUST execute sub‑steps using the same execution/state infrastructure as direct task/agent sync routes and persist state with unique exec IDs.
- R5. The tool MUST enforce configurable safety limits: `max_depth`, `max_steps`, `max_parallel`, and timeouts.
- R6. The tool MUST return a machine‑readable result containing each step’s status, `exec_id`, `output` (when present), and aggregated errors.
- R7. The tool MUST propagate cancellation and deadlines from the parent task context.
- R8. The tool MUST emit telemetry for total invocation and per‑step metrics (success/failure, latency, response sizes).
- R9. The tool SHOULD offer a planner mode that converts the natural‑language `prompt` into a valid structured plan.
- R10. The tool SHOULD cap fan‑out when `max_parallel` is unspecified using a project/runtime default.

## User Experience

- Triggered implicitly via agent tool‑calling or explicitly via task tool reference.
- Single surface: user writes a natural language directive; system compiles and executes a plan, returning a consolidated object.
- Error experience: partial results when possible, with step‑level errors and guidance; links to `/executions/agents/{exec_id}` for each step.
- Accessibility: plain text inputs/outputs with JSON schemas; no graphical configuration required.

## High‑Level Technical Constraints

- Must reuse existing synchronous execution path (DirectExecutor + task repository) for consistency and durability.
- Must honor runtime/context propagation (logger, config, request IDs) and avoid any `context.Background()` in runtime paths.
- Security/compliance: propagate existing redaction and logging policies; avoid leaking secrets through nested prompts.
- Performance targets: see Success Metrics; ensure fair scheduling via concurrency caps.
- Provider dependencies: model/provider variability can affect behavior; include guardrails for drift.

## Non‑Goals (Out of Scope)

- Third‑party marketplace or user‑defined custom agents (future consideration).
- Fully autonomous, unattended execution without human oversight/confirmation.
- Voice/multimodal input; MVP is text‑only.
- Offline/air‑gapped operation.
- Real‑time multi‑user collaboration within a single orchestration session.

## Phased Rollout Plan

- MVP: prompt → sequential plan; single parallel block; recursion depth = 2; metrics & exec IDs; safety caps.
- Phase 2: nested parallel blocks, richer merge strategies, improved planner quality.
- Phase 3: self‑healing/retry policies and advanced cost/latency budgeting.

## Success Metrics

- See “Goals”. Track: adoption, completion rate, p90 latency, cost per task, retries > 3, YAML reduction.

## Risks and Mitigations

- Model drift and volatility → pin model defaults, add regression suites; monitor deltas.
- Cascading failures/loops → recursion depth/timeouts; detect repetitive tool calls; abort with guidance.
- Cost/rate limit spikes → concurrency caps; backoff; per‑step budgeting; surface costs in telemetry.
- Data leakage across steps → template constraints; sensitive‑field redaction; review prompts for PII handling.
- UX overload → keep single surface; simplified outputs; link deeper details only when necessary.

## Open Questions

- Planner UX: configurable planner agent vs. fixed system prompt?
- Merge strategies beyond collect‑all (e.g., first‑success, reduce)?
- Per‑step retry policies and user‑visible partial streaming?

## Appendix

- Comparative UX references: single‑surface orchestration patterns (internal research). 
- Evaluation protocol: benchmark tasks, acceptance criteria for completion rate/latency.

