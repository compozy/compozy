package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		{name: "Should sanitize special characters", input: "requests.total", expected: "compozy_requests_total"},
		{name: "Should normalize case", input: "RequestsTotal", expected: "compozy_requeststotal"},
		{
			name:     "Should handle multiple special chars",
			input:    "api/v1:requests-total",
			expected: "compozy_api_v1_requests_total",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MetricName(tt.input)
			assert.Equal(t, tt.expected, got)
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
		{
			name:       "Should normalize subsystem with spaces",
			subsystem:  "api gateway",
			metricName: "requests",
			expected:   "compozy_api_gateway_requests",
		},
		{
			name:       "Should normalize mixed case subsystem",
			subsystem:  "AuthService",
			metricName: "logins",
			expected:   "compozy_authservice_logins",
		},
		{
			name:       "Should normalize subsystem and metric special chars",
			subsystem:  " API Gateway ",
			metricName: "requests.total",
			expected:   "compozy_api_gateway_requests_total",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MetricNameWithSubsystem(tt.subsystem, tt.metricName)
			assert.Equal(t, tt.expected, got)
		})
	}
}
