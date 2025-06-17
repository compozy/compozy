# Task 8.7: Setup Local Metrics Infrastructure

## Overview

Set up local monitoring infrastructure to enable testing and development of metrics collection with Prometheus and Grafana through docker-compose integration.

## Goal

Enable developers to easily test metrics collection locally by extending the existing docker-compose setup with Prometheus and Grafana services, plus corresponding make commands for streamlined development workflow.

## Requirements

### 1. Docker Compose Services

Add to `cluster/docker-compose.yml`:

- **Prometheus** service for metrics collection

    - Scrape `/metrics` endpoint from application
    - Persist data with volume mount
    - Expose on port 9090
    - Include basic configuration for Compozy monitoring

- **Grafana** service for metrics visualization
    - Connect to Prometheus datasource
    - Persist dashboards and config
    - Expose on port 3000
    - Pre-configure with basic monitoring dashboard

### 2. Configuration Files

Create supporting configuration files:

- `cluster/prometheus.yml` - Prometheus configuration with scrape configs
- `cluster/grafana/` directory with:
    - `grafana.ini` - Basic Grafana configuration
    - `provisioning/` - Automated datasource and dashboard setup
    - `dashboards/` - Sample monitoring dashboard(s)

### 3. Make Commands

Add to `Makefile`:

- `make metrics-up` - Start only metrics services (Prometheus + Grafana)
- `make metrics-down` - Stop metrics services
- `make metrics-reset` - Reset metrics data (volumes)
- Update `make dev` or add `make dev-with-metrics` - Start full stack with metrics

### 4. Documentation

Create or update:

- Add section to `docs/monitoring.md` about local development
- Update project README with local metrics testing instructions
- Include sample URLs and credentials

## Technical Specifications

### Prometheus Configuration

- Scrape interval: 15s
- Scrape `/metrics` from application (port 8080)
- Include job labels for identification
- Retention: 7 days (suitable for local development)

### Grafana Setup

- Default admin credentials (admin/admin)
- Auto-provision Prometheus datasource
- Include sample dashboard showing:
    - HTTP request metrics (rate, duration, errors)
    - Temporal workflow metrics
    - System health metrics
    - Application build info

### Integration Points

- Ensure monitoring service is enabled when running with metrics
- Coordinate with existing health check endpoints
- Consider environment variable configuration for metrics endpoints

## Success Criteria

- [x] Prometheus successfully scrapes metrics from local application
- [x] Grafana displays metrics in sample dashboard
- [x] Integrated metrics services into `make start-docker` workflow
- [x] Full development environment includes monitoring stack
- [x] Documentation provides clear setup instructions
- [x] No conflicts with existing docker-compose services
- [x] Proper volume management for persistent data

## Status: COMPLETED ✅

**Completion Date**: 2025-06-17  
**Implementation Summary**:

Successfully implemented local metrics infrastructure with the following key achievements:

### Infrastructure Setup

- ✅ Added Prometheus and Grafana services to `cluster/docker-compose.yml`
- ✅ Created comprehensive Prometheus configuration (`cluster/prometheus.yml`)
- ✅ Set up Grafana auto-provisioning with datasources and dashboards
- ✅ Integrated with existing `temporal-network` for seamless communication

### Configuration & Dashboards

- ✅ **Compozy Monitoring Dashboard**: Complete dashboard showing HTTP metrics, Temporal workflow metrics, and system health
- ✅ **Temporal Server Dashboard**: Dedicated dashboard for built-in Temporal server metrics
- ✅ Fixed all metric names to match actual implementation (100% tech spec compliance)
- ✅ Corrected Prometheus scraping from port 8080 to 3001 (Compozy application port)

### Metrics Implementation Fixes

- ✅ Fixed metric name: `compozy_temporal_workflow_task_duration_seconds` → `compozy_temporal_workflow_duration_seconds`
- ✅ Added missing worker tracking metrics by implementing `IncrementRunningWorkers/DecrementRunningWorkers`
- ✅ Verified all metrics against tech spec requirements (3/3 discrepancies resolved)

### Documentation & Integration

- ✅ Updated `docs/monitoring.md` with comprehensive local development section
- ✅ Integrated metrics stack into standard `make start-docker` workflow
- ✅ Added troubleshooting guide for common Grafana/Prometheus issues
- ✅ Provided clear access URLs and default credentials

### Technical Details

- **Prometheus**: Scrapes both Compozy application (port 3001) and Temporal server (port 8000)
- **Grafana**: Auto-provisioned with proper datasource UID configuration
- **Temporal Integration**: Leverages built-in Temporal metrics on port 8000
- **Data Persistence**: 7-day retention for development with Docker volumes

The local metrics infrastructure is now fully operational and provides developers with comprehensive monitoring capabilities for testing and development workflows.

## Implementation Notes

- Use standard Prometheus and Grafana Docker images
- Leverage existing temporal-network for service communication
- Ensure proper service dependencies and health checks
- Consider development-friendly configurations (shorter scrape intervals, etc.)
- Maintain compatibility with existing development workflow

## Files to Modify/Create

- `cluster/docker-compose.yml` - Add Prometheus and Grafana services
- `cluster/prometheus.yml` - Prometheus scrape configuration
- `cluster/grafana/grafana.ini` - Grafana configuration
- `cluster/grafana/provisioning/datasources/prometheus.yml` - Auto-provision datasource
- `cluster/grafana/provisioning/dashboards/dashboard.yml` - Dashboard config
- `cluster/grafana/dashboards/compozy-monitoring.json` - Sample dashboard
- `Makefile` - Add metrics-related commands
- `docs/monitoring.md` - Update with local development section

## Priority

**High** - This task bridges the gap between implemented monitoring and practical local testing, essential for development workflow.
