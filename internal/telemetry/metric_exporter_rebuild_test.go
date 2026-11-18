package telemetry_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// handBuiltMetrics wraps metrics in a collection under one registered scope, as a hostile or hand-made
// input the real SDK would not produce.
func handBuiltMetrics(metrics ...metricdata.Metrics) *metricdata.ResourceMetrics {
	return &metricdata.ResourceMetrics{ScopeMetrics: []metricdata.ScopeMetrics{{
		Scope:   instrumentation.Scope{Name: telemetry.ScopeObserve},
		Metrics: metrics,
	}}}
}

func permissionCounter() metricdata.Metrics {
	return metricdata.Metrics{Name: telemetry.MetricPermissionRequests, Data: metricdata.Sum[int64]{
		Temporality: metricdata.CumulativeTemporality, IsMonotonic: true,
		DataPoints: []metricdata.DataPoint[int64]{{Value: 1}},
	}}
}

// Resource and scope rows of the sanitation table, on the metric side (UT-022 and UT-039 cover spans).
func TestSanitizingMetricExporterRebuildsResourceAndScope(t *testing.T) {
	t.Parallel()

	t.Run("Should replace a hostile resource with the built-in resource", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		input := handBuiltMetrics(permissionCounter())
		input.Resource = resource.NewWithAttributes("https://evil.example/schema",
			attribute.String("service.name", "leak"), attribute.String("team", "x"))
		if err := harness.exporter.Export(context.Background(), input); err != nil {
			t.Fatalf("Export() error = %v", err)
		}

		exported := harness.recorded.lastExport().Resource
		assertBuiltInResource(t, exported)
		if version, _ := exported.Set().Value("service.version"); version.AsString() != testServiceVersion {
			t.Fatalf("service.version = %q, want %q", version.AsString(), testServiceVersion)
		}
		if exported.SchemaURL() != "" {
			t.Fatalf("resource schema URL = %q, want none", exported.SchemaURL())
		}
	})

	scopes := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "Should export an unregistered scope as compozy.unknown",
			in:   "example.com/unregistered",
			want: telemetry.UnknownName,
		},
		{name: "Should keep a registered scope name", in: telemetry.ScopeStore, want: telemetry.ScopeStore},
	}
	for _, tc := range scopes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			harness := newMetricHarness(t, testRegistries(nil))
			input := handBuiltMetrics(permissionCounter())
			input.ScopeMetrics[0].Scope = instrumentation.Scope{
				Name: tc.in, Version: "9.9.9", SchemaURL: "https://evil.example/schema",
				Attributes: attribute.NewSet(attribute.String("leak", "x")),
			}
			if err := harness.exporter.Export(context.Background(), input); err != nil {
				t.Fatalf("Export() error = %v", err)
			}

			scope := harness.recorded.lastExport().ScopeMetrics[0].Scope
			if scope.Name != tc.want || scope.Version != testServiceVersion || scope.SchemaURL != "" ||
				scope.Attributes.Len() != 0 {
				t.Fatalf("scope = %+v, want %q at version %q without schema URL or attributes",
					scope, tc.want, testServiceVersion)
			}
		})
	}

	t.Run("Should export an empty built-in-resource collection for a nil collection", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		if err := harness.exporter.Export(context.Background(), nil); err != nil {
			t.Fatalf("Export(nil) error = %v", err)
		}

		exported := harness.recorded.lastExport()
		if len(exported.ScopeMetrics) != 0 {
			t.Fatalf("scope metrics = %+v, want none", exported.ScopeMetrics)
		}
		assertBuiltInResource(t, exported.Resource)
	})
}

