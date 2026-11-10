package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
)

// traceParentKey is the W3C trace context header, also the ACP prompt _meta key.
const traceParentKey = "traceparent"

// Propagator extracts and injects only the W3C traceparent. An incoming tracestate is ignored, so it reaches no
// sampler, span, or outgoing request.
func Propagator() propagation.TextMapPropagator {
	return traceParentPropagator{}
}

type traceParentPropagator struct{}

func (traceParentPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	propagation.TraceContext{}.Inject(ctx, traceParentCarrier{carrier})
}

func (traceParentPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return propagation.TraceContext{}.Extract(ctx, traceParentCarrier{carrier})
}

func (traceParentPropagator) Fields() []string {
	return []string{traceParentKey}
}

// traceParentCarrier shows the W3C propagator only the traceparent field.
type traceParentCarrier struct {
	propagation.TextMapCarrier
}

func (c traceParentCarrier) Get(key string) string {
	if key != traceParentKey {
		return ""
	}
	return c.TextMapCarrier.Get(key)
}

func (c traceParentCarrier) Set(key, value string) {
	if key == traceParentKey {
		c.TextMapCarrier.Set(key, value)
	}
}

// TraceParent formats the W3C traceparent of ctx's span. It is empty unless the span records, so nothing propagates
// for a trace the daemon does not export (export disabled, or the trace not sampled).
func TraceParent(ctx context.Context) string {
	if !trace.SpanFromContext(ctx).IsRecording() {
		return ""
	}
	carrier := propagation.MapCarrier{}
	Propagator().Inject(ctx, carrier)
	return carrier.Get(traceParentKey)
}

// TracerProvider returns a provider whose tracers start each span with the global provider installed at that moment.
// Instrumentation built before Init, such as the HTTP and UDS middleware, so follows the provider Init installs.
func TracerProvider() trace.TracerProvider {
	return globalTracerProvider{}
}

type globalTracerProvider struct{ embedded.TracerProvider }

func (globalTracerProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	return globalTracer{name: name, options: options}
}

type globalTracer struct {
	embedded.Tracer
	name    string
	options []trace.TracerOption
}

func (t globalTracer) Start(
	ctx context.Context,
	spanName string,
	options ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	return otel.Tracer(t.name, t.options...).Start(ctx, spanName, options...)
}
