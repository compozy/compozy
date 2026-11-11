package providerenv

import (
	"net/url"
	"strings"
)

// ApplyTelemetryForwarding sets the OTLP export variables the daemon forwards to a provider and the session's
// OTEL_RESOURCE_ATTRIBUTES, replacing inherited values. forward holds the telemetry runtime's KEY=value pairs; empty
// means forwarding is off or export is inactive, and env is returned unchanged. Callers apply agent-defined variables
// afterwards so those win.
func ApplyTelemetryForwarding(env []string, forward []string, sessionID string, agent string) []string {
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