// The SDK view filter bounds series in memory; the wrapper alone would only drop dimensions at export time.
func TestMeterProviderOptionsAggregateAwayAttributesOutsideTheInstrument(t *testing.T) {
	t.Parallel()

	t.Run("Should aggregate counter series that differ only by an unregistered dimension", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		tokens, err := harness.provider.Meter(telemetry.ScopeObserve).Int64Counter(telemetry.MetricTokens)
		if err != nil {
			t.Fatalf("Int64Counter() error = %v", err)
		}
		for _, session := range []string{"sess-0123456789abcdef", "sess-fedcba9876543210"} {
			tokens.Add(context.Background(), 5, metric.WithAttributes(
				telemetry.AttrSessionID.String(session),
				telemetry.AttrTokenType.String(telemetry.TokenInput),
				attribute.String("prompt.text", session),
			))
		}

		collected, _ := harness.collect(t)
		sum, ok := findMetric(t, &collected, telemetry.MetricTokens).Data.(metricdata.Sum[int64])
		if !ok || len(sum.DataPoints) != 1 {
			t.Fatalf("raw compozy.tokens data = %#v, want one int64 sum point", sum)
		}
		want := attribute.NewSet(telemetry.AttrTokenType.String(telemetry.TokenInput))
		if got := sum.DataPoints[0].Attributes; !got.Equals(&want) || sum.DataPoints[0].Value != 10 {
			t.Fatalf(
				"raw point = %v value %d, want %v value 10",
				got.ToSlice(),
				sum.DataPoints[0].Value,
				want.ToSlice(),
			)
		}
	})

	t.Run("Should aggregate gauge series to one attribute-less point", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		active, err := harness.provider.Meter(telemetry.ScopeObserve).Int64Gauge(telemetry.MetricSessionActive)
		if err != nil {
			t.Fatalf("Int64Gauge() error = %v", err)
		}
		active.Record(
			context.Background(),
			3,
			metric.WithAttributes(telemetry.AttrSessionID.String("sess-0123456789abcdef")),
		)
		active.Record(
			context.Background(),
			4,
			metric.WithAttributes(telemetry.AttrSessionID.String("sess-fedcba9876543210")),
		)

		collected, _ := harness.collect(t)
		gauge, ok := findMetric(t, &collected, telemetry.MetricSessionActive).Data.(metricdata.Gauge[int64])
		if !ok || len(gauge.DataPoints) != 1 {
			t.Fatalf("raw compozy.session.active data = %#v, want one int64 gauge point", gauge)
		}
		if point := gauge.DataPoints[0]; point.Attributes.Len() != 0 || point.Value != 4 {
			t.Fatalf("raw point = %v value %d, want no attributes and value 4", point.Attributes.ToSlice(), point.Value)
		}
	})
}

// stubMetricExporter is the wrapped exporter: it records what reaches it and answers with preset values.
type stubMetricExporter struct {
	mu          sync.Mutex
	exported    *metricdata.ResourceMetrics
	temporality metricdata.Temporality
	histogram   sdkmetric.Aggregation
	err         error
	ctxValue    any
	flushes     int
	shutdowns   int
}

type stubContextKey struct{}

var _ sdkmetric.Exporter = (*stubMetricExporter)(nil)

func (e *stubMetricExporter) Temporality(sdkmetric.InstrumentKind) metricdata.Temporality {
	return e.temporality
}

func (e *stubMetricExporter) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	if kind == sdkmetric.InstrumentKindHistogram && e.histogram != nil {
		return e.histogram
	}
	return sdkmetric.DefaultAggregationSelector(kind)
}

func (e *stubMetricExporter) Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exported, e.ctxValue = metrics, ctx.Value(stubContextKey{})
	return e.err
}

func (e *stubMetricExporter) ForceFlush(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.flushes, e.ctxValue = e.flushes+1, ctx.Value(stubContextKey{})
	return e.err
}

func (e *stubMetricExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdowns, e.ctxValue = e.shutdowns+1, ctx.Value(stubContextKey{})
	return e.err
}

func (e *stubMetricExporter) lastExport() *metricdata.ResourceMetrics {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exported
}

