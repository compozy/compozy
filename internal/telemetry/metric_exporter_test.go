package telemetry_test

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// UT-020
func TestSanitizingMetricExporterAppliesInstrumentPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should keep exactly the compozy.tokens dimensions", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		tokens, err := harness.provider.Meter(telemetry.ScopeObserve).Int64Counter(telemetry.MetricTokens)
		if err != nil {
			t.Fatalf("Int64Counter() error = %v", err)
		}
		tokens.Add(context.Background(), 340, metric.WithAttributes(
			telemetry.AttrSessionID.String("sess-0123456789abcdef"),
			telemetry.AttrGenAISystem.String(testProvider),
			telemetry.AttrGenAIModel.String(testCatalogModel),
			telemetry.AttrAgent.String(testAgent),
			telemetry.AttrTokenType.String(telemetry.TokenOutput),
		))

		_, exported := harness.collect(t)
		sum, ok := findMetric(t, exported, telemetry.MetricTokens).Data.(metricdata.Sum[int64])
		if !ok || len(sum.DataPoints) != 1 {
			t.Fatalf("compozy.tokens data = %#v, want one int64 sum point", sum)
		}
		want := attribute.NewSet(
			telemetry.AttrGenAISystem.String(testProvider),
			telemetry.AttrGenAIModel.String(testCatalogModel),
			telemetry.AttrAgent.String(testAgent),
			telemetry.AttrTokenType.String(telemetry.TokenOutput),
		)
		if got := sum.DataPoints[0].Attributes; !got.Equals(&want) || sum.DataPoints[0].Value != 340 {
			t.Fatalf("point = %v value %d, want %v value 340", got.ToSlice(), sum.DataPoints[0].Value, want.ToSlice())
		}
	})

	t.Run("Should keep compozy.db.coalesced on write durations", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		writes, err := harness.provider.Meter(telemetry.ScopeStore).Float64Histogram(telemetry.MetricDBWriteDuration)
		if err != nil {
			t.Fatalf("Float64Histogram() error = %v", err)
		}
		writes.Record(context.Background(), 0.06, metric.WithAttributes(
			telemetry.AttrDBOperation.String("sessiondb.flush"),
			telemetry.AttrDBCoalesced.Bool(true),
			telemetry.AttrOutcome.String(telemetry.OutcomeOK),
			telemetry.AttrTurnID.String("turn-0123456789abcdef"),
		))

		_, exported := harness.collect(t)
		histogram, ok := findMetric(t, exported, telemetry.MetricDBWriteDuration).Data.(metricdata.Histogram[float64])
		if !ok || len(histogram.DataPoints) != 1 {
			t.Fatalf("compozy.db.write.duration data = %#v, want one float64 histogram point", histogram)
		}
		want := attribute.NewSet(
			telemetry.AttrDBOperation.String("sessiondb.flush"),
			telemetry.AttrDBCoalesced.Bool(true),
			telemetry.AttrOutcome.String(telemetry.OutcomeOK),
		)
		if got := histogram.DataPoints[0].Attributes; !got.Equals(&want) {
			t.Fatalf("point attributes = %v, want %v", got.ToSlice(), want.ToSlice())
		}
	})

	t.Run("Should drop instruments outside the catalog", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		counter, err := harness.provider.Meter(telemetry.ScopeObserve).Int64Counter("prompt.text.bytes")
		if err != nil {
			t.Fatalf("Int64Counter() error = %v", err)
		}
		counter.Add(context.Background(), 1)

		if _, exported := harness.collect(t); len(exported.ScopeMetrics) != 0 {
			t.Fatalf("scope metrics = %+v, want the unregistered instrument dropped", exported.ScopeMetrics)
		}
	})
}

