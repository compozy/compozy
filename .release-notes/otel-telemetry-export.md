---
title: Export daemon traces and metrics with OpenTelemetry
type: feat
---

CompozyOS can now export daemon traces and metrics to an OpenTelemetry collector over OTLP HTTP or gRPC. Export is off by default. Operators enable it through the global `observability.otel.*` settings, store authentication in environment or vault-backed header refs, and check export health with `compozy status`. Exported values are sanitized, and collector failures never fail a session turn. The repository includes a local Grafana, Tempo, Loki and Prometheus stack under `infra/observability/`.

Migration notes: no action needed. Existing configuration and local observability keep working; the status API gains an optional `otel` object.
