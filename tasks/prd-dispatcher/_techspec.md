# Dispatcher Shutdown Remediation Plan

## Executive Summary

- Abrupt Compozy server exits leave multiple `dispatcher-*` Temporal workflows running, so new workers reuse stale dispatchers instead of claiming ownership.
- Dispatcher identity is tied to a process-local UUID. Temporal never sees a deterministic winner, and our shutdown logic only terminates dispatchers during graceful exits.
- We need deterministic ownership, automated cleanup of stragglers, richer observability, and a safe rollout that respects runtime standards (context propagation, `logger.FromContext`, `config.FromContext`).

## Context & Symptoms

- Killing a server (`SIGKILL`, crashes, container eviction) leaves dispatcher workflows alive; subsequent boots log "dispatcher already running" while reusing a stale worker (`engine/worker/mod.go:624-856`).
- Shutdown only reaches `Worker.Stop` from `handleGracefulShutdown`, so emergency exits omit `TerminateWorkflow` + heartbeat cleanup (`engine/infra/server/lifecycle.go:77-107`).
- Redis dispatcher heartbeat keys accumulate because removal is tied to graceful cleanup in `RemoveDispatcherHeartbeat`.
- Operations reported an ever-growing set of dispatcher IDs in Temporal and Redis after rolling restarts.

## Current Lifecycle Risks

- **Identity drift:** Dispatcher IDs change every boot (`dispatcher-<queue>-<server-id>`). Temporal sees different workflow IDs per process, creating zombies.
- **Best-effort cleanup:** Crash paths skip `TerminateWorkflow` and heartbeat TTL cleanup, so stale dispatchers continue routing signals.
- **No takeover:** `SignalWithStartWorkflow` handles `WorkflowExecutionAlreadyStarted` as success, so a new worker never evicts the stale execution and silently reuses it.
- **Observability blind spot:** We lack alerts on dispatcher count drift, stale heartbeat age, and takeover churn; operations find problems manually.

## Goals & Non-goals

**Goals**

- Enforce exactly one healthy dispatcher per project/task queue with automatic takeover on restart.
- Remove orphaned workflows and heartbeat keys within SLA after ungraceful exits.
- Expose metrics/alerts for dispatcher cardinality, stale cleanup backlog, and takeover attempts.
- Provide a rollout playbook that avoids signal loss or duplicate routing.

**Non-goals**

- No changes to business-level task routing semantics beyond dispatcher lifecycle.
- No Temporal namespace topology changes (assume existing namespaces remain).
- No redesign of Redis heartbeat schema beyond keys associated with dispatchers.

## Approach Overview

Implementation proceeds in three focused phases, each tied to concrete acceptance criteria and verified against the Dispatcher Shutdown PRD and tech spec before code freeze.

### Phase 1 - Deterministic Dispatcher Ownership (Week 1)

**Implementation**

- Derive dispatcher workflow IDs from stable inputs `dispatcher-<project>-<task-queue>` and include `serverID` in workflow input metadata instead of the ID.
- Call `SignalWithStartWorkflow` with `WorkflowIdReusePolicyTerminateIfRunning` so a new boot terminates lingering executions before continuing.[1]
- When `WorkflowExecutionAlreadyStarted` is returned (legacy deployments), issue an explicit `TerminateWorkflow` then retry startup to guarantee takeover.
- Persist takeover telemetry (counter + latency histogram) via existing monitoring interceptors.

**Acceptance criteria**

- Restarting a worker replaces any stale dispatcher with the new execution within one attempt.
- Temporal history shows `TerminateWorkflow` preceding new execution during takeover in integration environment.

### Phase 2 - Stale Dispatcher Reconciliation (Weeks 2-3)

**Implementation**