// An environment-selected aggregation (OTEL_EXPORTER_OTLP_METRICS_DEFAULT_HISTOGRAM_AGGREGATION) must never
// reach the SDK: the wrapper drops what it cannot rebuild, so exponential histograms would vanish silently.
func TestSanitizingMetricExporterPinsDefaultAggregation(t *testing.T) {
	t.Parallel()

	t.Run("Should keep histograms when the wrapped exporter asks for exponential buckets", func(t *testing.T) {
		t.Parallel()

		next := &stubMetricExporter{
			temporality: metricdata.CumulativeTemporality,
			histogram:   sdkmetric.AggregationBase2ExponentialHistogram{MaxSize: 160, MaxScale: 20},
		}
		res := newTestResource()
		wrapper := telemetry.NewSanitizingMetricExporter(
			next,
			telemetry.NewSanitizer(testRegistries(nil), res, testServiceVersion),
		)
		reader := sdkmetric.NewPeriodicReader(wrapper, sdkmetric.WithInterval(time.Hour))
		provider := sdkmetric.NewMeterProvider(
			append(telemetry.MeterProviderOptions(res), sdkmetric.WithReader(reader))...)
		t.Cleanup(func() {
			if err := provider.Shutdown(context.Background()); err != nil {
				t.Errorf("MeterProvider.Shutdown() error = %v", err)
			}
		})
		turns, err := provider.Meter(telemetry.ScopeSession).Float64Histogram(telemetry.MetricSessionTurnDuration)
		if err != nil {
			t.Fatalf("Float64Histogram() error = %v", err)
		}
		turns.Record(
			context.Background(),
			1.5,
			metric.WithAttributes(telemetry.AttrOutcome.String(telemetry.OutcomeOK)),
		)
		if err := provider.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush() error = %v", err)
		}

		exported := next.lastExport()
		if exported == nil || len(exported.ScopeMetrics) != 1 {
			t.Fatalf("exported collection = %+v, want the turn duration histogram", exported)
		}
		histogram, ok := findMetric(t, exported, telemetry.MetricSessionTurnDuration).Data.(metricdata.Histogram[float64])
		if !ok || len(histogram.DataPoints) != 1 || histogram.DataPoints[0].Count != 1 {
			t.Fatalf("turn duration data = %#v, want one explicit-bucket point with count 1", histogram)
		}
	})

	t.Run("Should delegate temporality to the wrapped exporter", func(t *testing.T) {
		t.Parallel()

		next := &stubMetricExporter{temporality: metricdata.DeltaTemporality}
		wrapper := telemetry.NewSanitizingMetricExporter(next,
			telemetry.NewSanitizer(testRegistries(nil), newTestResource(), testServiceVersion))
		if got := wrapper.Temporality(sdkmetric.InstrumentKindCounter); got != metricdata.DeltaTemporality {
			t.Fatalf("Temporality() = %v, want %v", got, metricdata.DeltaTemporality)
		}
	})
}

// Points whose sanitized dimensions collide are merged, so token and cost totals survive the collapse of
// distinct unregistered or configuration-only values into one exported value.
func TestSanitizingMetricExporterMergesCollidingPoints(t *testing.T) {
	t.Parallel()

	t.Run("Should sum integer counter points that collapse to one exported series", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		tokens, err := harness.provider.Meter(telemetry.ScopeObserve).Int64Counter(telemetry.MetricTokens)
		if err != nil {
			t.Fatalf("Int64Counter() error = %v", err)
		}
		for model, value := range map[string]int64{"unregistered-a": 3, "unregistered-b": 4, testCatalogModel: 5} {
			tokens.Add(context.Background(), value, metric.WithAttributes(
				telemetry.AttrGenAISystem.String(testProvider), telemetry.AttrGenAIModel.String(model),
				telemetry.AttrTokenType.String(telemetry.TokenInput),
			))
		}

		_, exported := harness.collect(t)
		sum, ok := findMetric(t, exported, telemetry.MetricTokens).Data.(metricdata.Sum[int64])
		if !ok {
			t.Fatalf("compozy.tokens data = %#v, want an int64 sum", sum)
		}
		got := make(map[string]int64, len(sum.DataPoints))
		for _, point := range sum.DataPoints {
			model, _ := point.Attributes.Value(telemetry.AttrGenAIModel)
			got[model.AsString()] += point.Value
		}
		want := map[string]int64{telemetry.ValueOther: 7, testCatalogModel: 5}
		if len(sum.DataPoints) != len(want) || !maps.Equal(got, want) {
			t.Fatalf("token totals = %v over %d points, want %v with one point per exported model",
				got, len(sum.DataPoints), want)
		}
	})

	t.Run("Should sum float counter points that collapse to one exported series", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		cost, err := harness.provider.Meter(telemetry.ScopeObserve).Float64Counter(telemetry.MetricCost)
		if err != nil {
			t.Fatalf("Float64Counter() error = %v", err)
		}
		for model, value := range map[string]float64{"unregistered-a": 0.25, "unregistered-b": 0.5} {
			cost.Add(context.Background(), value, metric.WithAttributes(
				telemetry.AttrGenAISystem.String(testProvider), telemetry.AttrGenAIModel.String(model),
				telemetry.AttrCostStatus.String(telemetry.CostActual),
			))
		}

		_, exported := harness.collect(t)
		sum, ok := findMetric(t, exported, telemetry.MetricCost).Data.(metricdata.Sum[float64])
		if !ok || len(sum.DataPoints) != 1 || sum.DataPoints[0].Value != 0.75 {
			t.Fatalf("compozy.cost data = %#v, want one point with value 0.75", sum)
		}
	})

	t.Run("Should keep the later gauge point when it arrives last", func(t *testing.T) {
		t.Parallel()

		assertGaugeKeepsLatest(t, false)
	})

	t.Run("Should keep the later gauge point when it arrives first", func(t *testing.T) {
		t.Parallel()

		assertGaugeKeepsLatest(t, true)
	})

	t.Run("Should merge collapsed histogram points into one distribution", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		start, end := time.Unix(1_700_000_000, 0), time.Unix(1_700_000_060, 0)
		// Three unregistered models collapse to "other"; the second brings the largest value and the earliest
		// start, the third the smallest value and the latest end, so every merge branch is taken.
		input := handBuiltMetrics(
			metricdata.Metrics{Name: telemetry.MetricSessionTTFT, Data: metricdata.Histogram[float64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[float64]{
					collidingHistogramPoint("unregistered-a", start, end, 2, 3, []uint64{1, 1}, 1, 2),
					collidingHistogramPoint(
						"unregistered-b", start.Add(-time.Second), end, 1, 5, []uint64{0, 1}, 5, 5),
					collidingHistogramPoint(
						"unregistered-c", start, end.Add(time.Second), 1, 0.5, []uint64{1, 0}, 0.5, 0.5),
				},
			}},
		)
		if err := harness.exporter.Export(context.Background(), input); err != nil {
			t.Fatalf("Export() error = %v", err)
		}

		histogram, ok := findMetric(t, harness.recorded.lastExport(), telemetry.MetricSessionTTFT).
			Data.(metricdata.Histogram[float64])
		if !ok || len(histogram.DataPoints) != 1 {
			t.Fatalf("compozy.session.ttft data = %#v, want one merged histogram point", histogram)
		}
		got := histogram.DataPoints[0]
		low, _ := got.Min.Value()
		high, _ := got.Max.Value()
		if got.Count != 4 || got.Sum != 8.5 || !slices.Equal(got.BucketCounts, []uint64{2, 2}) || low != 0.5 ||
			high != 5 {
			t.Fatalf("merged point = count %d sum %v buckets %v min %v max %v, want 4, 8.5, [2 2], 0.5, 5",
				got.Count, got.Sum, got.BucketCounts, low, high)
		}
		if !got.StartTime.Equal(start.Add(-time.Second)) || !got.Time.Equal(end.Add(time.Second)) {
			t.Fatalf("merged window = %v..%v, want the earliest start and latest end", got.StartTime, got.Time)
		}
	})
}

