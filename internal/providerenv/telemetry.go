package providerenv

import (
	"net/url"
	"slices"
	"strings"
)

// ApplyTelemetryForwarding makes telemetry forwarding opt-in: it first removes every OTEL_* and TRACEPARENT variable
// the daemon inherited, so a provider never exports through the daemon's own environment, then sets the OTLP export
// variables the daemon forwards and the session's OTEL_RESOURCE_ATTRIBUTES. forward holds the telemetry runtime's
// KEY=value pairs; empty means forwarding is off or export is inactive. Callers apply agent-defined variables
// afterwards so those win.
func ApplyTelemetryForwarding(env []string, forward []string, sessionID string, agent string) []string {
	env = slices.DeleteFunc(env, func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.HasPrefix(key, "OTEL_") || key == "TRACEPARENT"
	})
	if len(forward) == 0 {
		return env
	}
	for _, entry := range forward {
		key, value, _ := strings.Cut(entry, "=")
		env = SetEnvValue(env, key, value)
	}
	return SetEnvValue(env, "OTEL_RESOURCE_ATTRIBUTES",
		"compozy.session_id="+url.PathEscape(sessionID)+",compozy.agent="+url.PathEscape(agent))
}

// ApplyTraceParent sets TRACEPARENT to the provider spawn span's W3C traceparent. An empty traceParent (the span is
// not recording) leaves env unchanged.
func ApplyTraceParent(env []string, traceParent string) []string {
	if traceParent == "" {
		return env
	}
	return SetEnvValue(env, "TRACEPARENT", traceParent)
}
