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
- `compozy_temporal_workflow_task_duration_seconds` - Workflow task execution duration histogram
    - Labels: `workflow_type`, `otel_scope_name`, `otel_scope_version`
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

## Prometheus Configuration

To scrape metrics from Compozy, add the following to your Prometheus configuration:

```yaml
scrape_configs:
    - job_name: "compozy"
      static_configs:
          - targets: ["localhost:8080"] # Adjust host/port as needed
      metrics_path: "/metrics" # Or your custom path
```

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
spec:
    selector:
        matchLabels:
            app: compozy
    endpoints:
        - port: http
          path: /metrics
```

## Troubleshooting

### Monitoring Not Working

1. Check if monitoring is enabled:

    - Verify `MONITORING_ENABLED` environment variable
    - Check `monitoring.enabled` in `compozy.yaml`

2. Verify the endpoint is accessible:

    ```bash
    curl http://localhost:8080/metrics
    ```

3. Check logs for monitoring initialization:
    - Look for "Monitoring service initialized successfully"
    - Or "Monitoring is disabled in the configuration"

### Invalid Configuration

If you see validation errors, check:

- Path starts with `/`
- Path doesn't contain `/api/`
- Path has no query parameters

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

## Notes

- Monitoring is designed to have minimal performance impact (<0.5% overhead)
- If monitoring fails to initialize, the application will continue running without metrics
- Metrics endpoint is not versioned (not under `/api/v0/`)
- Local metrics data persists across container restarts using Docker volumes