// collidingHistogramPoint builds a one-bound histogram point for model; every unregistered model exports as
// "other", so points built for different unregistered models collide.
func collidingHistogramPoint(
	model string,
	start, end time.Time,
	count uint64,
	sum float64,
	buckets []uint64,
	low, high float64,
) metricdata.HistogramDataPoint[float64] {
	return metricdata.HistogramDataPoint[float64]{
		Attributes: attribute.NewSet(
			telemetry.AttrGenAISystem.String(testProvider), telemetry.AttrGenAIModel.String(model)),
		StartTime: start, Time: end, Count: count, Sum: sum, Bounds: []float64{1}, BucketCounts: buckets,
		Min: metricdata.NewExtrema(low), Max: metricdata.NewExtrema(high),
	}
}

// An aggregation the wrapper cannot rebuild is dropped, never passed through: its attributes and exemplars
// would otherwise reach the exporter without passing the allowlists.
func TestSanitizingMetricExporterDropsAggregationsItCannotRebuild(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	leak := attribute.NewSet(attribute.String("prompt.text", "SECRET-CANARY-7f3a"))
	cases := []struct {
		name string
		data metricdata.Aggregation
	}{
		{
			name: "Should drop an exponential histogram",
			data: metricdata.ExponentialHistogram[float64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.ExponentialHistogramDataPoint[float64]{
					{Attributes: leak, Time: now, Count: 1},
				},
			},
		},
		{
			name: "Should drop a summary",
			data: metricdata.Summary{
				DataPoints: []metricdata.SummaryDataPoint{{Attributes: leak, Time: now, Count: 1}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			harness := newMetricHarness(t, testRegistries(nil))
			input := handBuiltMetrics(
				metricdata.Metrics{Name: telemetry.MetricSessionTurnDuration, Data: tc.data},
				permissionCounter(),
			)
			if err := harness.exporter.Export(context.Background(), input); err != nil {
				t.Fatalf("Export() error = %v", err)
			}

			exported := harness.recorded.lastExport()
			if len(exported.ScopeMetrics) != 1 || len(exported.ScopeMetrics[0].Metrics) != 1 ||
				exported.ScopeMetrics[0].Metrics[0].Name != telemetry.MetricPermissionRequests {
				t.Fatalf("exported = %+v, want only the rebuildable permission counter", exported.ScopeMetrics)
			}
		})
	}
}

func assertGaugeKeepsLatest(t *testing.T, laterFirst bool) {
	t.Helper()
	harness := newMetricHarness(t, testRegistries(nil))
	now := time.Unix(1_700_000_000, 0)
	earlier := metricdata.DataPoint[int64]{
		Attributes: attribute.NewSet(attribute.String("series", "a")), Time: now, Value: 1,
	}
	later := metricdata.DataPoint[int64]{
		Attributes: attribute.NewSet(attribute.String("series", "b")), Time: now.Add(time.Second), Value: 7,
	}
	points := []metricdata.DataPoint[int64]{earlier, later}
	if laterFirst {
		points = []metricdata.DataPoint[int64]{later, earlier}
	}
	input := handBuiltMetrics(metricdata.Metrics{
		Name: telemetry.MetricSessionActive, Data: metricdata.Gauge[int64]{DataPoints: points},
	})
	if err := harness.exporter.Export(context.Background(), input); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	gauge, ok := findMetric(t, harness.recorded.lastExport(), telemetry.MetricSessionActive).Data.(metricdata.Gauge[int64])
	if !ok || len(gauge.DataPoints) != 1 || gauge.DataPoints[0].Value != 7 {
		t.Fatalf("compozy.session.active data = %#v, want one point with the later value 7", gauge)
	}
}

// stubSpanExporter is the wrapped trace exporter.
type stubSpanExporter struct {
	mu        sync.Mutex
	err       error
	ctxValue  any
	shutdowns int
	spans     int
}

var _ sdktrace.SpanExporter = (*stubSpanExporter)(nil)

func (e *stubSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans, e.ctxValue = e.spans+len(spans), ctx.Value(stubContextKey{})
	return e.err
}

func (e *stubSpanExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdowns, e.ctxValue = e.shutdowns+1, ctx.Value(stubContextKey{})
	return e.err
}

// The wrappers add no behavior to the wrapped exporter's lifecycle: its errors, its context, and its
// shutdown all reach the caller (status classification and the bounded flush depend on them).
func TestSanitizingExportersDelegateLifecycleAndErrors(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("collector unavailable")
	ctx := context.WithValue(context.Background(), stubContextKey{}, "caller")
	sanitizer := telemetry.NewSanitizer(testRegistries(nil), newTestResource(), testServiceVersion)

	t.Run("Should return the wrapped metric exporter's errors and context", func(t *testing.T) {
		t.Parallel()

		next := &stubMetricExporter{err: sentinel}
		wrapper := telemetry.NewSanitizingMetricExporter(next, sanitizer)
		for name, call := range map[string]func() error{
			"Export":     func() error { return wrapper.Export(ctx, handBuiltMetrics(permissionCounter())) },
			"ForceFlush": func() error { return wrapper.ForceFlush(ctx) },
			"Shutdown":   func() error { return wrapper.Shutdown(ctx) },
		} {
			if err := call(); !errors.Is(err, sentinel) {
				t.Fatalf("%s() error = %v, want the wrapped exporter's error", name, err)
			}
			if next.ctxValue != "caller" {
				t.Fatalf("%s() context value = %v, want the caller's context", name, next.ctxValue)
			}
		}
		if next.flushes != 1 || next.shutdowns != 1 {
			t.Fatalf("wrapped flushes = %d shutdowns = %d, want one each", next.flushes, next.shutdowns)
		}
	})

	t.Run("Should return the wrapped span exporter's errors and context", func(t *testing.T) {
		t.Parallel()

		next := &stubSpanExporter{err: sentinel}
		wrapper := telemetry.NewSanitizingSpanExporter(next, sanitizer)
		spans := tracetest.SpanStubs{{Name: telemetry.SpanToolExecute}}.Snapshots()
		for name, call := range map[string]func() error{
			"ExportSpans": func() error { return wrapper.ExportSpans(ctx, spans) },
			"Shutdown":    func() error { return wrapper.Shutdown(ctx) },
		} {
			if err := call(); !errors.Is(err, sentinel) {
				t.Fatalf("%s() error = %v, want the wrapped exporter's error", name, err)
			}
			if next.ctxValue != "caller" {
				t.Fatalf("%s() context value = %v, want the caller's context", name, next.ctxValue)
			}
		}
		if next.spans != 1 || next.shutdowns != 1 {
			t.Fatalf("wrapped spans = %d shutdowns = %d, want one each", next.spans, next.shutdowns)
		}
	})
}
