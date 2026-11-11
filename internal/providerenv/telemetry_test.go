package providerenv

import (
	"slices"
	"strings"
	"testing"
)

func TestTelemetryForwarding(t *testing.T) {
	t.Parallel()

	const traceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	inherited := []string{
		"PATH=/usr/bin",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://inherited:4318",
		"OTEL_EXPORTER_OTLP_HEADERS=api-key=daemon-secret",
		"TRACEPARENT=00-11111111111111111111111111111111-2222222222222222-01",
	}

	// UT-070
	t.Run("Should drop inherited OTEL_* and TRACEPARENT and add none while forwarding is off", func(t *testing.T) {
		t.Parallel()

		env := ApplyTelemetryForwarding(slices.Clone(inherited), nil, "sess-9c1e", "general")
		env = ApplyTraceParent(env, "")
		if want := []string{"PATH=/usr/bin"}; !slices.Equal(env, want) {
			t.Fatalf("env = %v, want only the non-telemetry inherited environment %v", env, want)
		}
	})

	// UT-071
	t.Run("Should forward the five variables with the resolved header and the spawn traceparent", func(t *testing.T) {
		t.Parallel()

		forward := []string{
			"OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318",
			"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
			"OTEL_EXPORTER_OTLP_HEADERS=api-key=resolved-license",
		}
		env := ApplyTelemetryForwarding(slices.Clone(inherited), forward, "sess-9c1e", "general")
		env = ApplyTraceParent(env, traceParent)

		want := map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318",
			"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
			"OTEL_EXPORTER_OTLP_HEADERS":  "api-key=resolved-license",
			"OTEL_RESOURCE_ATTRIBUTES":    "compozy.session_id=sess-9c1e,compozy.agent=general",
			"TRACEPARENT":                 traceParent,
		}
		for key, value := range want {
			if got := envEntries(env, key); !slices.Equal(got, []string{value}) {
				t.Fatalf("%s entries = %v, want exactly %q", key, got, value)
			}
		}
		if got := envEntries(env, "PATH"); !slices.Equal(got, []string{"/usr/bin"}) {
			t.Fatalf("PATH entries = %v, want the inherited value kept", got)
		}
	})
}

func envEntries(env []string, key string) []string {
	var values []string
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == key {
			values = append(values, value)
		}
	}
	return values
}
