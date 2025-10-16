# Product Requirements Document (PRD) Template

## Overview

Compozy currently exposes execution outcomes for workflows, tasks, and agents but offers no insight into the LLM tokens consumed per run. This gap limits customers’ ability to audit usage or reconcile downstream costs and prevents internal teams from monitoring usage anomalies. The “LLM Usage Reporting” initiative will persist execution-level token metrics (prompt, completion, derived totals) for workflows, direct task runs, and agent executions, expose the data via existing execution endpoints, and surface operational telemetry so developers and operators can make informed decisions. This PRD focuses on backend changes only and will serve as a living document maintained alongside linked ADRs and change logs.

## Goals

- Capture token usage for ≥95% of executions (workflow, task, agent) within five minutes of completion.
- Expose usage summaries in `/executions/*` responses and CLI/SDK access paths with a documented JSON schema.
- Provide operational signals so the observability stack can alert on missing or anomalous usage ingests (<1% consecutive drop tolerance).
- Maintain compliance with existing architecture, configuration, and logging rules while avoiding measurable latency regressions (>5% p95) on execution APIs.

## User Stories

- As a platform developer integrating Compozy, I want consistent token usage data in execution APIs so I can reconcile LLM consumption with my billing system.
- As an internal operations analyst, I want to audit token usage across workflows, tasks, and agents so I can detect misuse or unexpected spikes.
- As a support engineer, I want failed executions to include whatever usage data exists so I can debug incidents without re-running workflows.
- As an engineering lead, I want dashboards/metrics about usage ingestion health so I can trigger remediation playbooks when provider data drifts.

## Core Features

1. **Usage Capture & Normalization**
   - Description: Extract token usage metadata from LLM responses (including reasoning/cached tokens where available) during workflow, task, and agent executions.
   - Importance: Ensures a single source of truth for execution-level token consumption.
   - Functional Requirements:
     - R1. System must populate `LLMResponse.Usage` with prompt, completion, total, and reasoning tokens when providers supply `GenerationInfo` or equivalent metadata.
     - R2. System must gracefully default usage fields to `null` and log a structured warning when providers omit usage data.

2. **Usage Persistence & Retrieval**
   - Description: Persist execution-level usage in PostgreSQL with relational links to `workflow_states` and `task_states` records.
   - Importance: Enables durable audit history and future analytics.
   - Functional Requirements:
     - R3. System must insert or upsert per-execution usage rows in `execution_llm_usage`, keyed by execution ID and component type (`workflow`, `task`, `agent`).
     - R4. System must enforce foreign-key integrity with workflow/task execution tables and reject orphan writes.

3. **API Exposure & Client Compatibility**
   - Description: Enrich existing `/executions/*` responses (agent, task, workflow) and CLI outputs with usage summaries.
   - Importance: Provides immediate value to developers without introducing new endpoints.
   - Functional Requirements:
     - R5. API responses must include a `usage` object containing `model`, `provider`, `prompt_tokens`, `completion_tokens`, `total_tokens`, and optional auxiliary fields (e.g., `reasoning_tokens`).
     - R6. Response payloads must retain backward-compatible behavior by defaulting to `null` usage values when data is unavailable, preserving existing clients.

4. **Observability & Alerting**
   - Description: Emit metrics recording ingest success, token totals, and error rates while integrating with existing monitoring rules.
   - Importance: Detects ingestion regressions and provider anomalies promptly.
   - Functional Requirements:
     - R7. System must publish Prometheus counters (`compozy_llm_prompt_tokens_total`, `compozy_llm_completion_tokens_total`, `compozy_llm_usage_events_total`) distinguished by component, model, and provider.
     - R8. System must log structured errors and emit alerts when ingestion fails for >1% of executions over a 15-minute window.

5. **Options & Trade-offs Considered**
   - Per-execution aggregate vs. per-message capture: Chose per-execution to limit storage growth and complexity; future phases may add granular logging.
   - Dedicated usage service vs. inlined logic: Selected modular `llmusage` package coupled to orchestrator hooks to balance cohesion and velocity.
   - Immediate backfill vs. forward-only ingest: Opted for forward-only to avoid launch delays; backfill remains a potential Phase 2 enhancement.

## User Experience

