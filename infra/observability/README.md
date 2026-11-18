# Local observability stack

A Grafana OSS stack that receives the CompozyOS daemon's OpenTelemetry export, with dashboards and
data sources already provisioned. Use it to look at traces and metrics while you work on CompozyOS, or as a
starting point for your own collector setup. Export itself is documented on the
[telemetry export](../../packages/site/content/docs/operations/telemetry-export.mdx) page.

```
CompozyOS daemon ──OTLP──► OTel Collector ─► Tempo (traces) ──metrics-generator──┐
~/.compozy/logs ─filelog─┘        │      └─► Loki (logs)                        ▼
                                  └────────► Prometheus (metrics, exemplars) ◄───┘
                                                   ▲
                                Grafana (Tempo ↔ Loki ↔ Prometheus cross-linked)
```

| Component              | What it adds                                                                   | Port (127.0.0.1) |
| ---------------------- | ------------------------------------------------------------------------------ | ---------------- |
| OTel Collector contrib | receives OTLP; tails the daemon log file; converts delta metrics to cumulative | 4317, 4318       |
| Tempo                  | traces; metrics-generator for span RED metrics, service graph, TraceQL metrics | 3200             |
| Loki                   | daemon logs, Logs Drilldown                                                    | 3100             |
| Prometheus             | metrics through its OTLP receiver, exemplars that open the trace               | 9090             |
| Grafana                | provisioned data sources and the CompozyOS daemon dashboard                    | 3000             |

## Run

Requires Docker with Compose.

```bash
cd infra/observability
docker compose up -d
```

Turn on export in the daemon. The keys are operator-only and take effect after a restart:

```bash
compozy config set observability.otel.enabled true
compozy config set observability.otel.endpoint http://localhost:4318
compozy config set observability.otel.metric_interval 15s   # default 60s
compozy daemon stop && compozy daemon start
compozy status   # OTel: ok
```

Open <http://localhost:3000/d/compozy-daemon>.

## Where to look

- **CompozyOS daemon dashboard:** turns (p50/p95, time to first token), tokens and cost, tool calls, ACP RPC
  latency, SQLite writes, per-span latency from Tempo, recent turns (click one to open its trace), and daemon logs.
- **Explore → Tempo → TraceQL:** `{resource.service.name="compozy-daemon" && name="session.turn"}` lists turns.
  A turn reads `POST …/prompt → session.turn → acp session/prompt`, with `mcp.hosted.call → tool.execute`,
  `hook.dispatch` and `db.write` inside it.
- **Explore → Tempo → Service Graph** and **Drilldown → Traces / Metrics / Logs** for query-less exploration.
- From a span: **Logs for this span** (Loki, filtered by session) and **Span p95 / errors** (Prometheus).
- On a histogram panel, the exemplar dots open the trace.

## Settings

| Variable                 | Default           | Use                                                                      |
| ------------------------ | ----------------- | ------------------------------------------------------------------------ |
| `COMPOZY_LOG_DIR`        | `~/.compozy/logs` | Daemon log directory the collector tails.                                |
| `COMPOZY_LOG_TZ`         | `UTC`             | Time zone of the daemon's plain-text log lines, such as `Europe/Berlin`. |
| `LOCAL_UID`, `LOCAL_GID` | `1000`            | User the collector runs as; it must be able to read the log directory.   |

## Notes

- The daemon exports traces and metrics only. Logs come from the collector tailing `compozy.log` from its end, so
  older lines are not imported. Log lines carry `session_id`, not `trace_id`, so trace-to-logs links by session.
- Retention: Prometheus 30 days, Tempo and Loki 14 days. Data lives in the `compozy-observability_*` Docker volumes;
  `docker compose down -v` removes it.
- There is no authentication. Every port binds to 127.0.0.1; do not expose this stack on a network.
- Any other OTLP client can send to the same collector on `localhost:4317` (gRPC) or `localhost:4318` (HTTP).
