package monitoring_test

import monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"

var (
	buildInfoMetricName = monitoringmetrics.MetricNameWithSubsystem("system", "build_info")
	uptimeMetricName    = monitoringmetrics.MetricNameWithSubsystem("system", "uptime_seconds")
)