- Build a dispatcher supervisor component (`engine/worker/supervisor.go`) that runs on worker startup and every minute thereafter.
- Supervisor lists heartbeat keys via existing Redis helpers, classifies them as healthy vs stale using TTL metadata, and terminates associated Temporal workflows before deleting keys.
- Supervisor uses inherited request context and obtains logger/config from context to respect runtime standards.
- Emit metrics: `dispatcher.active`, `dispatcher.stale`, `dispatcher.terminations`, `dispatcher.cleanup_errors`.

**Acceptance criteria**

- Killing a worker results in supervisor cleanup of heartbeat + Temporal workflow within SLA (default 2m) in staging.
- Grafana dashboard surfaces dispatcher counts and stale cleanup backlog; alert fires when stale count exceeds threshold for >5m.

### Phase 3 - Verification & Operational Hardening (Weeks 3-4)

**Implementation**

- Extend dispatcher workflow to register incoming workers (server ID + task queue) so the workflow knows which hosts are active.
- Keep a single dispatcher heartbeat but add a periodic self-check that evicts workers whose heartbeat timestamps exceed the stale threshold.
- Ensure all registration and heartbeat activities inherit context from callers (no `context.Background()` usage) and read configuration via `config.FromContext`.
- Emit minimal metrics needed for alerts: takeover count, stale dispatcher cleanup count, worker eviction count.

**Acceptance criteria**

- Load test with rolling restarts maintains a single dispatcher while correctly evicting dead worker heartbeats.
- Ops runbook updated with registration and eviction semantics.

### Validation & Rollout (Weeks 4-5)

**Testing**

- Unit tests: ID builder, takeover branch (success, AlreadyStarted->Terminate->retry), supervisor stale classification, heartbeat eviction state machine.
- Integration tests: crash simulation (kill worker process) verifies cleanup within SLA; Temporal outage simulation ensures retry with backoff.
- Load tests: bursts of dispatcher signals during repeated rolling restarts to confirm no duplicate dispatchers or missed signals.
- Compliance: run `make lint`, `make test`, targeted `go test ./engine/worker -run Dispatcher` in CI, plus chaos scenarios in staging.

**Rollout plan**

- Deploy Phase 1 changes to staging, confirm takeover telemetry and absence of duplicate dispatcher IDs.
- Roll Phase 2 cleanup into staging, validate Redis cleanup latency, then promote both phases to production.
- Enable Phase 3 heartbeat checks once production metrics are stable; verify dispatcher and worker counts match expected values before closing rollout.

## Operational Readiness

- **Runbooks:** Update dispatcher ops guide with takeover flow, supervisor manual trigger command, and heartbeat eviction troubleshooting.
- **Dashboards:** Add panels for takeover latency, stale dispatcher count, heartbeat eviction rate, and TerminateWorkflow error rate.
- **Alerting:** Configure alerts for duplicate dispatcher detections, stale heartbeat backlog, supervisor failure rate, and takeover retries > N.

## Risks & Mitigations

- **Aggressive termination kills healthy dispatcher:** Limit TerminateExisting scope to the exact project/task queue and log every termination for audit.
- **Supervisor overload on Redis/Temporal:** Rate limit cleanup loops and stagger schedules across workers.
- **Heartbeat noise:** Batch eviction events and set conservative thresholds to avoid flapping.
- **Spec drift:** Review PRD/tech spec before implementation for updates; block merges until product sign-off is captured in tracking ticket.

## Open Questions

- Confirm SLA for dispatcher cleanup in PRD; current proposal assumes <=2 minutes.
- Do we need to broadcast dispatcher takeover events to other subsystems (e.g., task executors)?
- Should per-worker heartbeat eviction feed into incident response tooling beyond metrics (e.g., PagerDuty)?

## References

1. Temporal documentation - _Start Workflow with ID Reuse Policy TerminateIfRunning_. https://docs.temporal.io/cli/workflow/#start
2. Temporal TypeScript SDK documentation - _WorkflowHandle.signalWithStart_. https://docs.temporal.io/develop/typescript/apis/#workflowhandle-signalwithstart
