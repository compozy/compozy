package telemetry_test

import (
	"context"
	"testing"

	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// BenchmarkSpanOverhead measures one span with three attributes, per span only:
//
//	go test ./internal/telemetry -run '^$' -bench BenchmarkSpanOverhead -benchmem -cpu 1,8
//
// SDK modes batch through the real sanitizing exporter over a discarding exporter. The batcher blocks
// instead of dropping and is flushed inside the measurement, so every recorded span's export counts.
func BenchmarkSpanOverhead(b *testing.B) {
	attrs := []attribute.KeyValue{
		telemetry.AttrTool.String(testTool),
		telemetry.AttrToolRisk.String("read"),
		telemetry.AttrOutcome.String(telemetry.OutcomeOK),
	}
	cases := []struct {
		name  string
		sdk   bool
		ratio float64
	}{
		{name: "noop"},
		{name: "sdk_100", sdk: true, ratio: 1},
		{name: "sdk_10", sdk: true, ratio: 0.1},
		{name: "sdk_0", sdk: true, ratio: 0},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			var provider trace.TracerProvider = noop.NewTracerProvider()
			flush := func() {}
			if tc.sdk {
				sdkProvider := newBenchmarkTracerProvider(b, tc.ratio)
				provider = sdkProvider
				flush = func() {
					if err := sdkProvider.ForceFlush(context.Background()); err != nil {
						b.Fatalf("ForceFlush() error = %v", err)
					}
				}
			}
			tracer := provider.Tracer(telemetry.ScopeTools)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				ctx := context.Background()
				for pb.Next() {
					_, span := tracer.Start(ctx, telemetry.SpanToolExecute, trace.WithAttributes(attrs...))
					span.End()
				}
			})
			flush()
		})
	}
}

func newBenchmarkTracerProvider(b *testing.B, ratio float64) *sdktrace.TracerProvider {
	b.Helper()
	exporter := telemetry.NewSanitizingSpanExporter(
		tracetest.NewNoopExporter(),
		telemetry.NewSanitizer(testRegistries(nil), newTestResource(), testServiceVersion),
	)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithBatcher(exporter, sdktrace.WithBlocking()),
	)
	b.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			b.Errorf("TracerProvider.Shutdown() error = %v", err)
		}
	})
	return provider
}

// BenchmarkDurationHistogramDisabled measures the duration-histogram lookup each turn, ACP request, and tool call
// pays while export is disabled (the global no-op meter provider Init installs):
//
//	go test ./internal/telemetry -run '^$' -bench BenchmarkDurationHistogramDisabled -benchmem
func BenchmarkDurationHistogramDisabled(b *testing.B) {
	otel.SetMeterProvider(metricnoop.NewMeterProvider())
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := telemetry.DurationHistogram(ctx, telemetry.ScopeACP, telemetry.MetricACPRPCDuration); ok {
			b.Fatal("DurationHistogram() recorded with export disabled")
		}
	}
}
