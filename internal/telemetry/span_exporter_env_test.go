package telemetry_test

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"testing"

	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// UT-022
// not parallel: t.Setenv injects OTEL_RESOURCE_ATTRIBUTES into the hostile input resource.
func TestSanitizingSpanExporterRebuildsSpanFields(t *testing.T) {
	t.Run("Should export only registered span fields from a hostile span", func(t *testing.T) {
		t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "team=x,service.name=leak")
		hostileResource, err := resource.New(context.Background(), resource.WithFromEnv())
		if err != nil {
			t.Fatalf("resource.New(WithFromEnv) error = %v", err)
		}
		provider, exported := newTracerProvider(t, testRegistries(nil), sdktrace.WithResource(hostileResource))
		parent, linked := hostileSpanContexts(t)

		_, span := provider.Tracer(telemetry.ScopeSession).Start(
			trace.ContextWithRemoteSpanContext(context.Background(), parent),
			"prompt: hello",
			trace.WithAttributes(attribute.String("prompt.text", "hello"), telemetry.AttrAgent.String("../../etc")),
			trace.WithLinks(trace.Link{SpanContext: linked, Attributes: []attribute.KeyValue{
				attribute.String("prompt.text", "hello"),
			}}),
		)
		span.RecordError(errors.New("open /home/ana/secret.txt: permission denied"))
		span.AddEvent(telemetry.EventFirstToken, trace.WithAttributes(attribute.String("chunk", "hello")))
		span.End()

		got := onlySpan(t, exported)
		if got.Name != telemetry.UnknownName {
			t.Fatalf("span name = %q, want %q", got.Name, telemetry.UnknownName)
		}
		wantAttrs := []attribute.KeyValue{telemetry.AttrAgent.String(telemetry.ValueOther)}
		if !slices.Equal(got.Attributes, wantAttrs) {
			t.Fatalf("span attributes = %v, want %v", got.Attributes, wantAttrs)
		}
		if len(got.Events) != 1 || got.Events[0].Name != telemetry.EventFirstToken ||
			len(got.Events[0].Attributes) != 0 {
			t.Fatalf("span events = %+v, want only %q without attributes", got.Events, telemetry.EventFirstToken)
		}
		if len(got.Links) != 1 || len(got.Links[0].Attributes) != 0 ||
			got.Links[0].SpanContext.SpanID() != linked.SpanID() || got.Links[0].SpanContext.TraceState().Len() != 0 {
			t.Fatalf("span links = %+v, want the linked context without attributes or trace state", got.Links)
		}
		if got.SpanContext.TraceState().Len() != 0 || got.Parent.TraceState().Len() != 0 {
			t.Fatalf("trace state = %q / parent %q, want both empty",
				got.SpanContext.TraceState().String(), got.Parent.TraceState().String())
		}
		if got.Parent.SpanID() != parent.SpanID() || got.SpanContext.TraceID() != parent.TraceID() {
			t.Fatalf("span context = %v parent %v, want the remote parent kept", got.SpanContext, got.Parent)
		}
		assertBuiltInResource(t, got.Resource)
	})
}

// hostileSpanContexts returns a remote parent and a link target that both carry vendor trace state.
func hostileSpanContexts(t *testing.T) (trace.SpanContext, trace.SpanContext) {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("TraceIDFromHex() error = %v", err)
	}
	vendorState, err := trace.ParseTraceState("vendor=x")
	if err != nil {
		t.Fatalf("ParseTraceState() error = %v", err)
	}
	spanContext := func(spanID string, remote bool) trace.SpanContext {
		t.Helper()
		id, err := trace.SpanIDFromHex(spanID)
		if err != nil {
			t.Fatalf("SpanIDFromHex(%q) error = %v", spanID, err)
		}
		return trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID, SpanID: id, TraceFlags: trace.FlagsSampled, TraceState: vendorState, Remote: remote,
		})
	}
	return spanContext("00f067aa0ba902b7", true), spanContext("0102030405060708", false)
}

func assertBuiltInResource(t *testing.T, res *resource.Resource) {
	t.Helper()
	keys := make([]string, 0, res.Len())
	for _, kv := range res.Attributes() {
		keys = append(keys, string(kv.Key))
	}
	want := []string{"host.arch", "os.type", "service.instance.id", "service.name", "service.version"}
	if !slices.Equal(keys, want) {
		t.Fatalf("resource keys = %v, want %v", keys, want)
	}
	wantValues := map[attribute.Key]string{
		"service.name":        "compozy-daemon",
		"service.version":     testServiceVersion,
		"service.instance.id": testInstanceID,
		"os.type":             runtime.GOOS,
		"host.arch":           runtime.GOARCH,
	}
	for key, want := range wantValues {
		if got, ok := res.Set().Value(key); !ok || got.AsString() != want {
			t.Fatalf("%s = %q (present %t), want %q", key, got.AsString(), ok, want)
		}
	}
}
