# Dispatcher Monitoring Infrastructure

This document describes the comprehensive dispatcher monitoring infrastructure that has been added to Compozy.

## Overview

The dispatcher monitoring system provides detailed observability into dispatcher health, lifecycle events, heartbeat tracking, and performance metrics. It integrates seamlessly with the existing OpenTelemetry + Prometheus monitoring stack.

## Architecture

### Components

1. **Temporal Interceptor Extensions** (`engine/infra/monitoring/interceptor/temporal.go`)

    - Added dispatcher-specific metrics to the existing Temporal interceptor
    - Tracks dispatcher lifecycle events, heartbeats, and uptime

2. **Dispatcher Health Monitoring** (`engine/infra/monitoring/dispatcher.go`)

    - Dedicated dispatcher health status tracking
    - Stale dispatcher detection
    - Health aggregation functions

3. **Worker Integration** (`engine/worker/mod.go`)

    - Automatic dispatcher registration/unregistration
    - Lifecycle event emission
    - Heartbeat metric recording

4. **Grafana Dashboard** (`cluster/grafana/dashboards/compozy-monitoring.json`)
    - Visual monitoring panels for dispatcher health
    - Real-time metrics visualization

## Metrics Exported

### Dispatcher Lifecycle Metrics

| Metric Name                                 | Type            | Description                  | Labels                   |
| ------------------------------------------- | --------------- | ---------------------------- | ------------------------ |
| `compozy_dispatcher_active_total`           | UpDownCounter   | Currently active dispatchers | `dispatcher_id`          |
| `compozy_dispatcher_heartbeat_total`        | Counter         | Total dispatcher heartbeats  | `dispatcher_id`          |
| `compozy_dispatcher_lifecycle_events_total` | Counter         | Total lifecycle events       | `dispatcher_id`, `event` |
| `compozy_dispatcher_uptime_seconds`         | ObservableGauge | Dispatcher uptime in seconds | `dispatcher_id`          |

### Dispatcher Health Metrics

| Metric Name                        | Type            | Description                            | Labels                                                                      |
| ---------------------------------- | --------------- | -------------------------------------- | --------------------------------------------------------------------------- |
| `compozy_dispatcher_health_status` | ObservableGauge | Health status (1=healthy, 0=unhealthy) | `dispatcher_id`, `is_stale`, `time_since_heartbeat`, `consecutive_failures` |

## API Functions

### Temporal Interceptor Functions

- `StartDispatcher(ctx, dispatcherID)` - Records dispatcher start event
- `StopDispatcher(ctx, dispatcherID)` - Records dispatcher stop event
- `RecordDispatcherHeartbeat(ctx, dispatcherID)` - Records heartbeat event
- `RecordDispatcherRestart(ctx, dispatcherID)` - Records restart event

### Health Monitoring Functions

- `RegisterDispatcher(ctx, dispatcherID, staleThreshold)` - Register for health monitoring
- `UnregisterDispatcher(ctx, dispatcherID)` - Remove from health monitoring
- `UpdateDispatcherHeartbeat(ctx, dispatcherID)` - Update heartbeat timestamp
- `GetDispatcherHealth(dispatcherID)` - Get health status for specific dispatcher
- `GetAllDispatcherHealth()` - Get health status for all dispatchers
- `GetHealthyDispatcherCount()` - Count of healthy dispatchers
- `GetStaleDispatcherCount()` - Count of stale dispatchers

## Integration Points

### Worker Lifecycle Integration

The worker automatically:

1. Registers dispatchers for health monitoring on startup
2. Records start events when dispatchers are launched
3. Records stop events when dispatchers are terminated
4. Updates heartbeat metrics when heartbeat activities execute
5. Unregisters dispatchers during cleanup

### Heartbeat Activity Integration

The `DispatcherHeartbeat` activity (`engine/worker/activities/dispatcher_heartbeat.go`) now automatically:

- Records heartbeat metrics via `RecordDispatcherHeartbeat()`
- Updates health monitoring via `UpdateDispatcherHeartbeat()`

