package telemetry

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// dbWriteBuckets are the write duration and admission wait bounds in seconds: SQLite writes take sub-millisecond to
// seconds under contention.
var dbWriteBuckets = []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// DBWrite is one finished database write: Duration spans admission, every retry, and commit; AdmissionWait is the
// part spent waiting for write admission. Its fields mirror store.WriteObservation, which converts to it.
type DBWrite struct {
	Operation     string
	Coalesced     bool
	Started       time.Time
	AdmissionWait time.Duration
	Duration      time.Duration
	Err           error
}

// RecordDBWrite records write in the write histograms and, when writeSpanWanted holds for slowDBWriteMS, as a db.write
// span with the measured timestamps under ctx's span.
func RecordDBWrite(ctx context.Context, write DBWrite, slowDBWriteMS int) {
	outcome := writeOutcome(write.Err)
	// Code-owned operation names are prose ("update session state"); the exported shape admits no spaces.
	operation := AttrDBOperation.String(strings.ReplaceAll(write.Operation, " ", "_"))
	coalesced := AttrDBCoalesced.Bool(write.Coalesced)
	meter := Meter(ScopeStore)
	duration, err := meter.Float64Histogram(MetricDBWriteDuration,
		metric.WithUnit("s"), metric.WithExplicitBucketBoundaries(dbWriteBuckets...))
	if err == nil {
		duration.Record(ctx, write.Duration.Seconds(),
			metric.WithAttributes(operation, coalesced, AttrOutcome.String(outcome)))
	}
	wait, err := meter.Float64Histogram(MetricDBWriteAdmissionWait,
		metric.WithUnit("s"), metric.WithExplicitBucketBoundaries(dbWriteBuckets...))
	if err == nil {
		wait.Record(ctx, write.AdmissionWait.Seconds(), metric.WithAttributes(operation))
	}
	threshold := time.Duration(slowDBWriteMS) * time.Millisecond
	if !writeSpanWanted(threshold, trace.SpanFromContext(ctx).IsRecording(), write.Coalesced, write.Duration) {
		return
	}
	_, span := Tracer(ScopeStore).Start(ctx, SpanDBWrite,
		trace.WithTimestamp(write.Started), trace.WithAttributes(operation, coalesced))
	if outcome == OutcomeError {
		MarkError(span, write.Err)
	}
	span.SetAttributes(AttrOutcome.String(outcome))
	span.End(trace.WithTimestamp(write.Started.Add(write.Duration)))
}

// writeSpanWanted is the single db.write span predicate: a positive threshold, a recording parent, and a write that is
// either not coalesced or at least as slow as the threshold.
func writeSpanWanted(threshold time.Duration, parentRecording, coalesced bool, duration time.Duration) bool {
	return threshold > 0 && parentRecording && (!coalesced || duration >= threshold)
}

// writeOutcome matches EndSpan: a canceled write is OutcomeCancelled, any other failure OutcomeError.
func writeOutcome(err error) string {
	switch {
	case err == nil:
		return OutcomeOK
	case errors.Is(err, context.Canceled):
		return OutcomeCancelled
	default:
		return OutcomeError
	}
}
