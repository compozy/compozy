---
status: pending
---

# Task 9.0: Complete Documentation

## Overview

Create comprehensive documentation for the monitoring feature, including setup guides, Kubernetes deployment examples, troubleshooting guides, and rollback procedures.

## Subtasks

- [ ] 9.1 Write monitoring setup guide in docs/monitoring.md
- [ ] 9.2 Add Kubernetes deployment examples with ServiceMonitor
- [ ] 9.3 Document Pod annotations for Prometheus scraping
- [ ] 9.4 Create troubleshooting guide for common monitoring issues
- [ ] 9.5 Document rollback procedures for disabling monitoring
- [ ] 9.6 Add monitoring configuration examples to README

## Implementation Details

### 9.1 Monitoring Setup Guide

Create `docs/monitoring.md`:

````markdown
# Monitoring Guide for Compozy

Compozy provides built-in Prometheus monitoring to track HTTP requests, Temporal workflows, and system health.

## Overview

The monitoring feature exposes metrics at the `/metrics` endpoint in Prometheus exposition format. These metrics can be scraped by Prometheus servers and visualized in Grafana or other monitoring tools.

## Quick Start

### Enable Monitoring

1. **Via Environment Variable:**
    ```bash
    export MONITORING_ENABLED=true
    compozy serve
    ```
````

2. **Via Configuration File:**
    ```yaml
    # compozy.yaml
    monitoring:
        enabled: true
        path: /metrics # optional, defaults to /metrics
    ```

### Verify Metrics

Once enabled, you can view metrics at:

```bash
curl http://localhost:8080/metrics
```

## Available Metrics

### HTTP Metrics

- `compozy_http_requests_total`: Total HTTP requests (labels: method, path, status_code)
- `compozy_http_request_duration_seconds`: Request latency histogram
- `compozy_http_requests_in_flight`: Currently active requests

### Temporal Workflow Metrics

- `compozy_temporal_workflow_started_total`: Started workflows (label: workflow_type)
- `compozy_temporal_workflow_completed_total`: Successfully completed workflows
- `compozy_temporal_workflow_failed_total`: Failed workflows
- `compozy_temporal_workflow_duration_seconds`: Workflow execution time histogram
- `compozy_temporal_workers_running_total`: Currently running workers
- `compozy_temporal_workers_configured_total`: Configured workers per instance

### System Metrics

- `compozy_build_info`: Build information (labels: version, commit_hash, go_version)
- `compozy_uptime_seconds_total`: Service uptime counter

## Configuration

### Environment Variables

- `MONITORING_ENABLED`: Enable/disable monitoring (overrides YAML config)

### YAML Configuration

```yaml
monitoring:
    enabled: true # Enable monitoring
    path: /metrics # Metrics endpoint path (default: /metrics)
```

### Configuration Precedence

1. Environment variables (highest priority)
2. YAML configuration
3. Default values (lowest priority)

## Performance Impact

The monitoring implementation has been validated to have:

- <0.5% impact on 95th percentile latency
- <2% increase in CPU/memory usage
- Negligible impact under normal load conditions

## Security Considerations

### Network Protection

In production, restrict access to `/metrics` at the network level:

- Use firewall rules to limit access to Prometheus servers
- Configure ingress controllers to restrict the endpoint
- Consider adding authentication in future versions

### Data Privacy

- No personally identifiable information (PII) is included in metrics
- HTTP path labels use route templates (e.g., `/users/:id` not `/users/123`)
- Error details are logged, not included in metric labels

````

### 9.2 Kubernetes Deployment Examples

Add to `docs/monitoring.md`:

```markdown
## Kubernetes Integration

### ServiceMonitor for Prometheus Operator

Create a ServiceMonitor resource for automatic Prometheus discovery:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: compozy-monitor
  namespace: default
  labels:
    app: compozy
    team: backend
spec:
  selector:
    matchLabels:
      app: compozy
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s
````

### Manual Prometheus Configuration

If not using Prometheus Operator, add to Prometheus config:

```yaml
scrape_configs:
    - job_name: "compozy"
      static_configs:
          - targets: ["compozy-service.default.svc.cluster.local:8080"]
      metrics_path: "/metrics"
```

````

### 9.3 Pod Annotations Documentation

```markdown
### Pod Annotations for Prometheus

