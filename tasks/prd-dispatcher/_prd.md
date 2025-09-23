# Dispatcher Shutdown Remediation PRD

## Overview

Abrupt Compozy server exits can leave multiple `dispatcher-*` Temporal workflows running. New workers may silently reuse stale dispatchers instead of claiming ownership, leading to nondeterministic routing and operational toil. This PRD defines the product outcomes, user goals, and success criteria for ensuring there is exactly one healthy dispatcher per project/task queue, with automatic takeover after restarts, timely cleanup of orphans, and first-class observability for operations.

Primary users are SRE/Operations, Platform Engineers, and On-call responders who need predictable dispatcher ownership, automated remediation after failures, and actionable visibility.

## Goals

- Ensure exactly one healthy dispatcher per project/task queue at all times.
- Enable deterministic takeover on restart without manual intervention.
- Remove orphaned dispatcher workflows and Redis heartbeat keys within SLA after ungraceful exits.
- Provide clear telemetry, dashboards, and alerts for dispatcher health and cleanup.
- Roll out changes safely without impacting business-level routing semantics.

Key metrics to track:

- p95 dispatcher takeover latency < 30s during worker restarts.
- Stale dispatcher cleanup SLA ≤ 2 minutes (95th percentile).
- Duplicate dispatcher count = 0 during rolling restarts in staging and production.
- Alert MTTA (mean time to acknowledge) for dispatcher health alerts < 5 minutes.

## User Stories

- As an SRE, I want only one active dispatcher per project/task queue so that routing is deterministic and safe during restarts.
- As a Platform Engineer, I want automatic takeover of stale dispatchers so that deployments and crash recovery do not require manual cleanup.
- As an On-call responder, I want dashboards and alerts that show stale dispatchers and cleanup activity so I can quickly triage issues.
- As an Operator, I want a clear rollout playbook and runbooks so I can verify behavior and remediate exceptions consistently.

## Core Features

### 1) Deterministic Dispatcher Ownership

High-level: Dispatcher workflow IDs derive from stable inputs (`dispatcher-<project>-<task-queue>`). Startup uses a termination-first policy to evict stale executions and proceed, guaranteeing a single active dispatcher.

Functional requirements:

1. R1: The system must derive dispatcher workflow IDs from stable inputs `dispatcher-<project>-<task-queue>`.
2. R2: The system must start or take over the dispatcher using a termination-first policy that guarantees stale executions are ended before proceeding.
3. R3: The system must record takeover telemetry (count and latency histograms) for observability.
4. R4: On encountering legacy AlreadyStarted scenarios, the system must terminate the existing execution and retry startup until a single dispatcher is active.

### 2) Stale Dispatcher Reconciliation (Supervisor)

High-level: A supervisor periodically inspects Redis heartbeat keys, classifies healthy vs. stale using TTL metadata, terminates associated Temporal workflows for stale entries, and deletes the keys. Runs at startup and every minute.

Functional requirements:

5. R5: The system must run a supervisor on worker startup and at a fixed interval (default 1 minute) to reconcile stale dispatchers.
6. R6: The supervisor must classify heartbeat entries as healthy vs. stale using TTL metadata and configurable thresholds.
7. R7: For stale entries, the supervisor must terminate the corresponding Temporal workflow before deleting the heartbeat key.
8. R8: The supervisor must emit metrics for `dispatcher.active`, `dispatcher.stale`, `dispatcher.terminations`, and `dispatcher.cleanup_errors`.

### 3) Verification & Operational Hardening

High-level: Extend dispatcher workflow to register incoming workers and periodically evict dead worker heartbeats. Provide minimal but sufficient metrics and a hardened rollout with runbooks/dashboards.

Functional requirements:

9. R9: The dispatcher must track registered workers (server ID + task queue) to understand active hosts.
10. R10: The dispatcher must periodically self-check and evict workers whose heartbeat timestamps exceed the stale threshold.
11. R11: All registration and heartbeat activities must inherit context from callers and read configuration via the standard runtime patterns.
12. R12: The system must expose minimal metrics required for alerting: takeover count, stale dispatcher cleanup count, and worker eviction count.

