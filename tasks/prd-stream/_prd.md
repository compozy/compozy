# Product Requirements Document (PRD) — Executions Streaming (/stream)

## Overview

Introduce real-time streaming for execution updates in Compozy so developers and operators can observe workflow, agent, and task progress as it happens. The feature exposes dedicated streaming endpoints that deliver progressive events for long-running or multi-step executions, improving UX over periodic polling and enabling responsive UIs, dashboards, and automations.

## Goals

- Deliver live execution updates with low perceived latency (< 1s p50 from server emit to client render under normal load).
- Provide durable, resumable event streams for workflows; provide progressive text streaming for agents/tasks when no output schema is defined.
- Standardize event semantics and structure (event types, timestamps, IDs) for SDKs and UI.
- Ensure easy client consumption (browser EventSource, Node, curl) and seamless reconnection.
- Maintain compatibility with existing sync/async execution flows and status endpoints.

Key metrics

- Time-to-first-event (TTFE) after starting a stream.
- Event delivery success rate and reconnection success rate.
- Adoption: % of UI/SDK flows using streaming over polling; CLI usage of streaming docs.

## User Stories

- As an application developer, I want to subscribe to workflow progress so I can render step-by-step updates and completion without polling.
- As an operator, I want to tail agent/task output in near real time to diagnose issues faster.
- As a CLI/automation user, I want a straightforward way to follow an execution stream and act on completion.
- As a platform integrator, I want a consistent event model across workflows, agents, and tasks to minimize custom parsing.

## Core Features

1. Streaming endpoints

- New GET endpoints under the executions namespace for workflows, agents, and tasks.
- Streams include well-defined event types (e.g., workflow_status, tool_call, llm_chunk, complete, error).

2. Resumption & heartbeats

- Support reconnection with Last-Event-ID for workflow streams to resume without data loss.
- Periodic heartbeats ensure long-lived connections stay healthy and are load‑balancer friendly.

3. Text vs JSON streaming

- Workflows always stream structured JSON events.
- Agents/Tasks stream structured JSON when output schemas exist; otherwise stream human-readable text chunks for interactive experiences.

4. Developer ergonomics

- Documented examples for browser, Node, and curl.
- Event filtering and basic configuration (poll interval where applicable).

Functional requirements (high-level)

1. Provide three public streaming endpoints for workflows, agents, and tasks.
2. Define and publish an event catalog with stable fields (id, type, ts, data).
3. Offer reconnection semantics and heartbeats so clients can run long sessions reliably.
4. Respect existing authz; streams must only expose data a client is authorized to see.
5. Provide docs, examples, and tests to ensure adoption and quality.

## User Experience

- Personas: backend developers, operators/SREs, SDK consumers, dashboard users.
- Flows:
  - Start execution → open stream → render updates progressively → close on complete/error.
  - For agent/task without schemas, render text like a console log view as chunks arrive.
- UX considerations:
  - Show “Connecting…” state and last received event ID on reconnects.
  - Provide copyable examples; clarify that text-only streams do not backfill missed chunks across reconnects.
- Accessibility: streamed text should be readable with basic screen readers; avoid color-only semantics.

## High-Level Technical Constraints

- Transport: Server‑Sent Events (SSE) over HTTP; browser-native EventSource support required.
- Durability: Workflow streams must be resumable via a durable source of truth; agent/task text streaming may be best‑effort.
- Security/Privacy: Do not emit secrets; follow existing redaction and auth policies.
- Performance targets: Maintain low overhead per connection; bounded connection lifetimes; observable backpressure behavior.
- Compatibility: No breaking changes to existing APIs; feature is purely additive.

## Non-Goals (Out of Scope)

- Implementing WebSockets transport.
- Building a complex front-end console beyond examples and basic docs.
- Guaranteed backfill of missed text-only chunks across reconnects (text mode is best‑effort).
- Streaming of binary payloads.

## Phased Rollout Plan

- MVP (Public Preview)
  - Workflow streaming endpoint with durable JSON events, reconnection, and heartbeats.
  - Docs, examples, event catalog, and SDK guidance.

- Phase 2
  - Agent/Task streaming endpoints.
  - Text-only progressive output for executions without output schemas; structured JSON when schemas exist.
  - CLI/SDK usage notes and basic filters.

- Phase 3
  - Operational hardening: limits, metrics, dashboards, and scaling guidance.
  - Optional convenience features (client filters, default UI widgets).

Success per phase

- MVP: stable workflow streams in UI/SDK; reconnection verified; documentation complete.
- Phase 2: agent/task streams adopted by core use cases; examples runnable locally.
- Phase 3: production SLOs met; metrics and dashboards in place; adoption milestones hit.

## Success Metrics

- Reliability: ≥ 99.9% stream availability during business hours (excluding planned maintenance).
- UX: p50 time-to-first-event < 1s; p95 < 2s for typical executions.
- Adoption: ≥ 50% of eligible UI/SDK views prefer streaming over polling within one release.
- Supportability: Mean time to diagnose long-running executions decreases vs baseline.

## Risks and Mitigations

- Network/proxy behavior with SSE (risk: broken buffering or idle timeouts) → clear headers, heartbeats, guidance for reverse proxies.
- Client reconnection edge cases (risk: duplicate/missed events) → document Last-Event-ID and event idempotency guidance in examples.
- Server resource usage for many concurrent streams → connection limits, observability, and documented scaling patterns.
- Adoption risk if docs/examples are insufficient → prioritize simple, copy-paste clients and CLI guidance.

## Open Questions

- Do we want CLI commands to consume `/stream` directly in this milestone, or keep CLI follow behavior as polling-only for now?
- Any organization-specific CORS or auth header requirements to document for SSE usage across domains?
- Default connection lifetime and polling interval defaults to expose as configuration?

## Appendix

- Background: user requests for live progress and better visibility into long-running workflows.
- Prior art: common SSE patterns in API streaming and LLM token display; EventSource ergonomics.

## Planning Artifacts (Post-Approval)

- Docs Plan: `tasks/prd-stream/_docs.md` (template: `tasks/docs/_docs-plan-template.md`)
- Examples Plan: `tasks/prd-stream/_examples.md` (template: `tasks/docs/_examples-plan-template.md`)
- Tests Plan: `tasks/prd-stream/_tests.md` (template: `tasks/docs/_tests-plan-template.md`)
