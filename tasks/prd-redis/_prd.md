# Product Requirements Document — Replace PostgreSQL With Redis

## Overview

Compozy will replace all PostgreSQL-backed persistence with Redis while preserving external behavior. This effort is a greenfield replacement: we will remove Postgres completely (code, migrations, and wiring) rather than maintaining a dual-backend or compatibility layer. End-user APIs and workflow behaviors remain unchanged. The migration aligns our runtime with already-adopted Redis patterns (cache, rate limit, memory, resources), simplifies operations, and positions the platform for future vector features on Redis 8.x. All changes must keep the full test suite and linter green throughout delivery.

## Greenfield Approach (No Backwards Compatibility)

- Remove Postgres entirely from runtime paths: drivers, repositories, migrations, startup checks, and make targets tied to Postgres.
- Do not introduce a backend toggle or shadow/dual-read mode; Redis becomes the only persistence backend.
- Maintain behavior and functionality; tests and lints must pass. Old internal APIs may be replaced if needed to fit Redis, but public product behavior must remain intact.
- Update developer tooling, local/dev compose files, and documentation to assume Redis as the sole store.

## Goals

- Maintain identical external behavior across Auth, Workflow state, and Task state domains.
- Prefer stable public contracts; internal repository shapes may change if it improves correctness/fit for Redis.
- Ensure correctness of secondary indexes and ordering views required by current product use cases.
- Provide clear observability around the new persistence path (health, metrics, labels).
- Preserve delivery quality: tests and lints stay green at all times.
- Support retention policies (TTLs) for terminal workflow/task states; no TTLs for Auth entities.

## User Stories

- As a platform engineer, I can select the persistence backend via configuration so I can roll out Redis incrementally and safely.
- As a backend developer, I continue to use the same repository interfaces without code changes in domain layers.
- As QA, I rely on dual-read validation to confirm Redis responses match the baseline behavior with zero drift before flip.
- As an SRE, I can observe store health/metrics and see the active backend labeled in telemetry to operate and troubleshoot the system.

## Core Features

Describe WHAT must be delivered (not how), with numbered functional requirements.

1. Redis-native persistence (behavior parity)

R1. Implement Redis-native persistence for `engine/auth`, `engine/workflow`, and `engine/task`, preserving all current behaviors.

R2. Ensure query surfaces supported today remain available (e.g., lookups by ids/status/agent/tool/task hierarchy, latest-first listings).

R3. Guarantee atomicity and index correctness for writes that affect multiple logical views.

2. Wiring and bootstrap (Redis-only)

R4. Initialize the Redis client at startup, perform health check (ping), and label telemetry with `store_driver=redis`.

R5. Remove database migration logic and Postgres health checks from startup and ops paths.

3. Data views and ordering

R7. Maintain the ability to list workflow states and task states by the same keys and filters product relies on today.

R8. Provide created-at descending views used by product surfaces (e.g., latest-first per workflow and per parent task).

4. Retention and lifecycle

R9. Apply configurable TTLs to terminal task states (recommended 7–30 days) and to workflow states (recommended 30–90 days); Auth objects have no TTL.

R10. Document retention policies and the operational guidance for developers and SREs.

5. Validation

R11. Keep all existing domain-level behavior tests green; add unit/property tests for Redis persistence semantics and indexing.

6. Operations and tooling

R13. Offer an administrative reindex/check tool to detect and repair inconsistencies between canonical objects and derived index views (dry-run and repair modes).

R14. Provide metrics around repository operations (reads/writes, contention/locks, retries) comparable to current baselines.

7. Testability

R15. Ensure repository tests run with an in-memory/dev Redis (e.g., miniredis) without external Docker dependencies for unit tests.

R16. Keep existing domain-level behavior tests unchanged; the backend swap must remain transparent.

## User Experience

Primary users are internal roles (platform engineers, backend developers, QA, SRE). There is no UI surface change. Developer experience changes are limited to:

- Configuration: no backend toggle; Redis is the sole store.
- Observability: telemetry clearly indicates the active backend; standard dashboards include store health and key operation metrics.
- Documentation: short guides for retention policy, admin reindex/check commands, and rollout instructions.

Accessibility is not applicable (no end-user UI changes). Documentation must remain concise and discoverable in `docs/` and `README` quick start.

## High-Level Technical Constraints

- Adhere to core project requirements: `logger.FromContext(ctx)` and `config.FromContext(ctx)` usage, no global singletons, proper context propagation; avoid workarounds.
- External behavior and repository contracts must remain unchanged.
- Production store must be Redis 8.x or equivalent managed service providing required capabilities; keep existing Go client version policy.
- Data privacy/logging: avoid exposing sensitive identifiers or raw secrets in logs; include only key kinds where needed.
- Performance: no regression relative to the current baseline at p50/p95 for repository operations; improvements are desired and tracked.
- Reliability: provide clear health checks and metrics to support SRE operations.

Implementation details (e.g., specific scripts, internal transaction patterns) are covered by the technical specification and are out of scope for this PRD.

## Non-Goals (Out of Scope)

- Migrating or modifying Temporal’s own database (remains PostgreSQL).
- Adding Redis Stack or full-text/fuzzy search features in this phase; vector search enablement is not part of MVP.
- Changing public APIs, domain models, or repository interfaces.
- Introducing new user-facing UI or product features.
- Altering business logic in domains; only the persistence backend changes.

## Delivery Plan (Greenfield Cutover)

- Implement Redis persistence matching existing behaviors and queries.
- Wire Redis as the only backend; remove Postgres code, migrations, health checks, and tooling.
- Ensure unit/integration tests run green with in-memory/dev Redis for unit tests.
- Update developer tooling, docs, and dashboards to assume Redis-only.

## Success Metrics

- Quality gates: `make lint` and `make test` pass at each phase.
- Performance: no p50/p95 latency regression in repository operations compared to the existing baseline; track and report deltas.
- Reliability/ops: Redis health checks succeed at startup and during continuous operation; store label `store_driver=redis` appears in telemetry and dashboards.
- Testability: repository unit tests run without external Docker dependencies in CI for the Redis path.
- Retention: TTL policies applied to terminal workflow/task states; Auth has no TTL; documented and verified.
- Postgres removal: all Postgres-specific code paths, migrations, and make targets are deleted; compose/dev reflect Redis-only.

## Risks and Mitigations

- Index consistency drift → Validate via tests (including property/stress) and admin reindex/check tools; require atomic multi-view updates (implementation detail deferred to tech spec).
- Memory growth due to secondary indexes → Define retention policies; monitor keyspace size; add alerts and budgets; prune rarely used views as needed.
- Durability and recovery expectations → Operate Redis with recommended persistence settings; document operational guidance.
- Concurrency/locking semantics → Standardize lock behavior and metrics; validate with stress/property tests.
- Rollout risk (hidden regressions) → Phased rollout with config flag, shadow-mode validation, and explicit success gates.

## Open Questions

- Do we need additional search/indexing surfaces (e.g., fuzzy or cross-field) beyond exact matches in the near term? If so, are these better addressed post-migration?
- What performance improvement targets (quantitative) should be set beyond “no regression” for the MVP? (Track and decide post-baseline.)

## Appendix

- Source Technical Specification: `tasks/prd-redis/_techspec.md`
- Related operational documentation to be added under `docs/` (retention, backups/persistence, admin reindex/check usage).
