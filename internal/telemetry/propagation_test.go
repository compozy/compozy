package telemetry_test

import (
	"context"
	"testing"

	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const testTraceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func TestPropagatorCarriesOnlyTraceParent(t *testing.T) {
	t.Parallel()

	t.Run("Should extract the traceparent and ignore the incoming tracestate", func(t *testing.T) {
		t.Parallel()

		carrier := propagation.MapCarrier{"traceparent": testTraceParent, "tracestate": "vendor=secret"}
		sc := trace.SpanContextFromContext(telemetry.Propagator().Extract(context.Background(), carrier))
		if !sc.IsValid() || !sc.IsRemote() || sc.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Fatalf("extracted span context = %+v, want the remote traceparent context", sc)
		}
		if sc.TraceState().Len() != 0 {
			t.Fatalf("extracted tracestate = %q, want ignored", sc.TraceState().String())
		}
	})

	t.Run("Should inject the traceparent without a tracestate", func(t *testing.T) {
		t.Parallel()

		state, err := trace.ParseTraceState("vendor=secret")
		if err != nil {
			t.Fatalf("ParseTraceState() error = %v", err)
		}
		sc := trace.SpanContextFromContext(
			telemetry.Propagator().
				Extract(context.Background(), propagation.MapCarrier{"traceparent": testTraceParent}),
		).WithTraceState(state)
		carrier := propagation.MapCarrier{}
		telemetry.Propagator().Inject(trace.ContextWithSpanContext(context.Background(), sc), carrier)
		if got := carrier.Get("traceparent"); got != testTraceParent {
			t.Fatalf("injected traceparent = %q, want %q", got, testTraceParent)
		}
		if got := carrier.Keys(); len(got) != 1 {
			t.Fatalf("injected keys = %v, want only traceparent", got)
		}
		if fields := telemetry.Propagator().Fields(); len(fields) != 1 || fields[0] != "traceparent" {
			t.Fatalf("Fields() = %v, want [traceparent]", fields)
		}
	})
}

func TestTraceParentFormatsOnlyRecordedSpans(t *testing.T) {
	t.Parallel()

	t.Run("Should format the traceparent of a recording span", func(t *testing.T) {
		t.Parallel()

		provider := sdktrace.NewTracerProvider()
		t.Cleanup(func() {
			if err := provider.Shutdown(context.Background()); err != nil {
				t.Errorf("TracerProvider.Shutdown() error = %v", err)
			}
		})
		ctx, span := provider.Tracer(telemetry.ScopeACP).Start(context.Background(), telemetry.SpanSessionTurn)
		defer span.End()
		sc := span.SpanContext()
		want := "00-" + sc.TraceID().String() + "-" + sc.SpanID().String() + "-01"
		if got := telemetry.TraceParent(ctx); got != want {
			t.Fatalf("TraceParent() = %q, want %q", got, want)
		}
	})

	t.Run("Should not propagate a remote parent the daemon does not record", func(t *testing.T) {
		t.Parallel()

		ctx := telemetry.Propagator().
			Extract(context.Background(), propagation.MapCarrier{"traceparent": testTraceParent})
		if got := telemetry.TraceParent(ctx); got != "" {
			t.Fatalf("TraceParent() = %q, want empty for a span the daemon does not record", got)
		}
	})
}
