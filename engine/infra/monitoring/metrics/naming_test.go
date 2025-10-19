package metrics

import "testing"

func TestMetricName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "Should add prefix to unprefixed metric", input: "requests_total", expected: "compozy_requests_total"},
		{
			name:     "Should keep already prefixed metric",
			input:    "compozy_custom_metric",
			expected: "compozy_custom_metric",
		},
		{name: "Should return prefix when input is blank", input: "", expected: "compozy_"},
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
			name:       "Should include subsystem and name",
			subsystem:  "auth",
			metricName: "requests_total",
			expected:   "compozy_auth_requests_total",
		},
		{
			name:       "Should trim subsystem underscores",
			subsystem:  "_scheduler_",
			metricName: "retries_total",
			expected:   "compozy_scheduler_retries_total",
		},
		{
			name:       "Should return subsystem when name is empty",
			subsystem:  "dispatcher",
			metricName: "",
			expected:   "compozy_dispatcher",
		},
		{
			name:       "Should keep already prefixed metric",
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
