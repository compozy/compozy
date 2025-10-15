## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>infra/monitoring</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 5.0: Observability Metrics & Alerts

## Overview

Instrument Prometheus metrics, create Grafana alerts/dashboards, and document runbooks so operators can detect ingestion gaps or anomalies in LLM usage reporting.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Register metrics (`compozy_llm_prompt_tokens_total`, `compozy_llm_completion_tokens_total`, `compozy_llm_usage_events_total`, `compozy_llm_usage_failures_total`, `compozy_llm_usage_latency_seconds`).
- Add alert definitions to `cluster/grafana/alerts/llm-usage-alerts.yaml` with thresholds described in `_techspec.md`.
- Extend Grafana dashboard to include coverage %, failure trends, and latency histograms.
- Update observability runbook with response steps and link to new dashboard.
</requirements>

## Subtasks

- [ ] 5.1 Register metrics in `infra/monitoring` and expose via collector hook
- [ ] 5.2 Create `cluster/grafana/alerts/llm-usage-alerts.yaml` with critical/warning rules
- [ ] 5.3 Update Grafana dashboard JSON/panels for usage insights
- [ ] 5.4 Document alert response/runbook updates
- [ ] 5.5 Add observability tests (metrics counters, failure threshold)

## Implementation Details

- Reference “Monitoring & Observability” section in `_techspec.md`.
- Failure alerts should trigger when failure rate >1% for 15 minutes; include additional warning rules (>3× baseline, zero usage).
- Use existing monitoring helper patterns; reuse `monitoring.ExecutionMetrics` style where applicable.

### Relevant Files

- `infra/monitoring/execution_metrics.go` (or new file)
- `infra/monitoring/llm_usage_metrics.go` (new)
- `cluster/grafana/alerts/llm-usage-alerts.yaml`
- Grafana dashboard JSON (LLM operations board)
- Observability runbook docs

### Dependent Files

- `engine/llm/usage/collector.go`
- `engine/infra/postgres/usage_repo.go`

## Deliverables

- Metrics emitted during executions and visible via Prometheus scrape
- Alert YAML committed and referenced in deployment manifests
- Grafana dashboard updated with usage panels
- Runbook updated with troubleshooting guidance

## Tests

- Observability assertions mapped from `_tests.md`:
  - [ ] Validate Prometheus registry exposes new counters/labels
  - [ ] Verify failure counter increments during simulated ingestion failure

## Success Criteria

- Metrics available in staging and triggered via smoke tests
- Alerts fire in simulated scenarios and are actionable per runbook
- Dashboard visualizes token usage trends and failure rates
