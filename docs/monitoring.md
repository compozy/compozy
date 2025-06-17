# Monitoring Configuration

Compozy provides built-in monitoring capabilities using Prometheus-compatible metrics. This guide explains how to configure and use the monitoring features.

## Overview

The monitoring feature exposes a `/metrics` endpoint that provides operational metrics in Prometheus exposition format. This endpoint can be scraped by Prometheus servers to collect metrics about HTTP requests, Temporal workflows, and system health.

## Configuration

Monitoring can be configured through environment variables or the project's `compozy.yaml` file.

### Configuration Precedence

Configuration values are applied in the following order (highest to lowest priority):

1. **Environment variables** - Always take precedence
2. **YAML configuration** - Project-specific settings
3. **Default values** - Built-in defaults

### Environment Variables

- `MONITORING_ENABLED` - Enable or disable monitoring globally (values: `true` or `false`)
    - Takes precedence over any YAML configuration
    - Default: `false`
- `MONITORING_PATH` - Set the metrics endpoint path
    - Takes precedence over any YAML configuration
    - Default: `/metrics`

Example:

```bash
export MONITORING_ENABLED=true
export MONITORING_PATH=/custom/metrics
compozy run
```

### YAML Configuration

Add a `monitoring` section to your `compozy.yaml` file:

```yaml
name: my-project
version: 0.1.0

workflows:
    - source: ./workflow.yaml

monitoring:
    enabled: true # Enable monitoring (default: false)
    path: /metrics # Metrics endpoint path (default: /metrics)

runtime:
    permissions:
        - --allow-read
        - --allow-net
        - --allow-env
```

### Configuration Options

| Option               | Type    | Default    | Description                   |
| -------------------- | ------- | ---------- | ----------------------------- |
| `monitoring.enabled` | boolean | `false`    | Enable or disable monitoring  |
| `monitoring.path`    | string  | `/metrics` | Path for the metrics endpoint |

### Path Validation

The monitoring path must:

- Start with `/`
- Not be under `/api/` (to avoid conflicts with API routes)
- Not contain query parameters

Valid examples: `/metrics`, `/monitoring/metrics`, `/custom-metrics`

Invalid examples: `/api/metrics`, `metrics`, `/metrics?format=json`

## Available Metrics

### HTTP Metrics

- `compozy_http_requests_total` - Total number of HTTP requests
    - Labels: `method`, `path`, `status_code`, `otel_scope_name`, `otel_scope_version`
- `compozy_http_request_duration_seconds` - HTTP request latency histogram
    - Labels: `method`, `path`, `status_code`, `otel_scope_name`, `otel_scope_version`
- `compozy_http_requests_in_flight` - Number of currently active HTTP requests
    - Labels: `otel_scope_name`, `otel_scope_version`

### Temporal Workflow Metrics

- `compozy_temporal_workflow_started_total` - Total number of started workflows
    - Labels: `workflow_type`, `otel_scope_name`, `otel_scope_version`
- `compozy_temporal_workflow_completed_total` - Total number of completed workflows
    - Labels: `workflow_type`, `otel_scope_name`, `otel_scope_version`
- `compozy_temporal_workflow_failed_total` - Total number of failed workflows
    - Labels: `workflow_type`, `otel_scope_name`, `otel_scope_version`
- `compozy_temporal_workflow_duration_seconds` - Workflow execution duration histogram
    - Labels: `workflow_type`, `otel_scope_name`, `otel_scope_version`
- `compozy_temporal_workers_running_total` - Number of currently running Temporal workers
    - Labels: `otel_scope_name`, `otel_scope_version`
- `compozy_temporal_workers_configured_total` - Number of configured Temporal workers
    - Labels: `otel_scope_name`, `otel_scope_version`

### System Health Metrics

- `compozy_build_info` - Build information (always has value 1)
    - Labels: `version`, `commit_hash`, `go_version`, `otel_scope_name`, `otel_scope_version`
- `compozy_uptime_seconds` - Service uptime in seconds
    - Labels: `otel_scope_name`, `otel_scope_version`

### OpenTelemetry Metadata

- `otel_scope_info` - Instrumentation scope metadata
- `target_info` - Target metadata with service information

## Example Configurations

### Basic Setup

Enable monitoring with default settings:

```yaml
monitoring:
    enabled: true
```

### Custom Metrics Path

Use a custom path for the metrics endpoint:

```yaml
monitoring:
    enabled: true
    path: /custom/metrics
```

### Environment Override

Enable monitoring via environment variable, overriding YAML config:

```bash
# Even if compozy.yaml has enabled: false
export MONITORING_ENABLED=true
compozy run
```

## Performance Impact

The monitoring implementation has been validated to have minimal performance impact:

- **Latency Overhead**: <0.5% increase in 95th percentile latency
- **CPU Usage**: <2% increase in CPU usage under normal load
- **Memory Usage**: <2% increase in memory usage
- **Negligible Impact**: Under typical workload conditions

### Performance Validation

The monitoring feature has been formally validated using:

- **Load Testing Tool**: `ghz` for gRPC/HTTP load generation
- **Profiling Tool**: `pprof` for performance profiling
- **Test Environment**: AWS `t3.medium` instance (2vCPU, 4GB RAM)
- **Load Profile**: 1,000 requests per second for 5 minutes
- **Success Criteria**: All overhead metrics remain within specified thresholds

## Prometheus Configuration

To scrape metrics from Compozy, add the following to your Prometheus configuration:

```yaml
scrape_configs:
    - job_name: "compozy"
      static_configs:
          - targets: ["localhost:8080"] # Adjust host/port as needed
      metrics_path: "/metrics" # Or your custom path
```

## Security Considerations

### Network Protection

In production environments, restrict access to the `/metrics` endpoint at the network level:

- **Firewall Rules**: Limit access to Prometheus servers only
- **Ingress Controllers**: Configure ingress to restrict the endpoint
- **Network Policies**: Use Kubernetes network policies for additional security

**Example Nginx Ingress restriction:**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
    annotations:
        nginx.ingress.kubernetes.io/whitelist-source-range: "10.0.0.0/8,172.16.0.0/12"
spec:
    rules:
        - http:
              paths:
                  - path: /metrics
                    pathType: Exact
                    backend:
                        service:
                            name: compozy
                            port:
                                number: 8080
```

### Data Privacy

- **No PII**: No personally identifiable information is included in metrics
- **Path Sanitization**: HTTP path labels use route templates (e.g., `/users/:id` not `/users/123`)
- **Error Isolation**: Error details are logged separately, not included in metric labels
- **Label Constraints**: Only approved, low-cardinality labels are used to prevent data leakage

## Kubernetes Integration

For Kubernetes deployments, you can use annotations or ServiceMonitor resources:

### Pod Annotations

```yaml
apiVersion: v1
kind: Pod
metadata:
    annotations:
        prometheus.io/scrape: "true"
        prometheus.io/path: "/metrics"
        prometheus.io/port: "8080"
```

### ServiceMonitor (Prometheus Operator)

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
```

### Manual Prometheus Configuration

If not using Prometheus Operator, add to your Prometheus configuration:

```yaml
scrape_configs:
    - job_name: "compozy"
      kubernetes_sd_configs:
          - role: pod
      relabel_configs:
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
            action: keep
            regex: true
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
            action: replace
            target_label: __metrics_path__
            regex: (.+)
          - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
            action: replace
            regex: ([^:]+)(?::\d+)?;(\d+)
            replacement: $1:$2
            target_label: __address__
```

### Helm Values Example

If using Helm charts:

```yaml
# values.yaml
monitoring:
    enabled: true

podAnnotations:
    prometheus.io/scrape: "true"
    prometheus.io/path: "/metrics"
    prometheus.io/port: "8080"

service:
    labels:
        app: compozy

serviceMonitor:
    enabled: true
    labels:
        team: backend
    interval: 30s
    scrapeTimeout: 10s
```

## Troubleshooting

### Metrics Endpoint Returns 503

**Problem:** `/metrics` endpoint returns HTTP 503 Service Unavailable

**Causes:**

1. Monitoring service failed to initialize
2. OpenTelemetry exporter initialization error
3. Port conflicts for the metrics endpoint

**Solutions:**

1. Check application logs for initialization errors:
    ```bash
    grep "monitoring" app.log
    ```
2. Verify OpenTelemetry dependencies are properly installed
3. Ensure no port conflicts for the metrics endpoint
4. Check if monitoring is enabled:
    - Verify `MONITORING_ENABLED` environment variable
    - Check `monitoring.enabled` in `compozy.yaml`

### No Metrics Appearing

**Problem:** Metrics endpoint responds but shows no metrics

**Causes:**

1. Monitoring is disabled
2. No traffic has been processed yet
3. Metrics collection failed silently

**Solutions:**

1. Verify monitoring is enabled:
    ```bash
    echo $MONITORING_ENABLED
    ```