## User Experience

- Personas: SRE/Operations, Platform Engineers, On-call responders.
- Key flows: restart a worker and observe deterministic takeover; simulate a crash and observe stale cleanup within SLA; monitor dashboards and alerts; follow runbook for exceptions.
- UI/UX: Grafana dashboards for dispatcher counts, takeover latency, stale cleanup backlog, eviction rate, and TerminateWorkflow error rate.
- Accessibility: Dashboards should be legible with clear labeling, color contrast, and alt text for key panels used in documentation.

## High-Level Technical Constraints

This PRD captures product-level constraints; detailed design is in the Tech Spec.

- Integrations: Temporal (workflows/termination), Redis (heartbeat keys), Grafana/Prometheus (metrics/alerts).
- Runtime standards: Context is inherited through all call paths; `logger.FromContext(ctx)` is used for logging; `config.FromContext(ctx)` is used for configuration; no global singletons.
- Performance targets: p95 takeover < 30s; stale cleanup ≤ 2 minutes (95th percentile) under rolling restarts.
- Security/compliance: No changes to data sensitivity; ensure metrics/logs do not leak secrets.
- Non-negotiable: No introduction of background contexts in runtime paths.

## Non-Goals (Out of Scope)

- No changes to business-level task routing semantics beyond dispatcher lifecycle.
- No Temporal namespace topology changes.
- No Redis heartbeat schema redesign beyond keys associated with dispatchers.
- No UI productization beyond dashboards/alerts required for operations.

## Phased Rollout Plan

- MVP (Phase 1): Deterministic ownership
  - Outcomes: Stable dispatcher IDs; termination-first takeover; takeover telemetry visible in dashboards.
  - Acceptance: Restart replaces stale dispatcher within one attempt; Temporal history shows termination preceding new execution.

- Phase 2: Stale reconciliation
  - Outcomes: Supervisor classifies and cleans stale dispatchers; metrics for active/stale/terminations/cleanup errors.
  - Acceptance: Crash simulation results in cleanup of workflow + heartbeat within ≤ 2 minutes (95th) in staging; alert fires when stale count exceeds threshold for >5m.

- Phase 3: Verification & hardening
  - Outcomes: Worker registration; periodic self-check eviction; minimal alerting metrics.
  - Acceptance: Load tests with rolling restarts maintain a single dispatcher while evicting dead worker heartbeats; ops runbook updated and validated.

## Success Metrics

- p95 takeover latency < 30s during worker restarts.
- Stale cleanup SLA ≤ 2 minutes (95th percentile) after ungraceful exits.
- Duplicate dispatcher count = 0 during rolling restarts.
- Alert MTTA < 5 minutes for dispatcher health alerts.
- Supervisor error rate within acceptable thresholds (< 1% of cycles erroring over 1h).

## Risks and Mitigations

- Aggressive termination could kill a healthy dispatcher; mitigate by limiting termination scope to exact project/task queue and logging every termination.
- Supervisor could overload Redis/Temporal; mitigate via rate limiting and staggered schedules across workers.
- Heartbeat noise could cause flapping; mitigate via conservative thresholds and batched eviction events.
- Spec drift between PRD and Tech Spec; mitigate with pre-merge reviews and sign-off captured in tracking tickets.

## Open Questions

- Confirm SLA for dispatcher cleanup (current proposal assumes ≤ 2 minutes).
- Determine whether to broadcast dispatcher takeover events to other subsystems (e.g., task executors).
- Decide if per-worker heartbeat eviction should integrate with incident tooling beyond metrics (e.g., PagerDuty).

## Appendix

- References
  - Temporal: Start Workflow with ID Reuse Policy TerminateIfRunning.
  - Temporal TypeScript SDK: `WorkflowHandle.signalWithStart`.
  - Internal runbooks and dashboards to be updated in the rollout phase.