Add these annotations to your Pod or Deployment for Prometheus autodiscovery:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: compozy
spec:
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/path: "/metrics"
        prometheus.io/port: "8080"
    spec:
      containers:
        - name: compozy
          image: compozy:latest
          env:
            - name: MONITORING_ENABLED
              value: "true"
````

### Helm Values Example

If using Helm:

```yaml
# values.yaml
monitoring:
    enabled: true

podAnnotations:
    prometheus.io/scrape: "true"
    prometheus.io/path: "/metrics"
    prometheus.io/port: "8080"
```

````

### 9.4 Troubleshooting Guide

```markdown
## Troubleshooting

### Metrics Endpoint Returns 503

**Problem:** `/metrics` endpoint returns HTTP 503 Service Unavailable

**Causes:**
1. Monitoring service failed to initialize
2. Prometheus exporter initialization error

**Solutions:**
1. Check application logs for initialization errors:
   ```bash
   grep "monitoring" app.log
````

2. Verify OpenTelemetry dependencies are properly installed
3. Ensure no port conflicts for the metrics endpoint

### No Metrics Appearing

**Problem:** Metrics endpoint responds but shows no metrics

**Causes:**

1. Monitoring is disabled
2. No traffic has been processed yet

**Solutions:**

1. Verify monitoring is enabled:
    ```bash
    echo $MONITORING_ENABLED
    ```
2. Check configuration file for `monitoring.enabled: true`
3. Generate some traffic to populate metrics

### High Memory Usage

**Problem:** Memory usage increases significantly with monitoring

**Causes:**

1. High cardinality labels (should be prevented by design)
2. Metric accumulation over time

**Solutions:**

1. Verify only allowed labels are used
2. Check for custom metrics with high cardinality
3. Restart service to clear accumulated metrics

### Prometheus Can't Scrape Metrics

**Problem:** Prometheus shows target as DOWN

**Solutions:**

1. Verify network connectivity:
    ```bash
    curl http://compozy-pod:8080/metrics
    ```
2. Check ServiceMonitor selector matches service labels
3. Verify pod annotations are correct
4. Check Prometheus logs for scrape errors

````

### 9.5 Rollback Procedures

```markdown
## Rollback Procedures

### Disabling Monitoring

To completely disable monitoring:

1. **Set environment variable:**
   ```bash
   export MONITORING_ENABLED=false
````

2. **Update configuration:**

    ```yaml
    monitoring:
        enabled: false
    ```

3. **Redeploy the application:**

    ```bash
    kubectl rollout restart deployment/compozy
    ```

4. **Remove Kubernetes resources:**

    ```bash
    # Remove ServiceMonitor
    kubectl delete servicemonitor compozy-monitor
    
    # Remove pod annotations
    kubectl annotate pods -l app=compozy \
        prometheus.io/scrape- \
        prometheus.io/path- \
        prometheus.io/port-
    ```

### Verification

Verify monitoring is disabled:

```bash
# Should return 503 or 404
curl -I http://localhost:8080/metrics
```

### Performance Verification

After disabling, verify no performance overhead:

1. Check CPU/memory usage returns to baseline
2. Verify request latencies are normal
3. Confirm no monitoring-related logs appear

````

### 9.6 README Updates

Add to main `README.md`:

```markdown
## Monitoring

Compozy includes built-in Prometheus monitoring for observability.

### Quick Start

Enable monitoring:
```bash
export MONITORING_ENABLED=true
compozy serve
````

View metrics:

```bash
curl http://localhost:8080/metrics
```

### Configuration

```yaml
# compozy.yaml
monitoring:
    enabled: true
    path: /metrics
```

See [docs/monitoring.md](docs/monitoring.md) for complete documentation.

### Available Metrics

- **HTTP**: Request rates, latencies, and active connections
- **Temporal**: Workflow execution metrics and worker status
- **System**: Build info and uptime

### Performance

Monitoring adds <0.5% latency overhead and <2% resource usage.

```

## Success Criteria
- Comprehensive monitoring guide created
- All metrics documented with descriptions
- Kubernetes integration examples provided
- ServiceMonitor and Pod annotation examples included
- Clear troubleshooting steps for common issues
- Rollback procedure documented and tested
- README updated with monitoring section
- Documentation follows project style guidelines
```
