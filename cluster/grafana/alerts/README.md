# Compozy Alerting Rules Documentation

This directory contains Grafana alerting rules for the Compozy system. The rules are organized by system component and severity level.

## Alert Files

- `schedule-alerts.yaml` - Alerting rules for workflow scheduling system
- `memory-alerts.yaml` - Alerting rules for memory system (NEW)

## Memory System Alerts

### Health Alerts (`compozy_memory_health`)

#### MemorySystemDown (Critical)

- **Condition**: `compozy_memory_health_status == 0` for 1 minute
- **Severity**: Critical
- **Description**: The overall memory system health is down
- **Action**: Investigate memory manager status, check dependencies (Redis, Temporal)

#### MemoryInstanceUnhealthy (Warning)

- **Condition**: Individual memory instance unhealthy for 2 minutes
- **Severity**: Warning
- **Description**: A specific memory instance is reporting unhealthy status
- **Action**: Check specific memory instance logs, verify storage connectivity

#### MemoryHighTokenUsage (Warning)

- **Condition**: Token usage > 85% for 3 minutes
- **Severity**: Warning
- **Description**: Memory instance approaching token limit
- **Action**: Review memory configuration, consider increasing limits or improving flushing

#### MemoryTokenLimitReached (Critical)

- **Condition**: Token usage >= 100% for 1 minute
- **Severity**: Critical
- **Description**: Memory instance has reached token limit
- **Action**: Immediate intervention required - increase limits or trigger manual flush

### Operation Alerts (`compozy_memory_operations`)

#### MemoryOperationFailures (Warning)

- **Condition**: > 5 operation failures in 5 minutes
- **Severity**: Warning
- **Description**: High rate of memory operation failures
- **Action**: Check error logs, verify storage backend health

#### MemoryOperationLatencyHigh (Warning)

- **Condition**: 95th percentile latency > 5 seconds for 3 minutes
- **Severity**: Warning
- **Description**: Memory operations taking too long
- **Action**: Check storage backend performance, network latency

#### MemoryLockContentionHigh (Warning)

- **Condition**: > 1 lock contention per second for 5 minutes
- **Severity**: Warning
- **Description**: High contention for memory locks
- **Action**: Review concurrent access patterns, consider lock timeout adjustments

### Performance Alerts (`compozy_memory_performance`)

#### MemoryFlushFailures (Warning)

- **Condition**: > 3 flush failures in 10 minutes
- **Severity**: Warning
- **Description**: Memory flush operations frequently failing
- **Action**: Check Temporal workflow execution, review flush strategy configuration

#### MemoryTrimOperationsHigh (Warning)

- **Condition**: > 2 trim operations per second for 5 minutes
- **Severity**: Warning
- **Description**: Frequent memory trimming indicates pressure
- **Action**: Review memory limits, consider adjusting flush thresholds

#### MemoryCircuitBreakerTripped (Critical)

- **Condition**: Any circuit breaker trips in 5 minutes
- **Severity**: Critical
- **Description**: System protection mechanism activated
- **Action**: Investigate underlying cause of failures, check privacy system health

### Privacy Alerts (`compozy_memory_privacy`)

#### MemoryPrivacyExclusionsHigh (Warning)

- **Condition**: > 0.5 privacy exclusions per second for 5 minutes
- **Severity**: Warning
- **Description**: High rate of messages excluded for privacy reasons
- **Action**: Review privacy policy configuration, check for misconfigured rules

#### MemoryRedactionOperationsHigh (Info)

- **Condition**: > 10 redaction operations per second for 3 minutes
- **Severity**: Info
- **Description**: High redaction activity
- **Action**: Monitor for expected privacy patterns, verify redaction effectiveness

### Capacity Alerts (`compozy_memory_capacity`)

#### MemoryGoroutinePoolExhaustion (Warning)

- **Condition**: >= 100 active goroutines for 2 minutes
- **Severity**: Warning
- **Description**: Memory system approaching goroutine limits
- **Action**: Check for stuck operations, consider scaling limits

#### MemoryTokensSavedLow (Info)

- **Condition**: < 100 tokens saved per second for 10 minutes
- **Severity**: Info
- **Description**: Low memory optimization efficiency
- **Action**: Review flushing strategies, analyze memory usage patterns

### Temporal Integration Alerts (`compozy_memory_temporal`)

#### MemoryTemporalActivitiesStuck (Warning)

- **Condition**: Temporal activity count unchanged for 5 minutes
- **Severity**: Warning
- **Description**: Memory Temporal activities may be stuck
- **Action**: Check Temporal worker health, review workflow execution

#### MemoryConfigResolutionFailures (Warning)

- **Condition**: > 0.1 config resolution failures per second for 2 minutes
- **Severity**: Warning
- **Description**: Memory configuration resolution failing
- **Action**: Check autoload configuration registry, verify memory resource definitions

### System Health Alerts (`compozy_memory_system_health`)

#### MemoryMetricsUnavailable (Critical)

- **Condition**: Compozy metrics endpoint down for 1 minute
- **Severity**: Critical
- **Description**: No metrics available for monitoring
- **Action**: Check application health, restart if necessary

#### MemorySystemOverloaded (Critical)

- **Condition**: High message rate (>100/sec) + High latency (>2s p95) + Lock contention (>0.5/sec) for 3 minutes
- **Severity**: Critical
- **Description**: Memory system showing multiple signs of overload
- **Action**: Immediate load reduction, check resource allocation, consider scaling

## Alert Severity Levels

### Critical

- Immediate attention required
- System functionality is impacted
- May require on-call response
- Examples: System down, limits reached, circuit breakers tripped

### Warning

- Attention needed within reasonable timeframe
- Performance degraded but functional
- Should be addressed to prevent escalation
- Examples: High resource usage, elevated error rates

### Info

- Informational alerts for awareness
- No immediate action required
- Useful for trend analysis
- Examples: High but normal activity levels

## Configuration Guidelines

### Threshold Tuning

All thresholds in these rules are based on expected normal operation patterns. You may need to adjust based on your specific workload:

- **Token usage thresholds**: Adjust based on your memory configurations
- **Latency thresholds**: Tune based on your storage backend performance
- **Rate thresholds**: Scale with your expected message volumes

### Integration

These rules are designed for Grafana alerting but can be adapted for Prometheus AlertManager. Key integration points:

- Labels include `component: memory` for filtering
- Annotations provide context for alert routing
- Severity levels support different notification channels

### Maintenance

- Review alert effectiveness monthly
- Adjust thresholds based on false positive rates
- Update documentation when changing rules
- Test alert routing and notification channels

## Troubleshooting

### Common Issues

1. **High false positive rate**: Thresholds may be too sensitive for your workload
2. **Missing alerts**: Check metric collection and label matching
3. **Alert fatigue**: Consider grouping related alerts or adjusting severities

### Debugging Steps

1. Verify metrics are being collected: Check `/metrics` endpoint
2. Test alert queries in Grafana/Prometheus query interface
3. Review alert evaluation frequency vs metric collection intervals
4. Check alert routing rules and notification channel configuration

For more information, see the Compozy monitoring and observability documentation.