- Personas: Platform developers, internal operations/finance, support engineers.
- Interactions: JSON API responses, CLI responses, future Grafana dashboards (no new UI surfaces).
- Accessibility: Maintain consistent JSON schema well documented in developer docs; ensure CLI outputs remain parsable without additional formatting. No localization requirements.
- Collaboration: PRD remains versioned (change log maintained in repo or linked workspace) and will reference ADRs to trace architectural decisions.

## High-Level Technical Constraints

- Integrations: Must hook into existing orchestrator (`engine/llm/orchestrator`) and execution routers without violating `.cursor/rules/architecture.mdc` (context-first patterns, no global singletons), `.cursor/rules/logger-config.mdc` (use `logger.FromContext`), and `.cursor/rules/global-config.mdc` (use `config.FromContext`).
- Data: Token usage contains no PII but should follow retention policies once defined (default 180 days).
- Performance: Execution API latency may increase by ≤5% p95; ingestion must not block critical paths (asynchronous or buffered writes preferred).
- Compliance/Security: Ensure database migrations align with security review process; maintain auditability through structured logging.
- Documentation: Link resulting ADRs and metrics dashboards in Appendix for traceability.

## Non-Goals (Out of Scope)

- Cost estimation or billing automation.
- UI dashboards or customer-facing reporting portals.
- Per-message or per-token streaming analytics.
- Historical backfill for legacy executions at launch.

## Phased Rollout Plan

- **MVP:** Capture token usage for workflows, tasks, and agents; expose data via APIs/CLI; emit baseline metrics. Success: ≥95% executions logging usage, metrics available in staging dashboards.
- **Phase 2:** Introduce aggregation exports (e.g., daily summaries), optional historical backfill tools, and enhanced alerting playbooks. Success: ability to export aggregated usage, retention policy formalized.
- **Phase 3:** Advanced analytics (trend dashboards, anomaly detection hooks) and cost-estimation integrations. Success: integration-ready datasets and automated anomaly classifications.

## Success Metrics

- Coverage: ≥95% of eligible executions include usage data within 5 minutes of completion.
- Accuracy: ≤5% variance between recorded totals and provider-reported totals in weekly audits.
- Reliability: <1% ingestion failures per rolling 15 minutes with automated alerting.
- Performance: ≤5% increase in p95 execution API latency after rollout.
- Operational Adoption: Observability dashboards actively monitored by ops team (measured via weekly review check-ins).

## Risks and Mitigations

- Provider Usage Inconsistency — _Mitigation:_ implement reconciliation jobs and alert on variance >5%.
- Storage Growth / Retention — _Mitigation:_ define retention/archival policy before GA; monitor table size.
- Migration Complexity — _Mitigation:_ test migrations in staging with representative data; provide rollback scripts.
- Latency Regression — _Mitigation:_ batch writes or perform asynchronous persistence; load test before release.
- Missing User Validation — _Mitigation:_ schedule developer interviews and gather telemetry feedback before GA; treat PRD as living doc to incorporate findings.

## Open Questions

- What is the formal retention period for usage data (default proposal: 180 days)?
- Should we support historical backfill tooling in Phase 2 or accept forward-only data?
- Which ADR IDs will capture architectural decisions for the `llmusage` package and schema changes?
- Who owns long-term maintenance of the usage metrics dashboard and alert tuning?

## Appendix

- References: `USAGE_FEATURE.md`, provider documentation for token usage (OpenAI, LangChainGo capabilities).
- Dependency & RACI Snapshot:
  - Orchestrator & collector changes — Engineering (LLM team).
  - Database migrations — Backend team.
  - Observability metrics — Infrastructure/Monitoring team.
  - Change-log stewardship — Product owner.
- Change Log: Maintain via repository history (`tasks/prd-usage/CHANGELOG.md`) or linked workspace; document all post-approval modifications using lightweight change-request notes.
- Living Document Guidance: PRD updates must reference corresponding ADRs and include summary of user validation outcomes after each discovery cycle.

## Planning Artifacts (Post-Approval)

[Created by the Tech Spec workflow and maintained alongside this PRD.]

- Docs Plan: `tasks/prd-[feature-slug]/_docs.md` (template: `tasks/docs/_docs-plan-template.md`)
- Examples Plan: `tasks/prd-[feature-slug]/_examples.md` (template: `tasks/docs/_examples-plan-template.md`)
- Tests Plan: `tasks/prd-[feature-slug]/_tests.md` (template: `tasks/docs/_tests-plan-template.md`)