// UT-024
func TestSanitizingMetricExporterExportsNoExemplars(t *testing.T) {
	t.Parallel()

	t.Run("Should record no exemplars while a sampled span is active", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
		t.Cleanup(func() {
			if err := tracerProvider.Shutdown(context.Background()); err != nil {
				t.Errorf("TracerProvider.Shutdown() error = %v", err)
			}
		})
		ctx, span := tracerProvider.Tracer(telemetry.ScopeSession).
			Start(context.Background(), telemetry.SpanSessionTurn)
		turns, err := harness.provider.Meter(telemetry.ScopeSession).
			Float64Histogram(telemetry.MetricSessionTurnDuration)
		if err != nil {
			t.Fatalf("Float64Histogram() error = %v", err)
		}
		turns.Record(ctx, 1.5, metric.WithAttributes(telemetry.AttrOutcome.String(telemetry.OutcomeOK)))
		span.End()

		collected, exported := harness.collect(t)
		if got := countExemplars(&collected); got != 0 {
			t.Fatalf("collected exemplars = %d, want 0 with the exemplar filter off", got)
		}
		if got := countExemplars(exported); got != 0 {
			t.Fatalf("exported exemplars = %d, want 0", got)
		}
	})

	t.Run("Should strip exemplars present in hand-built input", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		input := exemplarBearingMetrics()
		if got := countExemplars(input); got != 3 {
			t.Fatalf("input exemplars = %d, want 3", got)
		}
		if err := harness.exporter.Export(context.Background(), input); err != nil {
			t.Fatalf("Export() error = %v", err)
		}

		exported := harness.recorded.lastExport()
		if got := countExemplars(exported); got != 0 {
			t.Fatalf("exported exemplars = %d, want 0", got)
		}
		if got := len(exported.ScopeMetrics[0].Metrics); got != 3 {
			t.Fatalf("exported metrics = %d, want the three registered instruments kept", got)
		}
	})
}

func exemplarBearingMetrics() *metricdata.ResourceMetrics {
	now := time.Unix(1_700_000_000, 0)
	attrs := attribute.NewSet(telemetry.AttrOutcome.String(telemetry.OutcomeOK))
	leak := []attribute.KeyValue{attribute.String("prompt.text", "SECRET-CANARY-7f3a")}
	return &metricdata.ResourceMetrics{ScopeMetrics: []metricdata.ScopeMetrics{{
		Scope: instrumentation.Scope{Name: telemetry.ScopeSession},
		Metrics: []metricdata.Metrics{
			{Name: telemetry.MetricPermissionRequests, Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality, IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{{
					Attributes: attrs, Time: now, Value: 1,
					Exemplars: []metricdata.Exemplar[int64]{{FilteredAttributes: leak, Time: now, Value: 1}},
				}},
			}},
			{Name: telemetry.MetricSessionTurnDuration, Data: metricdata.Histogram[float64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[float64]{
					{
						Attributes:   attrs,
						Time:         now,
						Count:        1,
						Bounds:       []float64{1},
						BucketCounts: []uint64{0, 1},
						Sum:          1.5,
						Exemplars:    []metricdata.Exemplar[float64]{{FilteredAttributes: leak, Time: now, Value: 1.5}},
					},
				},
			}},
			{Name: telemetry.MetricSessionActive, Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{{
					Time: now, Value: 3,
					Exemplars: []metricdata.Exemplar[int64]{{FilteredAttributes: leak, Time: now, Value: 3}},
				}},
			}},
		},
	}}}
}

func countExemplars(metrics *metricdata.ResourceMetrics) int {
	count := 0
	for _, scopeMetrics := range metrics.ScopeMetrics {
		for _, item := range scopeMetrics.Metrics {
			switch data := item.Data.(type) {
			case metricdata.Sum[int64]:
				for _, point := range data.DataPoints {
					count += len(point.Exemplars)
				}
			case metricdata.Gauge[int64]:
				for _, point := range data.DataPoints {
					count += len(point.Exemplars)
				}
			case metricdata.Histogram[float64]:
				for _, point := range data.DataPoints {
					count += len(point.Exemplars)
				}
			}
		}
	}
	return count
}

