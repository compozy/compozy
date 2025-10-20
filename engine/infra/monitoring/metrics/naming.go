package metrics

import "strings"

const MetricPrefix = "compozy_"

// MetricName returns a metric name prefixed with the standard compozy namespace.
// It normalizes the name to lowercase and replaces invalid characters with underscores
// to ensure OTel/Prometheus compatibility.
func MetricName(name string) string {
	clean := strings.TrimSpace(name)
	// Replace disallowed/separator chars with '_', then lower-case.
	clean = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '.', '-', '/', ':':
			return '_'
		default:
			return r
		}
	}, clean)
	clean = strings.ToLower(clean)
	if clean == "" {
		return MetricPrefix
	}
	if strings.HasPrefix(clean, MetricPrefix) {
		return clean
	}
	return MetricPrefix + clean
}

// MetricNameWithSubsystem returns a metric name formatted as compozy_<subsystem>_<name>.
// Both subsystem and name are normalized to lowercase with spaces replaced by underscores.
func MetricNameWithSubsystem(subsystem string, name string) string {
	// Normalize subsystem: lowercase and replace spaces with underscores
	subsystem = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(subsystem), " ", "_"))
	subsystem = strings.Trim(subsystem, "_")
	// Normalize base: lowercase and replace spaces with underscores
	base := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
	base = strings.Trim(base, "_")
	if subsystem != "" {
		if base != "" {
			base = subsystem + "_" + base
		} else {
			base = subsystem
		}
	}
	return MetricName(base)
}
