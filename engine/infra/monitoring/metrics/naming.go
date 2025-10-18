package metrics

import "strings"

const MetricPrefix = "compozy_"

// MetricName returns a metric name prefixed with the standard compozy namespace.
func MetricName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return MetricPrefix
	}
	if strings.HasPrefix(clean, MetricPrefix) {
		return clean
	}
	return MetricPrefix + clean
}

// MetricNameWithSubsystem returns a metric name formatted as compozy_<subsystem>_<name>.
func MetricNameWithSubsystem(subsystem string, name string) string {
	base := strings.Trim(strings.TrimSpace(name), "_")
	if subsystem = strings.Trim(strings.TrimSpace(subsystem), "_"); subsystem != "" {
		if base != "" {
			base = subsystem + "_" + base
		} else {
			base = subsystem
		}
	}
	return MetricName(base)
}
