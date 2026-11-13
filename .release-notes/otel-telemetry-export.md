---
title: Export daemon traces and metrics with OpenTelemetry
type: feat
---

CompozyOS can now export daemon traces and metrics to an OpenTelemetry collector over OTLP HTTP or gRPC. Export is off by default. Operators enable it through the global `observability.otel.*` settings, store authentication in environment or vault-backed header refs, and check export health with `compozy status`. Exported values are sanitized, and collector failures never fail a session turn. The repository includes a local Grafana, Tempo, Loki and Prometheus stack under `infra/observability/`.

Migration notes: provider processes no longer inherit `OTEL_*` or `TRACEPARENT` from the daemon environment, including when daemon export is off. If you previously used that inheritance for independent provider telemetry, move those variables into the agent's `session_env`. Alternatively, enable daemon export and `observability.otel.forward_to_providers` to share its collector settings. Existing local observability keeps working; the status API gains an optional `otel` object.
