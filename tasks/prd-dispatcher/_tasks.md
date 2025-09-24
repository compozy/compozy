# Dispatcher Shutdown Remediation – Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/worker/mod.go` – Worker boot/shutdown and dispatcher startup integration
- `engine/worker/dispatcher.go` – Dispatcher workflow startup/ownership logic (existing or to be created)
- `engine/worker/supervisor.go` – Stale dispatcher reconciliation supervisor (to be created)
- `engine/infra/server/lifecycle.go` – Graceful shutdown hooks (reference for parity)
- `engine/infra/redis/*` – Heartbeat helpers and key management
- `engine/infra/monitoring/*` – Metrics emission and interceptors

### Integration Points

- Temporal workflow client initialization and `SignalWithStart` usage
- Redis heartbeat TTL classification and deletion
- Grafana dashboards and Prometheus metrics/alerts

### Documentation Files

- `tasks/prd-dispatcher/_prd.md` – Product Requirements Document
- `tasks/prd-dispatcher/_techspec.md` – Technical Specification
- Ops Runbooks – Dispatcher takeover, supervisor manual trigger, heartbeat eviction

## Tasks

- [ ] 1.0 Deterministic Dispatcher Ownership and Takeover
- [ ] 2.0 Supervisor for Stale Dispatcher Reconciliation
- [ ] 3.0 Verification & Operational Hardening (Worker Registration & Eviction)
- [ ] 4.0 Metrics & Telemetry (Takeover, Cleanup, Evictions) **[NOT NEEDED]**
- [ ] 5.0 Dashboards & Alerts (Grafana/Prometheus) **[NOT NEEDED]**
- [ ] 6.0 Test Suite: Unit, Integration, Load & Chaos **[NOT NEEDED]**
- [ ] 7.0 Rollout Plan, Runbooks, and Operationalization **[NOT NEEDED]**

## Execution Plan

- Critical Path: 1.0 → 2.0 → 3.0 → 6.0 → 7.0
- Parallel Track A: 4.0 → 5.0 (starts after 1.0 establishes metrics hooks)
- Parallel Track B: Documentation for ops (part of 7.0, can begin after 2.0)
