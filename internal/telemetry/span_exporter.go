package telemetry

import (
	"context"
	"runtime"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// newResource builds the only resource the exporters send. Environment resource detectors
// (OTEL_RESOURCE_ATTRIBUTES, OTEL_SERVICE_NAME) are never consulted.
func newResource(serviceName, serviceVersion, instanceID string) *resource.Resource {
	return resource.NewSchemaless(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", serviceVersion),
		attribute.String("service.instance.id", instanceID),
		attribute.String("os.type", runtime.GOOS),
		attribute.String("host.arch", runtime.GOARCH),
	)
}

// sanitizingSpanExporter is the trace side of the privacy boundary: every span is rebuilt from
// registered fields before it reaches next.
type sanitizingSpanExporter struct {
	next      sdktrace.SpanExporter
	sanitizer *sanitizer
}

var _ sdktrace.SpanExporter = (*sanitizingSpanExporter)(nil)

func newSanitizingSpanExporter(next sdktrace.SpanExporter, s *sanitizer) *sanitizingSpanExporter {
	return &sanitizingSpanExporter{next: next, sanitizer: s}
}

func (e *sanitizingSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	res := e.sanitizer.exportResource()
	clean := make([]sdktrace.ReadOnlySpan, 0, len(spans))
	for _, span := range spans {
		clean = append(clean, e.sanitizer.span(span, res))
	}
	return e.next.ExportSpans(ctx, clean)
}

func (e *sanitizingSpanExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}

// span builds a new span from scratch (allow-by-construction): registered name and scope, sanitized
// attributes, registered events without attributes, links without attributes, no trace state, the
// export-time resource res, and a status without description.
func (s *sanitizer) span(in sdktrace.ReadOnlySpan, res *resource.Resource) sdktrace.ReadOnlySpan {
	name, keys := s.spanName(in.Name())
	return tracetest.SpanStub{
		Name:                 s.finalString(name),
		SpanContext:          withoutTraceState(in.SpanContext()),
		Parent:               withoutTraceState(in.Parent()),
		SpanKind:             in.SpanKind(),
		StartTime:            in.StartTime(),
		EndTime:              in.EndTime(),
		Attributes:           s.attributes(in.Attributes(), keys),
		Events:               s.registeredEvents(in.Events()),
		Links:                linksWithoutAttributes(in.Links()),
		Status:               sdktrace.Status{Code: in.Status().Code},
		Resource:             res,
		InstrumentationScope: s.scope(in.InstrumentationScope()),
	}.Snapshot()
}

func withoutTraceState(spanContext trace.SpanContext) trace.SpanContext {
	return spanContext.WithTraceState(trace.TraceState{})
}

func (s *sanitizer) registeredEvents(events []sdktrace.Event) []sdktrace.Event {
	var clean []sdktrace.Event
	for _, event := range events {
		if eventNames.has(event.Name) {
			clean = append(clean, sdktrace.Event{Name: s.finalString(event.Name), Time: event.Time})
		}
	}
	return clean
}

func linksWithoutAttributes(links []sdktrace.Link) []sdktrace.Link {
	if len(links) == 0 {
		return nil
	}
	clean := make([]sdktrace.Link, 0, len(links))
	for _, link := range links {
		clean = append(clean, sdktrace.Link{SpanContext: withoutTraceState(link.SpanContext)})
	}
	return clean
}
