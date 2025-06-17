# Product Requirements Document (PRD): Monitoring for Compozy

## 1. Overview

Compozy currently lacks operational visibility, making it difficult to monitor service health, diagnose issues, and understand performance characteristics. This document outlines the requirements for adding Prometheus monitoring to the Compozy workflow orchestration engine.

The feature will introduce a standard `/metrics` endpoint to the existing Gin HTTP server, exposing key metrics related to service health, API performance, and Temporal workflow execution. This will provide DevOps engineers, SREs, and operations teams with the essential data needed to monitor the system effectively, ensure reliability, and integrate Compozy into standard observability stacks.

## 2. Goals

- **Provide Core Visibility:** Expose essential metrics for HTTP traffic, Temporal workflows, and basic system health to establish a baseline for operational monitoring.
- **Ensure Low Overhead:** The monitoring solution must have a negligible performance impact on the core application, targeted at <0.5% overhead with formal validation testing.
- **Enable Proactive Alerting:** The exposed metrics should be sufficient to configure basic alerts for critical conditions like high error rates, increased latency, or workflow failures.
- **Standardized Integration:** Use OpenTelemetry metrics instrumentation with the Prometheus exporter, allowing seamless integration with existing Prometheus servers and Grafana dashboards.
- **Production Ready:** Include Swagger documentation for the `/metrics` endpoint and follow all project architectural standards.

## 3. User Stories

- **As an SRE**, I want to monitor the HTTP request rate, error rate (5xx), and latency for all API endpoints, so that I can detect service degradation and potential SLO violations.
- **As a DevOps Engineer**, I want to track the number of started, completed, and failed Temporal workflows, broken down by workflow type, so that I can ensure the orchestration engine is processing tasks correctly and identify problematic workflows.
- **As an Operations Team Member**, I want to view basic system metrics like uptime and build information from the `/metrics` endpoint, so that I can quickly identify the running service version and its general health.
- **As a Developer**, I want to easily add new custom metrics for new features, so that observability can scale with the application.
- **As an API Consumer**, I want to see the `/metrics` endpoint documented in Swagger, so that I understand its purpose and response format.

## 4. Core Features

### 4.1 Prometheus Metrics Endpoint

- The system must expose a `/metrics` endpoint on the main Gin HTTP server.
- This endpoint will serve metrics in the standard Prometheus text-based format.
- The endpoint will be configured via environment variables and `compozy.yaml` project configuration.
- **Swagger Documentation:** The `/metrics` endpoint must be documented in Swagger with clear descriptions of its purpose, response format (Prometheus exposition format), and operational usage.

### 4.2 Configuration Options

The monitoring feature will support the following configuration options:

- `MONITORING_ENABLED=true|false` (environment variable): Global enable/disable
- `monitoring.enabled` (boolean, optional, default: `false`) in `compozy.yaml`: Project-level enable/disable
- `monitoring.path` (string, optional, default: `/metrics`) in `compozy.yaml`: Custom endpoint path

**Example `compozy.yaml`:**

```yaml
name: my-monitored-project
version: 0.1.0

workflows:
    - source: ./workflow.yaml

monitoring:
    enabled: true
    path: /metrics

runtime:
    permissions:
        - --allow-read
        - --allow-net
        - --allow-env
```

### 4.3 HTTP Server Metrics

- The system must collect and expose metrics for all HTTP requests handled by the Gin server.
- **Metrics to include:**
    - `compozy_http_requests_total`: A counter for incoming requests. Labeled by `method`, `path`, and `status_code`. _Note: The `path` label must use the parameterized route template (e.g., `/api/v1/users/:id`) to avoid high cardinality._
    - `compozy_http_request_duration_seconds`: A histogram of request latencies. Labeled by `method` and `path`. _Note: The `path` label must use the parameterized route template._
    - `compozy_http_requests_in_flight`: A gauge for the number of currently active requests.

### 4.4 Temporal Workflow Metrics

