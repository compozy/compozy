---
status: resolved
file: internal/telemetry/otel.go
line: 10
severity: medium
author: claude-code
provider_ref:
---

# Issue 001: Missing OTLP exporter shutdown

## Review Comment

The tracer provider was never flushed on shutdown; spans were dropped. Add a graceful flush.

## Triage

- Decision: `VALID`
- Notes: Fixed with a deferred shutdown flush.
