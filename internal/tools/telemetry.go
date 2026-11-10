package tools

import "context"

// CallSpan is the trace span of one registry call. The daemon injects the OpenTelemetry implementation because
// internal/telemetry depends on this package through internal/config.
type CallSpan interface {
	SetRisk(risk RiskClass)
	End(err error)
}

// CallTracer starts the span of one registry call under the caller's span.
type CallTracer func(ctx context.Context, id ToolID) (context.Context, CallSpan)

// WithCallTracer traces every Call as tool.execute.
func WithCallTracer(tracer CallTracer) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.callTracer = tracer
	}
}

type callSpanKey struct{}

// startCallSpan starts the injected call span and keeps it on ctx so dispatch can add the resolved risk.
func (r *RuntimeRegistry) startCallSpan(ctx context.Context, id ToolID) (context.Context, CallSpan) {
	if r == nil || r.callTracer == nil {
		return ctx, nil
	}
	ctx, span := r.callTracer(ctx, id)
	if span == nil {
		return ctx, nil
	}
	return context.WithValue(ctx, callSpanKey{}, span), span
}

func setCallSpanRisk(ctx context.Context, risk RiskClass) {
	if span, ok := ctx.Value(callSpanKey{}).(CallSpan); ok && risk != "" {
		span.SetRisk(risk)
	}
}