- The system must collect and expose metrics for Temporal workflow executions.
- **Metrics to include:**
    - `compozy_temporal_workflow_started_total`: A counter for started workflows. Labeled by `workflow_type`.
    - `compozy_temporal_workflow_completed_total`: A counter for successfully completed workflows. Labeled by `workflow_type`.
    - `compozy_temporal_workflow_failed_total`: A counter for failed workflows. Labeled by `workflow_type`.
    - `compozy_temporal_workflow_duration_seconds`: A histogram of workflow execution times. Labeled by `workflow_type`.
    - `compozy_temporal_workers_running_total`: A gauge indicating the number of currently running Temporal worker processes.
    - `compozy_temporal_workers_configured_total`: A gauge indicating the number of configured Temporal worker processes per instance.

### 4.5 System Health Metrics

- The system must expose basic information about the running service instance.
- **Metrics to include:**
    - `compozy_build_info`: A gauge with a value of 1. Labeled by `version`, `commit_hash`, and `go_version`.
    - `compozy_uptime_seconds_total`: A counter for service uptime in seconds (monotonic, resets on restart).

## 5. User Experience

- The primary user interface for this feature is the `/metrics` endpoint itself, which is designed for consumption by Prometheus scrapers, not direct human interaction.
- The endpoint will be documented in Swagger with clear descriptions for operational teams.
- Configuration will be straightforward via environment variables and project YAML.
- Documentation will be provided for DevOps/SREs explaining how to enable the endpoint, what metrics are available, and what labels are used.

## 6. Technical Architecture

### 6.1 Component Design

1. **Component Placement:** The monitoring components will reside within the `engine/infra/monitoring` package, following the established Clean Architecture principles. A new `MonitoringService` will be created to encapsulate the logic.
2. **Dependency Injection:** The `MonitoringService` will follow the mandatory constructor pattern with nil-safe configuration handling and be injected into the main Gin server component.
3. **Context Handling:** All service methods that perform I/O or long-running operations will accept `context.Context` as the first parameter per project standards.

### 6.2 Implementation Approach

- **OpenTelemetry + Prometheus Exporter:** Use the OpenTelemetry Go SDK for instrumentation, configured with the built-in Prometheus exporter.
- **Library Dependencies:**
    - `go.opentelemetry.io/otel/sdk/metric` for metrics SDK
    - `go.opentelemetry.io/otel/exporters/prometheus` to expose metrics at `/metrics`
    - `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin` for HTTP middleware integration
- **Integration Points:**
    - **Gin Middleware:** HTTP metrics collection via middleware in the request chain
    - **Temporal Interceptors:** Workflow metrics via Temporal's interceptor capabilities
    - **Shared Server:** Reuse the main Gin server instance (no separate HTTP server)

### 6.3 Testing Standards

The implementation must adhere to the project's testing requirements:

- All tests must use the `t.Run("Should...")` pattern
- Use `testify/assert` for assertions and `testify/mock` for mocking
- Include both positive and negative test cases
- Provide hermetic unit tests using `oteltest.NewMeterProvider()`

## 7. Performance Validation

- **Tooling:** Load testing using `ghz` and profiling with `pprof`
- **Environment:** AWS `t3.medium` instance (2vCPU, 4GB RAM)
- **Load Profile:** 1,000 requests per second for 5 minutes
- **Success Criteria:** 95th percentile latency increase <0.5%, CPU/memory usage within 2% of baseline
- **Ownership:** SRE team responsible for validation and sign-off

## 8. Non-Goals (Out of Scope)

- **Authentication:** The `/metrics` endpoint will not have authentication for the MVP (to be added in future iterations)
- **Alerting Rules:** This PRD does not cover the creation of Prometheus alerting rules
- **Grafana Dashboards:** Pre-built Grafana dashboards are not part of the MVP
- **Push-based Metrics:** Integration with Prometheus Pushgateway is explicitly out of scope
- **Advanced Metrics:** Detailed metrics for individual Temporal activities, database connection pools, or advanced resource usage
- **Distributed Tracing:** Only metrics collection, no tracing instrumentation

## 9. Development Roadmap

- **Timeline:** 2 weeks
- **Phase 1 (Week 1): HTTP & System Metrics**
    - [ ] Integrate OpenTelemetry SDK with Prometheus exporter
    - [ ] Create `MonitoringService` following constructor patterns
    - [ ] Implement `/metrics` endpoint with Swagger documentation
    - [ ] Implement Gin middleware for HTTP metrics with proper route templating
    - [ ] Implement system health metrics (`build_info`, `uptime_seconds_total`)
    - [ ] Add unit tests following `t.Run("Should...")` pattern
    - _Success Criteria:_ `/metrics` endpoint available with HTTP and system metrics
