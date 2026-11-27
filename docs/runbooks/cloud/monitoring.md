# Monitor and page for Compozy Cloud

Grafana Cloud receives Prometheus remote write once a minute. Select `env="production"` for paging. Staging and production use separate credentials and share the hosted stack. Counters and link counts are per replica. Use `sum` for counters and live control links, and `max` for database-wide gauges reported by both replicas.

| Alert | Production rule | Action |
| --- | --- | --- |
| Availability | Below 99 percent over five minutes for health or activation | Page |
| Sign-in failures | Error share above 5 percent over 15 minutes | Page |
| Stripe webhook failures | At least one server failure in 15 minutes | Page |
| Hosted tunnels | 700 allocated | Warn |
| Hosted tunnels | 900 allocated | Page |
| Database latency | p95 above 200 ms | Page |
| Grant latency | p95 above 300 ms | Warn |

Use these Grafana expressions with a one-minute evaluation interval. Set a no-data alert on the production receiver or heartbeat so a failed exporter is visible.

```promql
sum(increase(cloud_sign_ins_total{env="production",result="error"}[15m]))
  / clamp_min(sum(increase(cloud_sign_ins_total{env="production"}[15m])), 1) > 0.05

sum(increase(cloud_stripe_webhook_failures_total{env="production"}[15m])) > 0

max(cloud_tunnels_allocated{env="production"}) >= 700
max(cloud_tunnels_allocated{env="production"}) >= 900

histogram_quantile(0.95, sum by (le) (rate(cloud_db_latency_seconds_bucket{env="production"}[5m]))) > 0.2
histogram_quantile(0.95, sum by (le) (rate(cloud_grant_latency_seconds_bucket{env="production"}[5m]))) > 0.3
```

The 7-day activation share is calculated by provider for accounts created in the last 30 days. An account qualifies when a linked computer reports a valid session mint, or an enrolled installation is seen on a later UTC day, within seven days of account creation.

The Sentry project is `compozy-cloud`. Route incidents by `environment`. Events carry an operation label and, when supplied by the cloud client, a bounded client version and platform. Use the request correlation in private operational logs if additional investigation is needed.

## Better Stack

- Unpause monitor `5285073` for `https://cloud.compozy.com/healthz` and monitor `5285075` for `https://cloud.compozy.com/activate` after the first production smoke. Both run every 180 seconds from US and EU with 120 seconds of confirmation. Verify notification delivery to the operator's email.
- Until the paid plan is authorized, route Grafana alerts to the operator's email and keep the Better Stack email contact enabled. The current check cadence is a degraded availability signal for the five-minute target.
- Before stable, enable the paid configuration for a one-minute availability check, `status.compozy.com`, and phone paging. Create the public status page in the **Compozy** team, bind its custom domain using Better Stack's DNS instructions, add both checks, and verify a test incident and call. Route Grafana alerts to the team's incoming-webhook integration and assign the operator's on-call schedule. Publish and resolve incidents on the status page.
