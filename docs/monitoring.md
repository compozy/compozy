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
    - Labels: `method`, `path`, `status_code`
- `compozy_http_request_duration_seconds` - HTTP request latency histogram
    - Labels: `method`, `path`
- `compozy_http_requests_in_flight` - Number of currently active HTTP requests

### Temporal Workflow Metrics

- `compozy_temporal_workflow_started_total` - Total number of started workflows
    - Labels: `workflow_type`
- `compozy_temporal_workflow_completed_total` - Total number of completed workflows
    - Labels: `workflow_type`
- `compozy_temporal_workflow_failed_total` - Total number of failed workflows
    - Labels: `workflow_type`
- `compozy_temporal_workflow_duration_seconds` - Workflow execution duration histogram
    - Labels: `workflow_type`

### System Health Metrics

- `compozy_build_info` - Build information (always has value 1)
    - Labels: `version`, `commit_hash`, `go_version`
- `compozy_uptime_seconds_total` - Service uptime in seconds

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

## Notes

- Monitoring is designed to have minimal performance impact (<0.5% overhead)
- If monitoring fails to initialize, the application will continue running without metrics
- Metrics endpoint is not versioned (not under `/api/v0/`)
