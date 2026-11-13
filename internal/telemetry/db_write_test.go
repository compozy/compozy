package telemetry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// Not parallel: every case installs the global OTel providers the observer records into.
func TestRecordDBWrite(t *testing.T) {
	// Invariant (Business Rule 8): slow_db_write_ms > 0 && parent recording && (!coalesced || duration >= threshold).
	// Owner: the telemetry write observer; histograms record every write regardless.
	cases := []struct {
		name      string
		threshold int
		parent    bool
		coalesced bool
		duration  time.Duration
		wantSpan  bool
	}{
		{
			"Should span a fast non-coalesced write inside a trace [UT-040]",
			50,
			true,
			false,
			10 * time.Millisecond,
			true,
		},
		{
			"Should span a non-coalesced write just under the threshold [UT-040]",
			50,
			true,
			false,
			49 * time.Millisecond,
			true,
		},
		{"Should keep a fast coalesced write to histograms [UT-041]", 50, true, true, 49 * time.Millisecond, false},
		{"Should span a coalesced write at the threshold [UT-041]", 50, true, true, 50 * time.Millisecond, true},
		{"Should not span a non-coalesced write with threshold 0 [UT-042]", 0, true, false, time.Second, false},
		{"Should not span a coalesced write with threshold 0 [UT-042]", 0, true, true, time.Second, false},
		{"Should not span a slow write outside a trace [UT-042]", 50, false, false, time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exported, metrics, parentCtx, parent := installWriteProviders(t)
			ctx := context.Background()
			if tc.parent {
				ctx = parentCtx
			}
			started := time.Now().Add(-tc.duration)
			telemetry.RecordDBWrite(ctx, telemetry.DBWrite{
				Operation:     "append session events and transcript",
				Coalesced:     tc.coalesced,
				Started:       started,
				AdmissionWait: time.Millisecond,
				Duration:      tc.duration,
			}, tc.threshold)
			parent.End()

			var writes []tracetest.SpanStub
			for _, span := range exported.GetSpans() {
				if span.Name == telemetry.SpanDBWrite {
					writes = append(writes, span)
				}
			}
			if !tc.wantSpan {
				if len(writes) != 0 {
					t.Fatalf("db.write spans = %d, want none", len(writes))
				}
			} else {
				if len(writes) != 1 {
					t.Fatalf("db.write spans = %d, want 1", len(writes))
				}
				write := writes[0]
				if write.Parent.SpanID() != parent.SpanContext().SpanID() {
					t.Fatalf("db.write parent = %s, want %s", write.Parent.SpanID(), parent.SpanContext().SpanID())
				}
				if !write.StartTime.Equal(started) || !write.EndTime.Equal(started.Add(tc.duration)) {
					t.Fatalf("db.write %s..%s, want the measured %s..%s",
						write.StartTime, write.EndTime, started, started.Add(tc.duration))
				}
				attrs := attributeMap(write.Attributes)
				if attrs[telemetry.AttrDBOperation].AsString() != "append_session_events_and_transcript" ||
					attrs[telemetry.AttrDBCoalesced].AsBool() != tc.coalesced ||
					attrs[telemetry.AttrOutcome].AsString() != telemetry.OutcomeOK {
					t.Fatalf("db.write attributes = %v", write.Attributes)
				}
			}
			assertWriteHistograms(t, metrics, tc.coalesced, telemetry.OutcomeOK)
		})
	}

	t.Run("Should export a failed write as an error span and histogram point", func(t *testing.T) {
		exported, metrics, ctx, parent := installWriteProviders(t)
		telemetry.RecordDBWrite(ctx, telemetry.DBWrite{
			Operation: "observe.retention",
			Started:   time.Now(),
			Duration:  time.Millisecond,
			Err:       errors.New("disk I/O error"),
		}, 50)
		parent.End()
		spans := exported.GetSpans()
		if len(spans) != 2 || spans[0].Name != telemetry.SpanDBWrite || spans[0].Status.Code != codes.Error {
			t.Fatalf("spans = %+v, want an error db.write span before its parent", spans)
		}
		if outcome := attributeMap(spans[0].Attributes)[telemetry.AttrOutcome].AsString(); outcome != telemetry.OutcomeError {
			t.Fatalf("db.write outcome = %q, want %q", outcome, telemetry.OutcomeError)
		}
		assertWriteHistograms(t, metrics, false, telemetry.OutcomeError)
	})
}

// installWriteProviders installs global trace and metric providers behind the sanitizing wrappers until the test ends
// and starts a recording parent span.
func installWriteProviders(
	t *testing.T,
) (*tracetest.InMemoryExporter, *metricHarness, context.Context, trace.Span) {
	t.Helper()
	tracerProvider, exported := newTracerProvider(t, telemetry.Registries{})
	metrics := newMetricHarness(t, telemetry.Registries{})
	previousTracer, previousMeter := otel.GetTracerProvider(), otel.GetMeterProvider()
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(metrics.provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTracer)
		otel.SetMeterProvider(previousMeter)
	})
	ctx, parent := tracerProvider.Tracer("test").Start(context.Background(), "parent")
	return exported, metrics, ctx, parent
}

func assertWriteHistograms(t *testing.T, metrics *metricHarness, coalesced bool, outcome string) {
	t.Helper()
	_, exported := metrics.collect(t)
	duration, ok := findMetric(t, exported, telemetry.MetricDBWriteDuration).Data.(metricdata.Histogram[float64])
	if !ok || len(duration.DataPoints) != 1 || duration.DataPoints[0].Count != 1 {
		t.Fatalf("%s = %+v, want one point", telemetry.MetricDBWriteDuration, duration)
	}
	attrs := duration.DataPoints[0].Attributes
	if got, _ := attrs.Value(telemetry.AttrDBCoalesced); got.AsBool() != coalesced {
		t.Fatalf("%s coalesced = %v, want %v", telemetry.MetricDBWriteDuration, got.AsBool(), coalesced)
	}
	if got, _ := attrs.Value(telemetry.AttrOutcome); got.AsString() != outcome {
		t.Fatalf("%s outcome = %q, want %q", telemetry.MetricDBWriteDuration, got.AsString(), outcome)
	}
	if _, ok := attrs.Value(telemetry.AttrDBOperation); !ok {
		t.Fatalf("%s lost %s", telemetry.MetricDBWriteDuration, telemetry.AttrDBOperation)
	}
	wait, ok := findMetric(t, exported, telemetry.MetricDBWriteAdmissionWait).Data.(metricdata.Histogram[float64])
	if !ok || len(wait.DataPoints) != 1 || wait.DataPoints[0].Count != 1 {
		t.Fatalf("%s = %+v, want one point", telemetry.MetricDBWriteAdmissionWait, wait)
	}
}
