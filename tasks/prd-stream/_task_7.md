## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>observability</domain>
<type>telemetry</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 7.0: Streaming Observability & Telemetry

## Overview

Instrument the workflow, agent, and task SSE endpoints with first-class telemetry so operators can detect regressions and reason about live streams. This task implements the new metrics, logging, tracing, and alerting hooks identified in the observability research (active connection gauges, stream duration histograms, per-event counters, structured connect/disconnect logs, OpenTelemetry spans, and PromQL alerts for error spikes and latency). All instrumentation must align with existing monitoring infrastructure (`engine/infra/monitoring`) and respect project logging/config patterns (`logger.FromContext`, `config.FromContext`).

<requirements>
- Metrics:
  - Register new OTel instruments for active streams (Gauge), stream duration (Histogram), time-to-first-event (Histogram), events sent (Counter with `event_type`), and stream errors (Counter with `reason`). Use consistent naming via `engine/infra/monitoring/metrics` helpers and expose them in `engine/infra/monitoring`.
  - Increment/decrement metrics within `engine/workflow/router/stream.go`, `engine/agent/router/stream.go`, and `engine/task/router/stream.go` without exceeding 50-line function limits; prefer helper functions where needed.
- Logging:
  - Emit structured connect/disconnect logs (Info) and terminal error logs (Error) with exec IDs, workflow/agent/task identifiers, duration, total events, and close reason. Reuse `logger.FromContext(ctx)` and ensure no sensitive payloads are logged.
- Tracing:
  - Create or extend OpenTelemetry spans for the SSE connection lifecycle and per-event emission, stitching upstream trace context (Temporal, Redis) when available. Use existing tracing utilities if present.
- Alerts & Docs:
  - Document recommended PromQL alerts (error rate, connection drops, high time-to-first-event, stalled event flow) in `docs/content/docs/operations` or the most relevant runbook file, referencing metrics added above.
  - Update operator docs/readme (e.g., `docs/content/docs/api/overview.mdx` or a new `observability` note) describing the new telemetry signals and how to interpret them.
- Configuration:
  - Add any required knobs (sampling, histogram buckets) to config defaults via `config.FromContext`.
- Adhere to `.cursor/rules` (Go 1.25 features, no long functions, unit tests for new code, etc.).
</requirements>

## Subtasks

- [ ] 7.1 Define OTel metric instruments and registration helpers for SSE telemetry.
- [ ] 7.2 Wire metrics into each SSE handler (workflow, agent, task) with proper context handling and error tagging.
- [ ] 7.3 Extend structured logging and tracing spans for connect/disconnect and per-event emission.
- [ ] 7.4 Document PromQL alert templates and operator guidance for the new telemetry.

## Deliverables

- Updated SSE handlers and monitoring helpers with metrics, logging, and tracing instrumentation.
- Documentation updates covering metrics names, example dashboards, and alert queries.
- Configuration or helper additions required to expose telemetry safely in production.

## Tests

- Unit tests for metric registration (ensuring instruments exist with expected names/units) and for SSE handlers verifying counters/gauges update on connect/disconnect and error paths (use OTel test meter similar to existing monitoring tests).
- Tests confirming logging/tracing helpers populate expected attributes (table-driven or via test logger).
- Documentation lint/build remains green after updates (`bun run --cwd docs build`, `make lint`, `make test`).

## Success Criteria

- Operators can track active streams, latency, event volume, and error reasons through Prometheus.
- Logs and traces include enough context to diagnose disconnects or backend failures.
- Alerting guidance is published and aligns with the metrics emitted.
- All telemetry integrates cleanly with existing monitoring infrastructure and passes project CI gates.