2. Check configuration file for `monitoring.enabled: true`
3. Generate some traffic to populate metrics:
    ```bash
    curl -X POST http://localhost:8080/api/v0/workflows/test/executions \
        -H "Content-Type: application/json" \
        -d '{}'
    ```
4. Check logs for monitoring initialization:
    - Look for "Monitoring service initialized successfully"
    - Or "Monitoring is disabled in the configuration"

### High Memory Usage

**Problem:** Memory usage increases significantly with monitoring enabled

**Causes:**

1. High cardinality labels (should be prevented by design)
2. Metric accumulation over time
3. Memory leaks in instrumentation

**Solutions:**

1. Verify only approved labels are used (check metric definitions)
2. Check for custom metrics with high cardinality
3. Restart service to clear accumulated metrics
4. Monitor metric cardinality in Prometheus UI

### Prometheus Can't Scrape Metrics

**Problem:** Prometheus shows target as DOWN or scrape failures

**Solutions:**

1. Verify network connectivity:
    ```bash
    curl http://compozy-pod:8080/metrics
    ```
2. Check ServiceMonitor selector matches service labels
3. Verify pod annotations are correct and complete
4. Check Prometheus logs for specific scrape errors
5. Ensure firewall rules allow Prometheus access
6. Verify the metrics endpoint is accessible from Prometheus namespace

### Invalid Configuration

**Problem:** Configuration validation errors on startup

**Check these requirements:**

- Path starts with `/`
- Path doesn't contain `/api/` (to avoid conflicts)
- Path has no query parameters
- Boolean values are `true` or `false` (not `yes`/`no`)

**Valid examples:** `/metrics`, `/monitoring/metrics`, `/custom-metrics`

**Invalid examples:** `/api/metrics`, `metrics`, `/metrics?format=json`

## Local Development

For local development and testing, Compozy provides a complete monitoring stack with Prometheus and Grafana using Docker Compose.

### Quick Start

1. **Start the full development stack (includes metrics):**

    ```bash
    make start-docker
    ```

2. **Start development server:**

    ```bash
    make dev
    ```

3. **Access monitoring tools:**
    - **Prometheus**: http://localhost:9090
    - **Grafana**: http://localhost:3000
        - Username: `admin`
        - Password: `admin`

### Available Commands

| Command             | Description                                    |
| ------------------- | ---------------------------------------------- |
| `make start-docker` | Start full development stack including metrics |
| `make stop-docker`  | Stop all development services                  |
| `make clean-docker` | Stop and remove all volumes                    |
| `make dev`          | Start development server with hot reload       |

### What's Included

The local monitoring setup includes:

**Prometheus** (http://localhost:9090):

- Scrapes metrics from Compozy application (`/metrics`)
- Scrapes metrics from Temporal server (`:8000/metrics`)
- 7-day data retention for development

**Grafana** (http://localhost:3000):

- Pre-configured Prometheus datasource
- Sample dashboard showing:
    - HTTP request rates and latency
    - Temporal workflow metrics
    - System health indicators
    - Build information

**Temporal Metrics Integration**:

- Temporal server exposes metrics on port 8000
- Integration with existing temporal-network
- Built-in workflow and system metrics

### Environment Variables

You can customize the local setup with these variables in `.env`:

```bash
# Prometheus
PROMETHEUS_PORT=9090

# Grafana
GRAFANA_PORT=3000
GRAFANA_USER=admin
GRAFANA_PASSWORD=admin

# Temporal Metrics
TEMPORAL_METRICS_PORT=8000
```

### Accessing Metrics

Once the stack is running:

1. **Application metrics**: http://localhost:8080/metrics
2. **Temporal metrics**: http://localhost:8000/metrics
3. **Prometheus targets**: http://localhost:9090/targets
4. **Grafana dashboard**: http://localhost:3000 → "Compozy Monitoring Dashboard"

### Troubleshooting Local Setup

**"Datasource prometheus was not found" error:**

1. Wait 30-60 seconds after starting - Grafana needs time to provision datasources
2. Check Grafana logs: `docker logs grafana`
3. Verify Prometheus is running: `docker logs prometheus`
4. Check datasource status in Grafana UI: http://localhost:3000/datasources

**Dashboard shows "No data":**

1. Ensure monitoring is enabled in your application configuration:
    ```yaml
    monitoring:
        enabled: true
    ```
2. Verify metrics endpoints are accessible:
    ```bash
    curl http://localhost:8080/metrics
    curl http://localhost:8000/metrics
    ```
3. Check Prometheus targets status: http://localhost:9090/targets
4. Verify data is being scraped: Go to Prometheus → Graph → Search for `compozy_`
5. Make some HTTP requests to generate metrics:
    ```bash
    curl -X POST http://localhost:8080/api/v0/workflows/test/executions \
        -H "Content-Type: application/json" \
        -d '{}'
    ```

**Services not starting:**

1. Check for port conflicts: `docker ps`
2. Verify volumes: `docker volume ls | grep grafana`
3. Reset if needed: `make clean-docker && make start-docker`

### Customizing Dashboards

Grafana dashboards are automatically provisioned from `cluster/grafana/dashboards/`. You can:

1. Modify existing dashboards in Grafana UI
2. Export and save changes to JSON files
3. Add new dashboards to the provisioning directory

### Integration with Development Workflow

The monitoring stack integrates seamlessly with the existing development setup:

- Uses the same `temporal-network` Docker network
- Shares database connections and services
- Starts automatically with `make start-docker`
- No conflicts with existing ports or volumes

## Rollback Procedures

### Disabling Monitoring

To completely disable monitoring in your deployment:

#### 1. Environment Variable Method

Set the environment variable to disable monitoring:

```bash
export MONITORING_ENABLED=false
```

For containerized deployments, update your deployment configuration:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
    name: compozy
spec:
    template:
        spec:
            containers:
                - name: compozy
                  env:
                      - name: MONITORING_ENABLED
                        value: "false"
```

#### 2. Configuration File Method

Update your `compozy.yaml` file:

```yaml
monitoring:
    enabled: false
```

#### 3. Kubernetes Rollback Steps

For complete rollback in Kubernetes environments:

```bash
# 1. Update deployment to disable monitoring
kubectl patch deployment compozy -p '{"spec":{"template":{"spec":{"containers":[{"name":"compozy","env":[{"name":"MONITORING_ENABLED","value":"false"}]}]}}}}'

# 2. Remove ServiceMonitor
kubectl delete servicemonitor compozy-monitor

# 3. Remove pod annotations
kubectl annotate pods -l app=compozy \
    prometheus.io/scrape- \
    prometheus.io/path- \
    prometheus.io/port-

# 4. Redeploy the application
kubectl rollout restart deployment/compozy
```

#### 4. Docker Compose Rollback

For local development environments:

```bash
# 1. Update environment variable in docker-compose.yml or .env
echo "MONITORING_ENABLED=false" >> .env

# 2. Restart the services
make stop-docker && make start-docker
```

### Verification Steps

After disabling monitoring, verify the rollback was successful:

#### 1. Verify Endpoint is Disabled

```bash
# Should return 503, 404, or connection refused
curl -I http://localhost:8080/metrics
```

#### 2. Check Application Logs

Look for confirmation that monitoring is disabled:

```bash
grep -i "monitoring.*disabled" app.log
```

#### 3. Performance Verification

After disabling, verify no performance overhead remains:

1. **CPU/Memory Usage**: Should return to baseline levels
2. **Request Latencies**: Should match pre-monitoring performance
3. **Application Logs**: No monitoring-related log entries should appear

#### 4. Prometheus Verification

If Prometheus was scraping the endpoint:

1. **Check Prometheus Targets**: The target should show as DOWN or be removed
2. **Metric Availability**: Existing metrics will remain until data retention expires
3. **Scrape Errors**: Should see "connection refused" or similar errors in Prometheus logs

### Emergency Rollback

For immediate rollback in production issues:

```bash
# Quick disable via environment variable (fastest method)
kubectl set env deployment/compozy MONITORING_ENABLED=false

# Verify rollout
kubectl rollout status deployment/compozy
```

### Cleanup Persistent Data

To remove all monitoring-related data:

#### Local Development

```bash
# Remove Prometheus data
docker volume rm prometheus_data

# Remove Grafana data
docker volume rm grafana-storage

# Full cleanup
make clean-docker
```

#### Kubernetes

```bash
# Remove monitoring-related ConfigMaps
kubectl delete configmap -l app=compozy,component=monitoring

# Remove ServiceMonitor
kubectl delete servicemonitor -l app=compozy

# Clean up persistent volumes if used
kubectl delete pvc -l app=compozy,component=monitoring
```

## Notes

- Monitoring is designed to have minimal performance impact (<0.5% overhead)
- If monitoring fails to initialize, the application will continue running without metrics
- Metrics endpoint is not versioned (not under `/api/v0/`)
- Local metrics data persists across container restarts using Docker volumes