- **Phase 2 (Week 2): Temporal Metrics & Integration**
    - [ ] Instrument Temporal workers using interceptors
    - [ ] Implement all `compozy_temporal_*` metrics
    - [ ] Add comprehensive test suite including negative cases
    - [ ] Perform load testing and validation
    - [ ] Update project documentation
    - _Success Criteria:_ Complete metrics collection with performance validation

## 10. Success Metrics

- **Metric Availability:** All metrics defined in Core Features are present at `/metrics` endpoint
- **Performance Impact:** End-to-end overhead <0.5% under normal load (formally validated)
- **Integration Success:** `/metrics` endpoint successfully scraped by Prometheus
- **Code Quality:** Full adherence to project Go standards, testing patterns, and architectural rules
- **Documentation:** Swagger documentation complete and accurate
- **Label Compliance:** All metrics use only approved, low-cardinality labels

## 11. Security Considerations

- **Network-level Protection:** Production deployments should restrict `/metrics` access at the network level (Ingress/ALB rules)
- **No PII Exposure:** Metric labels and values must not contain personally identifiable information
- **Path Parameterization:** Ensure HTTP path labels use route templates to prevent ID leakage

## 12. Risks and Mitigations

- **Performance Overhead:** OpenTelemetry instrumentation could degrade performance
    - _Mitigation:_ Formal load testing with defined success criteria and SRE sign-off
- **High Cardinality:** Incorrect label usage could cause cardinality explosion
    - _Mitigation:_ Implement CI-based label validation and enforce allow-list
- **Dependency Conflicts:** New OTEL libraries might conflict with existing dependencies
    - _Mitigation:_ Careful dependency review and `go mod tidy` validation
- **Metric Accuracy:** Instrumentation bugs could provide incorrect data
    - _Mitigation:_ Comprehensive test suite covering edge cases and failure modes

## 13. Operational Requirements

### 13.1 Kubernetes Integration

For Prometheus Operator setups, deployments must include appropriate annotations:

```yaml
apiVersion: v1
kind: Pod
metadata:
    annotations:
        prometheus.io/scrape: "true"
        prometheus.io/path: "/metrics"
        prometheus.io/port: "8080"
```

### 13.2 Rollback Procedure

1. Set `MONITORING_ENABLED=false` in environment configuration
2. Update `monitoring.enabled: false` in `compozy.yaml`
3. Redeploy the application
4. Remove ServiceMonitor resources from Kubernetes

## 14. Open Questions

_No open questions at this time._

## 15. Appendix

### 15.1 Metric Reference Table

| Metric Name                                  | Type      | Labels                                 | Description                     |
| -------------------------------------------- | --------- | -------------------------------------- | ------------------------------- |
| `compozy_http_requests_total`                | Counter   | `method`, `path`, `status_code`        | Total HTTP requests             |
| `compozy_http_request_duration_seconds`      | Histogram | `method`, `path`                       | HTTP request latency            |
| `compozy_http_requests_in_flight`            | Gauge     | -                                      | Current active requests         |
| `compozy_temporal_workflow_started_total`    | Counter   | `workflow_type`                        | Started workflows               |
| `compozy_temporal_workflow_completed_total`  | Counter   | `workflow_type`                        | Completed workflows             |
| `compozy_temporal_workflow_failed_total`     | Counter   | `workflow_type`                        | Failed workflows                |
| `compozy_temporal_workflow_duration_seconds` | Histogram | `workflow_type`                        | Workflow execution time         |
| `compozy_temporal_workers_running_total`     | Gauge     | -                                      | Currently running workers       |
| `compozy_temporal_workers_configured_total`  | Gauge     | -                                      | Configured workers per instance |
| `compozy_build_info`                         | Gauge     | `version`, `commit_hash`, `go_version` | Build information               |
| `compozy_uptime_seconds_total`               | Counter   | -                                      | Service uptime                  |

### 15.2 References

- [OpenTelemetry Go SDK (Metrics)](https://github.com/open-telemetry/opentelemetry-go)
- [OpenTelemetry Prometheus Exporter](https://pkg.go.dev/go.opentelemetry.io/otel/exporters/prometheus)
- [Prometheus Instrumentation Best Practices](https://prometheus.io/docs/instrumenting/best_practices/)
