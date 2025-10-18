package metrics

import "testing"

func TestMetricName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "adds prefix", input: "requests_total", expected: "compozy_requests_total"},
		{name: "keeps prefixed", input: "compozy_custom_metric", expected: "compozy_custom_metric"},
		{name: "blank returns prefix", input: "", expected: "compozy_"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MetricName(tt.input); got != tt.expected {
				t.Fatalf("MetricName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMetricNameWithSubsystem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		subsystem  string
		metricName string
		expected   string
	}{
		{
			name:       "subsystem and name",
			subsystem:  "auth",
			metricName: "requests_total",
			expected:   "compozy_auth_requests_total",
		},
		{
			name:       "subsystem trims underscore",
			subsystem:  "_scheduler_",
			metricName: "retries_total",
			expected:   "compozy_scheduler_retries_total",
		},
		{name: "empty name", subsystem: "dispatcher", metricName: "", expected: "compozy_dispatcher"},
		{
			name:       "already prefixed",
			subsystem:  "",
			metricName: "compozy_existing_metric",
			expected:   "compozy_existing_metric",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MetricNameWithSubsystem(tt.subsystem, tt.metricName); got != tt.expected {
				t.Fatalf("MetricNameWithSubsystem(%q, %q) = %q, want %q", tt.subsystem, tt.metricName, got, tt.expected)
			}
		})
	}
}