// UT-046 (metrics; the span half is in span_exporter_test.go)
func TestSanitizingMetricExporterAppliesPerKeyFallbackPolicy(t *testing.T) {
	t.Parallel()

	secret := "telemetry-metric-secret-9e2d"
	t.Cleanup(redact.RegisterRequiredSecret(secret))
	secretModel := "org/model-" + secret

	t.Run("Should export every model value through the fallback policy and merge collisions", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil, secretModel))
		turns, err := harness.provider.Meter(telemetry.ScopeSession).
			Float64Histogram(telemetry.MetricSessionTurnDuration)
		if err != nil {
			t.Fatalf("Float64Histogram() error = %v", err)
		}
		cases := modelPolicyCases(secretModel)
		wantCounts := make(map[string]uint64, len(cases))
		for _, tc := range cases {
			turns.Record(context.Background(), 1, metric.WithAttributes(
				telemetry.AttrGenAISystem.String(testProvider), telemetry.AttrGenAIModel.String(tc.model),
			))
			wantCounts[tc.want]++
		}

		_, exported := harness.collect(t)
		histogram, ok := findMetric(t, exported, telemetry.MetricSessionTurnDuration).Data.(metricdata.Histogram[float64])
		if !ok {
			t.Fatalf("compozy.session.turn.duration data = %#v, want a float64 histogram", histogram)
		}
		gotCounts := make(map[string]uint64, len(histogram.DataPoints))
		for _, point := range histogram.DataPoints {
			model, _ := point.Attributes.Value(telemetry.AttrGenAIModel)
			gotCounts[model.AsString()] += point.Count
		}
		if len(histogram.DataPoints) != len(wantCounts) || !maps.Equal(gotCounts, wantCounts) {
			t.Fatalf("model point counts = %v over %d points, want %v with one point per exported value",
				gotCounts, len(histogram.DataPoints), wantCounts)
		}
	})

	t.Run("Should replace unregistered agents and providers and omit out-of-set outcomes", func(t *testing.T) {
		t.Parallel()

		harness := newMetricHarness(t, testRegistries(nil))
		turns, err := harness.provider.Meter(telemetry.ScopeSession).
			Float64Histogram(telemetry.MetricSessionTurnDuration)
		if err != nil {
			t.Fatalf("Float64Histogram() error = %v", err)
		}
		turns.Record(context.Background(), 1, metric.WithAttributes(
			telemetry.AttrAgent.String("unregistered-agent"),
			telemetry.AttrGenAISystem.String("unregistered-provider"),
			telemetry.AttrOutcome.String("exploded"),
		))

		_, exported := harness.collect(t)
		histogram, ok := findMetric(t, exported, telemetry.MetricSessionTurnDuration).Data.(metricdata.Histogram[float64])
		if !ok || len(histogram.DataPoints) != 1 {
			t.Fatalf("compozy.session.turn.duration data = %#v, want one float64 histogram point", histogram)
		}
		want := []attribute.KeyValue{
			telemetry.AttrAgent.String(telemetry.ValueOther),
			telemetry.AttrGenAISystem.String(telemetry.ValueOther),
		}
		if got := histogram.DataPoints[0].Attributes.ToSlice(); !slices.Equal(got, want) {
			t.Fatalf("point attributes = %v, want %v", got, want)
		}
	})

	// The model catalog is keyed by provider and model, so the point's own gen_ai.system must reach the registry.
	t.Run("Should classify the model under the provider recorded on the same point", func(t *testing.T) {
		t.Parallel()

		const providerBoundModel = "org/provider-bound-model"
		reg := testRegistries(nil)
		reg.Models = func(provider, model string) telemetry.ModelClass {
			if provider == testProvider && model == providerBoundModel {
				return telemetry.ModelCatalog
			}
			return telemetry.ModelUnknown
		}
		harness := newMetricHarness(t, reg)
		ttft, err := harness.provider.Meter(telemetry.ScopeSession).Float64Histogram(telemetry.MetricSessionTTFT)
		if err != nil {
			t.Fatalf("Float64Histogram() error = %v", err)
		}
		for _, system := range []string{testProvider, "unregistered-provider"} {
			ttft.Record(context.Background(), 1, metric.WithAttributes(
				telemetry.AttrGenAISystem.String(system), telemetry.AttrGenAIModel.String(providerBoundModel),
			))
		}

		_, exported := harness.collect(t)
		histogram, ok := findMetric(t, exported, telemetry.MetricSessionTTFT).Data.(metricdata.Histogram[float64])
		if !ok {
			t.Fatalf("compozy.session.ttft data = %#v, want a float64 histogram", histogram)
		}
		got := make(map[string]string, len(histogram.DataPoints))
		for _, point := range histogram.DataPoints {
			system, _ := point.Attributes.Value(telemetry.AttrGenAISystem)
			model, _ := point.Attributes.Value(telemetry.AttrGenAIModel)
			got[system.AsString()] = model.AsString()
		}
		want := map[string]string{testProvider: providerBoundModel, telemetry.ValueOther: telemetry.ValueOther}
		if len(histogram.DataPoints) != len(want) || !maps.Equal(got, want) {
			t.Fatalf(
				"exported (provider, model) points = %v over %d points, want %v",
				got,
				len(histogram.DataPoints),
				want,
			)
		}
	})
}