## Grafana Dashboard Panels

The enhanced dashboard includes a new "Dispatcher Health & Monitoring" section with:

1. **Dispatcher Health Status** - Color-coded health status per dispatcher
2. **Active Dispatchers** - Time series of active dispatcher count
3. **Dispatcher Uptime** - Current uptime for each dispatcher
4. **Dispatcher Heartbeat Rate** - Heartbeat frequency over time
5. **Dispatcher Lifecycle Events** - Start/stop/restart event rates with color coding

## Configuration

### Health Monitoring Configuration

- **Stale Threshold**: Configurable per dispatcher (default: 2 minutes)
- **Health Check Frequency**: Automatic via metric callback (every scrape)
- **Heartbeat TTL**: 5 minutes (Redis storage)

### Monitoring Service Integration

Dispatcher health metrics are automatically initialized when the monitoring service starts, provided monitoring is enabled in the project configuration.

## Usage Examples

### Querying Metrics (PromQL)

```promql
# Active dispatcher count
compozy_dispatcher_active_total

# Dispatcher health status
compozy_dispatcher_health_status

# Heartbeat rate over 5 minutes
rate(compozy_dispatcher_heartbeat_total[5m])

# Lifecycle events by type
rate(compozy_dispatcher_lifecycle_events_total[5m])

# Dispatcher uptime
compozy_dispatcher_uptime_seconds
```

### Health Checks

```go
// Check if a specific dispatcher is healthy
health, exists := monitoring.GetDispatcherHealth("my-dispatcher")
if exists && health.IsHealthy {
    // Dispatcher is healthy
}

// Get overall health statistics
healthyCount := monitoring.GetHealthyDispatcherCount()
staleCount := monitoring.GetStaleDispatcherCount()
```

## Testing

Comprehensive tests have been added:

- **Unit Tests**: `engine/infra/monitoring/dispatcher_test.go`
- **Integration Tests**: `engine/infra/monitoring/interceptor/dispatcher_metrics_test.go`

Tests cover:

- Health status tracking and transitions
- Heartbeat updates and stale detection
- Lifecycle event recording
- Error handling and graceful degradation

## Future Enhancements

Potential future improvements:

1. **Alerting Rules**: Prometheus alerting rules for dispatcher failures
2. **SLA Tracking**: Uptime percentage calculations
3. **Performance Metrics**: Dispatcher processing rates and latencies
4. **Auto-scaling**: Dispatcher scaling based on health and load metrics
5. **Distributed Tracing**: OpenTelemetry tracing integration

## Troubleshooting

### Common Issues

1. **Missing Metrics**: Ensure monitoring is enabled in project configuration
2. **Stale Dispatchers**: Check heartbeat activity frequency and Redis connectivity
3. **Dashboard Not Updating**: Verify Prometheus scraping and metric exposure

### Debug Commands

```bash
# Check metric endpoint
curl http://localhost:8080/metrics | grep dispatcher

# Verify monitoring service status
# (Check application logs for monitoring initialization messages)
```

## Files Modified

### Core Implementation

- `engine/infra/monitoring/interceptor/temporal.go` - Added dispatcher metrics
- `engine/infra/monitoring/dispatcher.go` - New dispatcher health monitoring module
- `engine/infra/monitoring/monitoring.go` - Integration with main monitoring service
- `engine/worker/mod.go` - Worker lifecycle integration
- `engine/worker/activities/dispatcher_heartbeat.go` - Heartbeat metric recording

### Testing

- `engine/infra/monitoring/dispatcher_test.go` - Health monitoring tests
- `engine/infra/monitoring/interceptor/dispatcher_metrics_test.go` - Metric recording tests

### Visualization

- `cluster/grafana/dashboards/compozy-monitoring.json` - Enhanced dashboard with dispatcher panels

This implementation provides comprehensive monitoring capabilities for dispatcher health and lifecycle management, following the established patterns and architecture of the Compozy monitoring system.
